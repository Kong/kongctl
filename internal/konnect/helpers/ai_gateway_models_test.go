package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
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
			"config": {
				"route": {"model": {"body": {"model": ["support-gpt"]}}},
				"model": {"name_header": true}
			},
			"formats": [{"type": "openai"}],
			"targets": [{
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
	routeModel := kkComps.CreateAIGatewayModelAliasConfigAIGatewayModelAliasConfigBody(
		kkComps.AIGatewayModelAliasConfigBody{
			Body: map[string]any{"model": []string{"support-gpt"}},
		},
	)
	targetConfig := kkComps.CreateAIGatewayTargetConfigOpenai(kkComps.AIGatewayTargetOpenaiConfig{})
	req := kkComps.CreateCreateAIGatewayModelRequestModel(kkComps.AIGatewayModelModel{
		DisplayName: "Support GPT",
		Name:        "support-gpt",
		Config: kkComps.AIGatewayModelModelConfig{
			Route: kkComps.AIGatewayModelRouteConfig{Model: &routeModel},
		},
		Formats: []kkComps.AIGatewayModelFormat{
			{Type: &formatType},
		},
		Targets: []kkComps.AIGatewayTarget{
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
	require.Contains(t, requestBody, "targets")
	var config map[string]any
	require.NoError(t, json.Unmarshal(requestBody["config"], &config))
	route := config["route"].(map[string]any)
	require.Equal(t, map[string]any{
		"body": map[string]any{"model": []any{"support-gpt"}},
	}, route["model"])
	require.JSONEq(
		t,
		`[{
			"name":"gpt-4o-mini",
			"provider":"shared-openai",
			"config":{"type":"openai"},
			"weight":100,
			"allow_auth_override":false
		}]`,
		string(requestBody["targets"]),
	)
}
