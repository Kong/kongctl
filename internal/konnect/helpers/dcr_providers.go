package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	kkSDK "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOPS "github.com/Kong/sdk-konnect-go/models/operations"

	"github.com/kong/kongctl/internal/konnect/apiutil"
)

type DCRProvidersAPI interface {
	ListDcrProviders(ctx context.Context, request kkOPS.ListDcrProvidersRequest,
		opts ...kkOPS.Option) (*kkOPS.ListDcrProvidersResponse, error)
	ListDcrProviderPayloads(ctx context.Context, request kkOPS.ListDcrProvidersRequest) (*DCRProviderListPayload, error)
	CreateDcrProvider(ctx context.Context,
		provider kkComps.CreateDcrProviderRequest) (*kkOPS.CreateDcrProviderResponse, error)
	UpdateDcrProvider(ctx context.Context, id string,
		provider kkComps.UpdateDcrProviderRequest) (*kkOPS.UpdateDcrProviderResponse, error)
	DeleteDcrProvider(ctx context.Context, id string) (*kkOPS.DeleteDcrProviderResponse, error)
}

// DCRProviderListPayload contains raw DCR provider response payloads.
type DCRProviderListPayload struct {
	Data  []any
	Total float64
}

// NormalizedDCRProviderPayload contains the common DCR provider fields returned by
// Konnect across list/create/update flows.
type NormalizedDCRProviderPayload struct {
	ID             string
	Name           string
	DisplayName    string
	DisplayNameSet bool
	ProviderType   string
	Issuer         string
	DCRConfig      map[string]any
	Labels         map[string]string
}

type dcrProviderListResponsePayload struct {
	Data []any `json:"data"`
	Meta struct {
		Page struct {
			Total float64 `json:"total"`
		} `json:"page"`
	} `json:"meta"`
}

type normalizedDCRProviderPayload struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	DisplayName  *string           `json:"display_name"`
	ProviderType string            `json:"provider_type"`
	Issuer       string            `json:"issuer"`
	DCRConfig    map[string]any    `json:"dcr_config"`
	Labels       map[string]string `json:"labels"`
}

func NormalizeDCRProviderPayload(data any) (*NormalizedDCRProviderPayload, error) {
	payloadBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DCR provider payload: %w", err)
	}

	var payload normalizedDCRProviderPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal DCR provider payload: %w", err)
	}

	displayName := ""
	if payload.DisplayName != nil {
		displayName = *payload.DisplayName
	}

	return &NormalizedDCRProviderPayload{
		ID:             payload.ID,
		Name:           payload.Name,
		DisplayName:    displayName,
		DisplayNameSet: payload.DisplayName != nil,
		ProviderType:   payload.ProviderType,
		Issuer:         payload.Issuer,
		DCRConfig:      payload.DCRConfig,
		Labels:         payload.Labels,
	}, nil
}

// DCRProvidersAPIImpl provides an implementation of the DCRProvidersAPI interface
type DCRProvidersAPIImpl struct {
	SDK        *kkSDK.SDK
	BaseURL    string
	Token      string
	HTTPClient kkSDK.HTTPClient
}

func (a *DCRProvidersAPIImpl) ListDcrProviders(ctx context.Context,
	request kkOPS.ListDcrProvidersRequest,
	opts ...kkOPS.Option,
) (*kkOPS.ListDcrProvidersResponse, error) {
	return a.SDK.DCRProviders.ListDcrProviders(ctx, request, opts...)
}

func (a *DCRProvidersAPIImpl) ListDcrProviderPayloads(
	ctx context.Context,
	request kkOPS.ListDcrProvidersRequest,
) (*DCRProviderListPayload, error) {
	if !a.canUseRawRequest() {
		res, err := a.ListDcrProviders(ctx, request)
		if err != nil {
			return nil, err
		}
		if res == nil || res.ListDcrProvidersResponse == nil {
			return &DCRProviderListPayload{}, nil
		}
		data := make([]any, 0, len(res.ListDcrProvidersResponse.Data))
		for _, provider := range res.ListDcrProvidersResponse.Data {
			data = append(data, provider)
		}
		return &DCRProviderListPayload{
			Data:  data,
			Total: res.ListDcrProvidersResponse.Meta.Page.Total,
		}, nil
	}

	path := "/v2/dcr-providers"
	query := url.Values{}
	if request.PageSize != nil {
		query.Set("page[size]", fmt.Sprintf("%d", *request.PageSize))
	}
	if request.PageNumber != nil {
		query.Set("page[number]", fmt.Sprintf("%d", *request.PageNumber))
	}
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}

	result, err := apiutil.Request(ctx, a.HTTPClient, http.MethodGet, a.BaseURL, path, a.Token, nil, nil)
	if err != nil {
		return nil, err
	}
	if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
		return nil, dcrProviderStatusError("list DCR providers", result)
	}

	var payload dcrProviderListResponsePayload
	if err := json.Unmarshal(result.Body, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal DCR provider list response: %w", err)
	}

	return &DCRProviderListPayload{
		Data:  payload.Data,
		Total: payload.Meta.Page.Total,
	}, nil
}

