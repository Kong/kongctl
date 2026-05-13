package helpers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOPS "github.com/Kong/sdk-konnect-go/models/operations"
)

var (
	errDCRProvidersSDKRequired       = errors.New("dcr providers helper requires SDK")
	errDCRProvidersSDKClientRequired = errors.New("dcr providers helper requires SDK.DCRProviders")
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

// DCRProviderListPayload contains DCR provider response payloads.
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
	SDK *kkSDK.SDK
}

func (a *DCRProvidersAPIImpl) ListDcrProviders(ctx context.Context,
	request kkOPS.ListDcrProvidersRequest,
	opts ...kkOPS.Option,
) (*kkOPS.ListDcrProvidersResponse, error) {
	if err := a.validateSDK(); err != nil {
		return nil, err
	}
	return a.SDK.DCRProviders.ListDcrProviders(ctx, request, opts...)
}

func (a *DCRProvidersAPIImpl) ListDcrProviderPayloads(
	ctx context.Context,
	request kkOPS.ListDcrProvidersRequest,
) (*DCRProviderListPayload, error) {
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

func (a *DCRProvidersAPIImpl) CreateDcrProvider(ctx context.Context,
	provider kkComps.CreateDcrProviderRequest,
) (*kkOPS.CreateDcrProviderResponse, error) {
	if err := a.validateSDK(); err != nil {
		return nil, err
	}
	return a.SDK.DCRProviders.CreateDcrProvider(ctx, provider)
}

func (a *DCRProvidersAPIImpl) UpdateDcrProvider(ctx context.Context, id string,
	provider kkComps.UpdateDcrProviderRequest,
) (*kkOPS.UpdateDcrProviderResponse, error) {
	if err := a.validateSDK(); err != nil {
		return nil, err
	}
	return a.SDK.DCRProviders.UpdateDcrProvider(ctx, id, provider)
}

func (a *DCRProvidersAPIImpl) DeleteDcrProvider(ctx context.Context,
	id string,
) (*kkOPS.DeleteDcrProviderResponse, error) {
	if err := a.validateSDK(); err != nil {
		return nil, err
	}
	return a.SDK.DCRProviders.DeleteDcrProvider(ctx, id)
}

func (a *DCRProvidersAPIImpl) validateSDK() error {
	if a == nil || a.SDK == nil {
		return errDCRProvidersSDKRequired
	}
	if a.SDK.DCRProviders == nil {
		return errDCRProvidersSDKClientRequired
	}
	return nil
}
