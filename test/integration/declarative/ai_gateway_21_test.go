//go:build integration

package declarative_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/executor"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestAIGateway21FieldsSurviveDeclarativeMapping(t *testing.T) {
	set, err := loader.New().LoadFile(filepath.Join("testdata", "ai-gateway-21", "config.yaml"))
	require.NoError(t, err)
	validateAIGatewayScenarioExplainSchemas(t, set)
	t.Run("runtime settings", func(t *testing.T) {
		gateway := set.AIGateways[0]
		payload := aiGateway21Payload(t, gateway)
		require.Equal(t, "2.1", payload["min_runtime_version"])
		require.Equal(t, false, payload["runtime_auto_upgrade"])
		fields := map[string]any{
			planner.FieldName:               gateway.Name,
			planner.FieldDisplayName:        gateway.DisplayName,
			planner.FieldMinRuntimeVersion:  *gateway.MinRuntimeVersion,
			planner.FieldRuntimeAutoUpgrade: *gateway.RuntimeAutoUpgrade,
		}
		adapter := executor.NewAIGatewayAdapter(nil)
		var create kkComps.CreateAIGatewayRequest
		require.NoError(t, adapter.MapCreateFields(t.Context(), &executor.ExecutionContext{}, fields, &create))
		require.Equal(t, gateway.MinRuntimeVersion, create.MinRuntimeVersion)
		require.Equal(t, gateway.RuntimeAutoUpgrade, create.RuntimeAutoUpgrade)
		var update kkComps.UpdateAIGatewayRequest
		require.NoError(t, adapter.MapUpdateFields(t.Context(), nil, fields, &update, nil))
		require.Equal(t, gateway.MinRuntimeVersion, update.MinRuntimeVersion)
		require.Equal(t, gateway.RuntimeAutoUpgrade, update.RuntimeAutoUpgrade)
	})

	t.Run("model costs", func(t *testing.T) {
		fields, err := set.AIGatewayModels[0].MutablePayloadMap()
		require.NoError(t, err)
		check := func(value any) {
			payload := aiGateway21Payload(t, value)
			config := payload["targets"].([]any)[0].(map[string]any)["config"].(map[string]any)
			require.Equal(t, []any{map[string]any{"modal": "text", "cost": 2.5}}, config["input_cost_list"])
			require.Equal(t, []any{map[string]any{"modal": "audio", "cost": float64(10)}}, config["output_cost_list"])
			require.Equal(t, []any{map[string]any{"modal": "image", "cost": float64(0)}}, config["cache_read_cost_list"])
		}
		check(fields)
		adapter := executor.NewAIGatewayModelAdapter(nil)
		var create kkComps.CreateAIGatewayModelRequest
		require.NoError(t, adapter.MapCreateFields(t.Context(), nil, fields, &create))
		check(create)
		var update kkComps.UpdateAIGatewayModelRequest
		require.NoError(t, adapter.MapUpdateFields(t.Context(), nil, fields, &update, nil))
		check(update)
		var response kkComps.AIGatewayModel
		aiGateway21Response(t, create, &response)
		dumped, err := resources.AIGatewayModelResourceFromResponse("sdk-68", response)
		require.NoError(t, err)
		check(dumped)
	})

	t.Run("policy condition", func(t *testing.T) {
		fields, err := set.AIGatewayPolicies[0].MutablePayloadMap()
		require.NoError(t, err)
		check := func(value any) {
			require.Equal(t, "http.path == '/v1/chat/completions'", aiGateway21Payload(t, value)["condition"])
		}
		check(fields)
		adapter := executor.NewAIGatewayPolicyAdapter(nil)
		var create kkComps.CreateAIGatewayPolicyRequest
		require.NoError(t, adapter.MapCreateFields(t.Context(), nil, fields, &create))
		check(create)
		var update kkComps.UpdateAIGatewayPolicyRequest
		require.NoError(t, adapter.MapUpdateFields(t.Context(), nil, fields, &update, nil))
		check(update)
		var response kkComps.AIGatewayPolicy
		aiGateway21Response(t, create, &response)
		dumped, err := resources.AIGatewayPolicyResourceFromResponse("sdk-68", response)
		require.NoError(t, err)
		check(dumped)
	})

	for _, server := range set.AIGatewayMCPServers {
		t.Run(server.Ref, func(t *testing.T) {
			fields, err := server.MutablePayloadMap()
			require.NoError(t, err)
			check := func(value any) {
				payload := aiGateway21Payload(t, value)
				config := payload["config"].(map[string]any)
				if server.Ref == "upstream" {
					require.Equal(t, "2025-11-25", config["server"].(map[string]any)["upstream_protocol_version"])
					return
				}
				versions := []any{"2026-07-28"}
				toolsTTL, discoverTTL := float64(60000), float64(0)
				if server.Ref == "conversion" {
					versions = append(versions, "2025-11-25")
					toolsTTL, discoverTTL = discoverTTL, toolsTTL
				}
				require.Equal(t, versions, config["allowed_versions"])
				require.Equal(t, map[string]any{
					"tools_list": map[string]any{"ttl_ms": toolsTTL, "cache_scope": "private"},
					"discover":   map[string]any{"ttl_ms": discoverTTL, "cache_scope": "public"},
				}, config["cache"])
			}
			check(fields)
			adapter := executor.NewAIGatewayMCPServerAdapter(nil)
			var create kkComps.CreateAIGatewayMCPServerRequest
			require.NoError(t, adapter.MapCreateFields(t.Context(), nil, fields, &create))
			check(create)
			var update kkComps.UpdateAIGatewayMCPServerRequest
			require.NoError(t, adapter.MapUpdateFields(t.Context(), nil, fields, &update, nil))
			check(update)
			var response kkComps.AIGatewayMCPServer
			aiGateway21Response(t, create, &response)
			dumped, err := resources.AIGatewayMCPServerResourceFromResponse("sdk-68", response)
			require.NoError(t, err)
			check(dumped)
		})
	}
}

func aiGateway21Payload(t *testing.T, value any) map[string]any {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(data, &payload))
	return payload
}

func aiGateway21Response(t *testing.T, request, response any) {
	t.Helper()
	payload := aiGateway21Payload(t, request)
	payload["id"] = "11111111-1111-4111-8111-111111111111"
	payload["created_at"] = "2026-09-22T00:00:00Z"
	payload["updated_at"] = "2026-09-22T00:00:00Z"
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, response))
}
