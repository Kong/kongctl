package helpers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

type dcrProviderHTTPClient struct {
	t        *testing.T
	requests []*http.Request
	bodies   [][]byte
}

func (c *dcrProviderHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.t.Helper()

	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
	}
	c.requests = append(c.requests, req.Clone(req.Context()))
	c.bodies = append(c.bodies, body)

	switch req.Method {
	case http.MethodGet:
		return dcrProviderJSONResponse(http.StatusOK, `{
			"data": [{
				"provider_type": "DcrProviderHttp",
				"dcr_config": {},
				"id": "provider-1",
				"name": "provider-one",
				"display_name": "Provider One",
				"issuer": "https://issuer.example.test",
				"active": false,
				"labels": {"KONGCTL-managed": "true"},
				"created_at": "2026-03-30T00:00:00Z",
				"updated_at": "2026-03-30T00:00:00Z"
			}],
			"meta": {"page": {"number": 2, "size": 25, "total": 51}}
		}`), nil
	case http.MethodPost:
		return dcrProviderJSONResponse(http.StatusCreated, dcrProviderResponseBody()), nil
	case http.MethodPatch:
		return dcrProviderJSONResponse(http.StatusOK, dcrProviderResponseBody()), nil
	case http.MethodDelete:
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Header:     http.Header{},
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	default:
		c.t.Fatalf("unexpected method: %s", req.Method)
		return nil, nil
	}
}

func dcrProviderJSONResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewBufferString(body)),
	}
}

func dcrProviderResponseBody() string {
	return `{
		"provider_type": "DcrProviderHttp",
		"dcr_config": {},
		"id": "provider-1",
		"name": "provider-one",
		"display_name": "Provider One",
		"issuer": "https://issuer.example.test",
		"active": false,
		"labels": {"KONGCTL-managed": "true"},
		"created_at": "2026-03-30T00:00:00Z",
		"updated_at": "2026-03-30T00:00:00Z"
	}`
}

func TestDCRProvidersAPIImplListDcrProviderPayloadsUsesSDKResponse(t *testing.T) {
	t.Parallel()

	client := &dcrProviderHTTPClient{t: t}
	api := newTestDCRProvidersAPI(client)
	pageSize := int64(25)
	pageNumber := int64(2)

	payload, err := api.ListDcrProviderPayloads(context.Background(), kkOps.ListDcrProvidersRequest{
		PageSize:   &pageSize,
		PageNumber: &pageNumber,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if payload == nil {
		t.Fatalf("expected payload")
	}
	if len(payload.Data) != 1 {
		t.Fatalf("expected one provider, got %d", len(payload.Data))
	}
	if payload.Total != 51 {
		t.Fatalf("unexpected total: %v", payload.Total)
	}

	if len(client.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(client.requests))
	}
	req := client.requests[0]
	if req.Method != http.MethodGet {
		t.Fatalf("unexpected method: %s", req.Method)
	}
	if got := req.URL.String(); got != "https://example.test/v2/dcr-providers?page%5Bnumber%5D=2&page%5Bsize%5D=25" {
		t.Fatalf("unexpected URL: %s", got)
	}
}

func TestDCRProvidersAPIImplCRUDDelegatesToSDK(t *testing.T) {
	t.Parallel()

	client := &dcrProviderHTTPClient{t: t}
	api := newTestDCRProvidersAPI(client)

	createReq := kkComps.CreateCreateDcrProviderRequestHTTP(kkComps.CreateDcrProviderRequestHTTP{
		DcrConfig: kkComps.CreateDcrConfigHTTPInRequest{
			DcrBaseURL: "https://dcr.example.test",
			APIKey:     "secret",
		},
		Name:   "provider-one",
		Issuer: "https://issuer.example.test",
	})
	if _, err := api.CreateDcrProvider(context.Background(), createReq); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	name := "provider-one-renamed"
	if _, err := api.UpdateDcrProvider(context.Background(), "provider-1", kkComps.UpdateDcrProviderRequest{
		Name: &name,
	}); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	if _, err := api.DeleteDcrProvider(context.Background(), "provider-1"); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}

	if len(client.requests) != 3 {
		t.Fatalf("expected three requests, got %d", len(client.requests))
	}

	expected := []struct {
		method string
		url    string
	}{
		{method: http.MethodPost, url: "https://example.test/v2/dcr-providers"},
		{method: http.MethodPatch, url: "https://example.test/v2/dcr-providers/provider-1"},
		{method: http.MethodDelete, url: "https://example.test/v2/dcr-providers/provider-1"},
	}
	for i, want := range expected {
		if got := client.requests[i].Method; got != want.method {
			t.Fatalf("request %d method = %s, want %s", i, got, want.method)
		}
		if got := client.requests[i].URL.String(); got != want.url {
			t.Fatalf("request %d URL = %s, want %s", i, got, want.url)
		}
	}
}

func newTestDCRProvidersAPI(client *dcrProviderHTTPClient) *DCRProvidersAPIImpl {
	return &DCRProvidersAPIImpl{
		SDK: kkSDK.New(
			kkSDK.WithServerURL("https://example.test"),
			kkSDK.WithClient(client),
		),
	}
}
