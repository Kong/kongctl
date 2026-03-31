package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

type capturingHTTPClient struct {
	t           *testing.T
	request     *http.Request
	requestBody []byte
}

func (c *capturingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.t.Helper()

	c.request = req.Clone(req.Context())
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	c.requestBody = body

	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewReader([]byte(`{
			"auth_strategy_ids": null,
			"visibility": "private",
			"created_at": "2026-03-30T00:00:00Z",
			"updated_at": "2026-03-30T00:00:00Z"
		}`))),
	}, nil
}

type visibilityAwareHTTPClient struct {
	t              *testing.T
	putRequest     *http.Request
	putRequestBody []byte
}

func (c *visibilityAwareHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.t.Helper()

	switch req.Method {
	case http.MethodGet:
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(bytes.NewReader([]byte(`{
				"auth_strategy_ids": ["existing-strategy"],
				"visibility": "private",
				"created_at": "2026-03-30T00:00:00Z",
				"updated_at": "2026-03-30T00:00:00Z"
			}`))),
		}, nil
	case http.MethodPut:
		c.putRequest = req.Clone(req.Context())
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		c.putRequestBody = body

		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(bytes.NewReader([]byte(`{
				"auth_strategy_ids": null,
				"visibility": "private",
				"created_at": "2026-03-30T00:00:00Z",
				"updated_at": "2026-03-30T00:00:00Z"
			}`))),
		}, nil
	default:
		c.t.Fatalf("unexpected method: %s", req.Method)
		return nil, nil
	}
}

func TestAPIPublicationAPIImplPublishAPIToPortalIncludesEmptyAuthStrategyIDs(t *testing.T) {
	t.Parallel()

	vis := kkComponents.APIPublicationVisibilityPrivate
	client := &capturingHTTPClient{t: t}

	api := &APIPublicationAPIImpl{
		SDK:        kkSDK.New(),
		BaseURL:    "https://example.test",
		Token:      "test-token",
		HTTPClient: client,
	}

	resp, err := api.PublishAPIToPortal(context.Background(), kkOps.PublishAPIToPortalRequest{
		APIID:    "api-123",
		PortalID: "portal-456",
		APIPublication: kkComponents.APIPublication{
			AuthStrategyIds: []string{},
			Visibility:      &vis,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.request == nil {
		t.Fatalf("expected request to be captured")
	}
	if client.request.Method != http.MethodPut {
		t.Fatalf("unexpected method: %s", client.request.Method)
	}
	if got := client.request.URL.String(); got != "https://example.test/v3/apis/api-123/publications/portal-456" {
		t.Fatalf("unexpected URL: %s", got)
	}
	if got := client.request.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Fatalf("unexpected Authorization header: %q", got)
	}

	var requestBody map[string]any
	if err := json.Unmarshal(client.requestBody, &requestBody); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	if authStrategyIDs, ok := requestBody["auth_strategy_ids"]; !ok {
		t.Fatalf("expected auth_strategy_ids to be present, got %v", requestBody["auth_strategy_ids"])
	} else if authStrategyIDs != nil {
		t.Fatalf("expected auth_strategy_ids to be null, got %v", authStrategyIDs)
	}
	if got := requestBody["visibility"]; got != "private" {
		t.Fatalf("unexpected visibility: %v", got)
	}

	if resp == nil || resp.APIPublicationResponse == nil {
		t.Fatalf("expected publication response, got %#v", resp)
	}
	if len(resp.APIPublicationResponse.AuthStrategyIds) != 0 {
		t.Fatalf("expected empty auth strategy IDs in response, got %v", resp.APIPublicationResponse.AuthStrategyIds)
	}
}

func TestAPIPublicationAPIImplPublishAPIToPortalPreservesExistingVisibilityWhenClearingAuthStrategyIDs(t *testing.T) {
	t.Parallel()

	client := &visibilityAwareHTTPClient{t: t}
	sdk := kkSDK.New(
		kkSDK.WithServerURL("https://example.test"),
		kkSDK.WithClient(client),
	)

	api := &APIPublicationAPIImpl{
		SDK:        sdk,
		BaseURL:    "https://example.test",
		Token:      "test-token",
		HTTPClient: client,
	}

	resp, err := api.PublishAPIToPortal(context.Background(), kkOps.PublishAPIToPortalRequest{
		APIID:    "api-123",
		PortalID: "portal-456",
		APIPublication: kkComponents.APIPublication{
			AuthStrategyIds: []string{},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.putRequest == nil {
		t.Fatalf("expected PUT request to be captured")
	}

	var requestBody map[string]any
	if err := json.Unmarshal(client.putRequestBody, &requestBody); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	if got := requestBody["visibility"]; got != "private" {
		t.Fatalf("expected preserved visibility, got %v", got)
	}
	if authStrategyIDs, ok := requestBody["auth_strategy_ids"]; !ok {
		t.Fatalf("expected auth_strategy_ids to be present, got %v", requestBody["auth_strategy_ids"])
	} else if authStrategyIDs != nil {
		t.Fatalf("expected auth_strategy_ids to be null, got %v", authStrategyIDs)
	}

	if resp == nil || resp.APIPublicationResponse == nil || resp.APIPublicationResponse.Visibility == nil {
		t.Fatalf("expected publication response with visibility, got %#v", resp)
	}
	if got := *resp.APIPublicationResponse.Visibility; got != kkComponents.APIPublicationVisibilityPrivate {
		t.Fatalf("unexpected visibility in response: %s", got)
	}
}

func TestAPIPublicationAPIImplPublishAPIToPortalPreservesExistingAuthStrategyIDsWhenOmitted(t *testing.T) {
	t.Parallel()

	vis := kkComponents.APIPublicationVisibilityPrivate
	client := &visibilityAwareHTTPClient{t: t}
	sdk := kkSDK.New(
		kkSDK.WithServerURL("https://example.test"),
		kkSDK.WithClient(client),
	)

	api := &APIPublicationAPIImpl{
		SDK:        sdk,
		BaseURL:    "https://example.test",
		Token:      "test-token",
		HTTPClient: client,
	}

	_, err := api.PublishAPIToPortal(context.Background(), kkOps.PublishAPIToPortalRequest{
		APIID:    "api-123",
		PortalID: "portal-456",
		APIPublication: kkComponents.APIPublication{
			Visibility: &vis,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.putRequest == nil {
		t.Fatalf("expected PUT request to be captured")
	}

	var requestBody map[string]any
	if err := json.Unmarshal(client.putRequestBody, &requestBody); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	authStrategyIDs, ok := requestBody["auth_strategy_ids"].([]any)
	if !ok {
		t.Fatalf("expected auth_strategy_ids to be preserved, got %v", requestBody["auth_strategy_ids"])
	}
	if len(authStrategyIDs) != 1 || authStrategyIDs[0] != "existing-strategy" {
		t.Fatalf("unexpected preserved auth_strategy_ids: %v", authStrategyIDs)
	}
	if got := requestBody["visibility"]; got != "private" {
		t.Fatalf("unexpected visibility: %v", got)
	}
}
