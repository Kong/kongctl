package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util/httpheaders"
	"github.com/stretchr/testify/require"
)

type aiGatewayModelCapturingClient struct {
	request     *http.Request
	requestBody []byte
}

func (c *aiGatewayModelCapturingClient) Do(req *http.Request) (*http.Response, error) {
	c.request = req.Clone(req.Context())
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	c.requestBody = body

	return &http.Response{
		StatusCode: http.StatusCreated,
		Header: http.Header{
			httpheaders.HeaderContentType: []string{httpheaders.MediaTypeJSON},
		},
		Body: io.NopCloser(bytes.NewReader([]byte(`{
			"id": "00000000-0000-0000-0000-000000000001",
			"created_at": "2026-06-25T00:00:00Z",
			"updated_at": "2026-06-25T00:00:00Z",
			"type": "model",
			"name": "support-gpt",
			"display_name": "Support GPT",
			"enabled": true,
			"config": {"route": {}, "model": {}},
			"formats": [{"type": "openai"}],
			"target_models": [{
				"name": "gpt-4o-mini",
				"provider": "shared-openai",
				"config": {"type": "openai"}
			}],
			"policies": [],
			"capabilities": ["generate"]
		}`))),
	}, nil
}

func TestAIGatewayModelAPIImplCreateAiGatewayModelAddsTargetsToSDKRequest(t *testing.T) {
	t.Parallel()

	client := &aiGatewayModelCapturingClient{}
	api := &AIGatewayModelAPIImpl{
		SDK:        kkSDK.New(),
		BaseURL:    "https://example.test",
		Token:      "test-token",
		HTTPClient: client,
	}

	formatType := kkComps.AIGatewayModelFormatTypeOpenai
	targetConfig := kkComps.CreateAIGatewayTargetModelConfigOpenai(kkComps.AIGatewayTargetModelOpenaiConfig{})
	req := kkComps.CreateCreateAIGatewayModelRequestModel(kkComps.AIGatewayModelModel{
		DisplayName: "Support GPT",
		Name:        "support-gpt",
		Config: kkComps.AIGatewayModelConfig{
			Route: kkComps.AIGatewayRouteConfig{},
			Model: kkComps.AIGatewayModelConfigModel{},
		},
		Formats: []kkComps.AIGatewayModelFormat{
			{Type: &formatType},
		},
		TargetModels: []kkComps.AIGatewayTargetModel{
			{
				Name:     "gpt-4o-mini",
				Provider: "shared-openai",
				Config:   targetConfig,
			},
		},
		Policies:     []string{},
		Capabilities: []kkComps.AIGatewayModelModelCapabilities{kkComps.AIGatewayModelModelCapabilitiesGenerate},
	})

	resp, err := api.CreateAiGatewayModel(context.Background(), "gateway-123", req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.AIGatewayModel)
	require.NotNil(t, client.request)
	require.Equal(t, http.MethodPost, client.request.Method)
	require.Equal(t, "https://example.test/v1/ai-gateways/gateway-123/models", client.request.URL.String())

	var requestBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(client.requestBody, &requestBody))
	require.Contains(t, requestBody, "target_models")
	require.Contains(t, requestBody, "targets")
	require.JSONEq(t, string(requestBody["target_models"]), string(requestBody["targets"]))
}

func TestAddTargetsToAIGatewayModelRequestCopiesTargetModels(t *testing.T) {
	req := newAIGatewayModelMutationRequest(t, http.MethodPost, "/v1/ai-gateways/gw-1/models", `{
		"type": "model",
		"target_models": [
			{"name": "gpt-4o-mini", "provider": "shared-openai", "config": {"type": "openai"}}
		]
	}`)

	require.NoError(t, addTargetsToAIGatewayModelRequest(req))

	payload := readAIGatewayModelRequestPayload(t, req)
	require.JSONEq(t, string(payload["target_models"]), string(payload["targets"]))
}

func TestAddTargetsToAIGatewayModelRequestPreservesExistingTargets(t *testing.T) {
	req := newAIGatewayModelMutationRequest(t, http.MethodPost, "/v1/ai-gateways/gw-1/models", `{
		"type": "model",
		"target_models": [
			{"name": "gpt-4o-mini", "provider": "shared-openai", "config": {"type": "openai"}}
		],
		"targets": [
			{"name": "custom", "provider": "shared-openai", "config": {"type": "openai"}}
		]
	}`)

	require.NoError(t, addTargetsToAIGatewayModelRequest(req))

	payload := readAIGatewayModelRequestPayload(t, req)
	require.JSONEq(
		t,
		`[{"name":"custom","provider":"shared-openai","config":{"type":"openai"}}]`,
		string(payload["targets"]),
	)
}

func TestAddTargetsToAIGatewayModelRequestSupportsUpdatePath(t *testing.T) {
	req := newAIGatewayModelMutationRequest(t, http.MethodPut, "/v1/ai-gateways/gw-1/models/model-1", `{
		"type": "model",
		"target_models": [
			{"name": "gpt-4o-mini", "provider": "shared-openai", "config": {"type": "openai"}}
		]
	}`)

	require.NoError(t, addTargetsToAIGatewayModelRequest(req))

	payload := readAIGatewayModelRequestPayload(t, req)
	require.Contains(t, payload, "targets")
}

func TestAddTargetsToAIGatewayModelRequestIgnoresNonModelMutation(t *testing.T) {
	req := newAIGatewayModelMutationRequest(t, http.MethodPost, "/v1/ai-gateways/gw-1/providers", `{
		"target_models": [{"name": "gpt-4o-mini"}]
	}`)

	require.NoError(t, addTargetsToAIGatewayModelRequest(req))

	payload := readAIGatewayModelRequestPayload(t, req)
	require.NotContains(t, payload, "targets")
}

func newAIGatewayModelMutationRequest(t *testing.T, method string, path string, body string) *http.Request {
	t.Helper()

	req, err := http.NewRequest(method, "https://example.test"+path, strings.NewReader(body))
	require.NoError(t, err)
	return req
}

func readAIGatewayModelRequestPayload(t *testing.T, req *http.Request) map[string]json.RawMessage {
	t.Helper()

	body, err := readAndRestoreRequestBody(req)
	require.NoError(t, err)

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &payload))
	return payload
}
