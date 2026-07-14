package executor

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestAIGatewayProviderAdapterMapCreateFieldsBuildsSDKUnion(t *testing.T) {
	t.Parallel()

	adapter := NewAIGatewayProviderAdapter(nil)
	fields := map[string]any{
		planner.FieldName:        "openai-provider",
		planner.FieldType:        "openai",
		planner.FieldDisplayName: "OpenAI Provider",
		planner.FieldConfig: map[string]any{
			"auth": map[string]any{
				"type": "basic",
				"headers": []any{
					map[string]any{"name": "Authorization", "value": "Bearer token"},
				},
			},
		},
		planner.FieldLabels: map[string]string{"team": "platform"},
	}

	var req kkComps.CreateAIGatewayModelProviderRequest
	require.NoError(t, adapter.MapCreateFields(t.Context(), nil, fields, &req))
	require.NotNil(t, req.AIGatewayModelProviderOpenai)
	require.Equal(t, "openai-provider", req.AIGatewayModelProviderOpenai.Name)
	require.Equal(t, "OpenAI Provider", req.AIGatewayModelProviderOpenai.DisplayName)
	require.Equal(t, "platform", req.AIGatewayModelProviderOpenai.Labels["team"])

	data, err := json.Marshal(req)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"type": "openai",
		"name": "openai-provider",
		"display_name": "OpenAI Provider",
		"labels": {"team": "platform"},
		"config": {
			"auth": {
				"type": "basic",
				"headers": [{"name": "Authorization", "value": "Bearer token"}]
			}
		}
	}`, string(data))
}

func TestAIGatewayProviderAdapterRejectsFieldsDiscardedBySDK(t *testing.T) {
	t.Parallel()

	adapter := NewAIGatewayProviderAdapter(nil)
	fields := map[string]any{
		planner.FieldName:        "openai-provider",
		planner.FieldType:        "openai",
		planner.FieldDisplayName: "OpenAI Provider",
		planner.FieldConfig: map[string]any{
			"auth": map[string]any{
				"type":         "basic",
				"header_name":  "Authorization",
				"header_value": "secret-value-must-not-be-reported",
			},
		},
	}

	var create kkComps.CreateAIGatewayModelProviderRequest
	err := adapter.MapCreateFields(t.Context(), nil, fields, &create)
	require.Error(t, err)
	require.ErrorContains(t, err, "config.auth.header_name")
	require.ErrorContains(t, err, "config.auth.header_value")
	require.NotContains(t, err.Error(), "secret-value-must-not-be-reported")

	var update kkComps.UpdateAIGatewayModelProviderRequest
	err = adapter.MapUpdateFields(t.Context(), nil, fields, &update, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "config.auth.header_name")
	require.ErrorContains(t, err, "config.auth.header_value")
	require.NotContains(t, err.Error(), "secret-value-must-not-be-reported")
}

func TestAIGatewayProviderScaffoldMapsToSDKRequest(t *testing.T) {
	t.Parallel()

	subject, err := resources.ResolveExplainSubject("ai_gateway_model_provider")
	require.NoError(t, err)
	scaffold, err := resources.RenderScaffoldYAML(subject)
	require.NoError(t, err)

	var document map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(scaffold), &document))
	providers, ok := document["ai_gateway_model_providers"].([]any)
	require.True(t, ok)
	require.Len(t, providers, 1)
	fields, ok := providers[0].(map[string]any)
	require.True(t, ok)
	delete(fields, resources.SchemaFieldRef)
	delete(fields, resources.SchemaFieldAIGateway)

	var request kkComps.CreateAIGatewayModelProviderRequest
	require.NoError(t, NewAIGatewayProviderAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request))
	require.NotNil(t, request.AIGatewayModelProviderOpenai)
	require.Equal(t, "Authorization", request.AIGatewayModelProviderOpenai.Config.Auth.Headers[0].Name)
	require.Equal(
		t,
		"Bearer ${MODEL_PROVIDER_API_KEY}",
		*request.AIGatewayModelProviderOpenai.Config.Auth.Headers[0].Value,
	)
}
