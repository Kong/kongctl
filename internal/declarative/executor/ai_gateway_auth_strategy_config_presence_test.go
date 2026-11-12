package executor

import (
	"encoding/json"
	"maps"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
	c "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayOIDCConfigPresenceThroughSavedPlanAndMetadataStamp(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra map[string]any
	}{
		{name: "omitted"},
		{name: "null", extra: map[string]any{"upstream_access_token_header": nil}},
		{name: "explicit-default", extra: map[string]any{"upstream_access_token_header": "authorization:bearer"}},
		{name: "custom", extra: map[string]any{"scopes": []any{}, "upstream_access_token_header": "x-custom-token"}},
	} {
		for _, verb := range []string{"create", "update"} {
			t.Run(verb+"/"+tc.name, func(t *testing.T) {
				config := map[string]any{
					"issuer": "https://example.com", "cache_tokens_salt": "test-salt",
					"auth_methods": []any{"bearer"}, "hide_credentials": false,
				}
				maps.Copy(config, tc.extra)
				change := planner.PlannedChange{Fields: map[string]any{
					planner.FieldName: "probe", planner.FieldDisplayName: "Probe", planner.FieldType: "openid-connect",
					planner.FieldConfig: config, planner.FieldManagedBy: resources.BuildChildManagedBy("presence", false, "dev"),
				}}
				data, err := json.Marshal(change)
				require.NoError(t, err)
				var saved planner.PlannedChange
				require.NoError(t, json.Unmarshal(data, &saved))
				capture := &realmsCapture{}
				client := sdk.New(sdk.WithClient(capture), sdk.WithServerURL("https://probe.invalid"))
				stateClient := state.NewClient(state.ClientConfig{AIGatewayAuthStrategiesAPI: client.AIGatewayAuthStrategies})
				adapter := NewAIGatewayAuthStrategyAdapter(stateClient)
				if verb == "create" {
					var request c.CreateAIGatewayAuthStrategyRequest
					require.NoError(t, adapter.MapCreateFields(t.Context(), nil, saved.Fields, &request))
					_, err = stateClient.CreateAIGatewayAuthStrategy(t.Context(), "gateway", request, "presence")
				} else {
					var request c.UpdateAIGatewayAuthStrategyRequest
					require.NoError(t, adapter.MapUpdateFields(t.Context(), nil, saved.Fields, &request, nil))
					_, err = stateClient.UpdateAIGatewayAuthStrategy(t.Context(), "gateway", "probe", request, "presence")
				}
				require.Error(t, err) // The capture client returns a synthetic HTTP 400 after recording the request.
				require.NotEmpty(t, capture.body)
				var body map[string]any
				require.NoError(t, json.Unmarshal(capture.body, &body))
				require.Equal(t, config, body[planner.FieldConfig])
				require.Contains(t, body, planner.FieldManagedBy)
			})
		}
	}
}