func (a *DCRProvidersAPIImpl) CreateDcrProvider(ctx context.Context,
	provider kkComps.CreateDcrProviderRequest,
) (*kkOPS.CreateDcrProviderResponse, error) {
	if a.canUseRawRequest() {
		payload, err := json.Marshal(provider)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal DCR provider create request: %w", err)
		}

		result, err := apiutil.Request(
			ctx,
			a.HTTPClient,
			http.MethodPost,
			a.BaseURL,
			"/v2/dcr-providers",
			a.Token,
			map[string]string{"Content-Type": "application/json"},
			bytes.NewReader(payload),
		)
		if err != nil {
			return nil, err
		}

		response := &kkOPS.CreateDcrProviderResponse{
			ContentType: result.Header.Get("Content-Type"),
			StatusCode:  result.StatusCode,
			RawResponse: newDCRProviderRawResponse(result),
		}
		if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
			return nil, dcrProviderStatusError("create DCR provider", result)
		}
		return response, nil
	}

	return a.SDK.DCRProviders.CreateDcrProvider(ctx, provider)
}

func (a *DCRProvidersAPIImpl) UpdateDcrProvider(ctx context.Context, id string,
	provider kkComps.UpdateDcrProviderRequest,
) (*kkOPS.UpdateDcrProviderResponse, error) {
	if a.canUseRawRequest() {
		payload, err := json.Marshal(provider)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal DCR provider update request: %w", err)
		}

		path := fmt.Sprintf("/v2/dcr-providers/%s", url.PathEscape(id))
		result, err := apiutil.Request(
			ctx,
			a.HTTPClient,
			http.MethodPatch,
			a.BaseURL,
			path,
			a.Token,
			map[string]string{"Content-Type": "application/json"},
			bytes.NewReader(payload),
		)
		if err != nil {
			return nil, err
		}

		response := &kkOPS.UpdateDcrProviderResponse{
			ContentType: result.Header.Get("Content-Type"),
			StatusCode:  result.StatusCode,
			RawResponse: newDCRProviderRawResponse(result),
		}
		if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
			return nil, dcrProviderStatusError("update DCR provider", result)
		}
		return response, nil
	}

	return a.SDK.DCRProviders.UpdateDcrProvider(ctx, id, provider)
}

func (a *DCRProvidersAPIImpl) DeleteDcrProvider(ctx context.Context,
	id string,
) (*kkOPS.DeleteDcrProviderResponse, error) {
	if a.canUseRawRequest() {
		path := fmt.Sprintf("/v2/dcr-providers/%s", url.PathEscape(id))
		result, err := apiutil.Request(
			ctx,
			a.HTTPClient,
			http.MethodDelete,
			a.BaseURL,
			path,
			a.Token,
			nil,
			nil,
		)
		if err != nil {
			return nil, err
		}

		response := &kkOPS.DeleteDcrProviderResponse{
			ContentType: result.Header.Get("Content-Type"),
			StatusCode:  result.StatusCode,
			RawResponse: newDCRProviderRawResponse(result),
		}
		if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
			return nil, dcrProviderStatusError("delete DCR provider", result)
		}
		return response, nil
	}

	return a.SDK.DCRProviders.DeleteDcrProvider(ctx, id)
}

func (a *DCRProvidersAPIImpl) canUseRawRequest() bool {
	return a != nil &&
		a.HTTPClient != nil &&
		strings.TrimSpace(a.BaseURL) != ""
}

func newDCRProviderRawResponse(result *apiutil.Result) *http.Response {
	if result == nil {
		return nil
	}

	return &http.Response{
		StatusCode: result.StatusCode,
		Header:     result.Header.Clone(),
		Body:       io.NopCloser(bytes.NewReader(result.Body)),
	}
}

func dcrProviderStatusError(action string, result *apiutil.Result) error {
	if result == nil {
		return fmt.Errorf("%s failed", action)
	}
	body := strings.TrimSpace(string(result.Body))
	if body == "" {
		return fmt.Errorf("%s failed with status %d", action, result.StatusCode)
	}
	return fmt.Errorf("%s failed with status %d: %s", action, result.StatusCode, body)
}
