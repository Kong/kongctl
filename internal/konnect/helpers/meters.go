package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/konnect/apiutil"
)

// MetersAPI is the SDK contract shared by imperative and declarative meters.
type MetersAPI = kkSDK.MetersSDK

// ListAllMeters follows API pagination before callers filter ownership or identity.
func ListAllMeters(ctx context.Context, api MetersAPI, pageSize int64) ([]kkComps.Meter, error) {
	if api == nil {
		return nil, fmt.Errorf("meters API client is not configured")
	}
	if pageSize < 1 || pageSize > 1000 {
		return nil, fmt.Errorf("meter page size must be between 1 and 1000")
	}
	result := make([]kkComps.Meter, 0)
	for page := int64(1); page <= 10000; page++ {
		resp, err := api.ListMeters(
			ctx,
			kkOps.ListMetersRequest{Page: &kkComps.PagePaginationQuery{Size: &pageSize, Number: &page}},
		)
		if err != nil {
			return nil, fmt.Errorf("list meters: %w", err)
		}
		if resp == nil || resp.MeterPagePaginatedResponse == nil {
			return nil, fmt.Errorf("list meters response missing data")
		}
		body := resp.MeterPagePaginatedResponse
		result = append(result, body.Data...)
		if len(body.Data) == 0 || float64(len(result)) >= body.Meta.Page.Total {
			return result, nil
		}
	}
	return nil, fmt.Errorf("meter pagination exceeded safety limit of 10000 pages")
}

// ResolveMeter accepts API keys or ULIDs, never display names or local refs.
func ResolveMeter(ctx context.Context, api MetersAPI, identifier string, pageSize int64) (*kkComps.Meter, error) {
	if api == nil {
		return nil, fmt.Errorf("meters API client is not configured")
	}
	if IsMeterULID(identifier) {
		resp, err := api.GetMeter(ctx, identifier)
		if err != nil {
			return nil, fmt.Errorf("get meter %q: %w", identifier, err)
		}
		if resp == nil || resp.Meter == nil {
			return nil, fmt.Errorf("get meter response missing data")
		}
		return resp.Meter, nil
	}
	values, err := ListAllMeters(ctx, api, pageSize)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if value.Key == identifier {
			return &value, nil
		}
	}
	return nil, fmt.Errorf("meter key %q not found; use its API key or ULID", identifier)
}

// MetersAPIImpl retains SDK authentication, retries, decoding and API errors,
// repairing only explicitly empty maps lost by generated omitempty tags.
type MetersAPIImpl struct {
	MetersAPI
	BaseURL     string
	Token       string
	TokenSource apiutil.TokenSource
	HTTPClient  kkSDK.HTTPClient
}

func (m *MetersAPIImpl) UpdateMeter(
	ctx context.Context,
	id string,
	req kkComps.UpdateMeterRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateMeterResponse, error) {
	empty := map[string]any{}
	if req.Dimensions != nil && len(req.Dimensions) == 0 {
		empty["dimensions"] = req.Dimensions
	}
	if req.Labels != nil && len(req.Labels) == 0 {
		empty["labels"] = req.Labels
	}
	if len(empty) == 0 {
		return m.MetersAPI.UpdateMeter(ctx, id, req, opts...)
	}
	if m.HTTPClient == nil || m.BaseURL == "" {
		return nil, fmt.Errorf("explicit empty meter maps require a configured HTTP client and base URL")
	}
	options := []kkSDK.SDKOption{
		kkSDK.WithServerURL(m.BaseURL),
		kkSDK.WithClient(&meterEmptyMapClient{client: m.HTTPClient, fields: empty}),
	}
	if m.TokenSource != nil {
		options = append(options, kkSDK.WithSecuritySource(func(ctx context.Context) (kkComps.Security, error) {
			token, err := m.TokenSource.Token(ctx)
			return kkComps.Security{PersonalAccessToken: &token}, err
		}))
	} else {
		options = append(options, kkSDK.WithSecurity(kkComps.Security{PersonalAccessToken: &m.Token}))
	}
	sdk := kkSDK.New(options...)
	return sdk.Meters.UpdateMeter(ctx, id, req, opts...)
}

type meterEmptyMapClient struct {
	client kkSDK.HTTPClient
	fields map[string]any
}

func (m *meterEmptyMapClient) Do(req *http.Request) (*http.Response, error) {
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("read meter update payload: %w", err)
	}
	if err := req.Body.Close(); err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode meter update payload: %w", err)
	}
	maps.Copy(payload, m.fields)
	data, err = json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req = req.Clone(req.Context())
	req.Body = io.NopCloser(bytes.NewReader(data))
	req.ContentLength = int64(len(data))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
	return m.client.Do(req)
}
