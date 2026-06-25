package executor

import (
	"context"
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
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

	var req kkComps.CreateAIGatewayProviderRequest
	require.NoError(t, adapter.MapCreateFields(context.Background(), nil, fields, &req))
	require.NotNil(t, req.AIGatewayProviderOpenai)
	require.Equal(t, "openai-provider", req.AIGatewayProviderOpenai.Name)
	require.Equal(t, "OpenAI Provider", req.AIGatewayProviderOpenai.DisplayName)
	require.Equal(t, "platform", req.AIGatewayProviderOpenai.Labels["team"])

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
