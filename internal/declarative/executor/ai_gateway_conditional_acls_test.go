package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConditionalACLs(t *testing.T) {
	for _, field := range []string{"allow_when", "deny_when"} {
		for _, kind := range []string{"model", "api", "a2a"} {
			for _, nested := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/nested=%t", field, kind, nested), func(t *testing.T) {
					acls := map[string]any{field: []any{"http.headers.x_end_user_id == null"}}
					child := map[string]any{
						"ref": "child", "name": "child", "display_name": "Child", "type": kind,
						"access": map[string]any{"acls": acls},
					}
					collection := "models"
					if kind == "a2a" {
						collection = "agents"
						child["config"] = map[string]any{"url": "https://agent.example.com"}
					} else {
						child["formats"] = []any{map[string]any{"type": "openai"}}
						capability := "generate"
						if kind == "api" {
							capability = "files"
						}
						child["capabilities"] = []any{capability}
						child["config"] = map[string]any{"route": map[string]any{"paths": []any{"/repro"}}}
						child["targets"] = []any{map[string]any{
							"name": "gpt-4o-mini", "provider": "some-provider", "config": map[string]any{"type": "openai"},
						}}
					}
					gateway := map[string]any{"ref": "gateway", "name": "gateway", "display_name": "Gateway"}
					document := map[string]any{"ai_gateways": []any{gateway}}
					rootCollection := "ai_gateway_" + collection
					if nested {
						gateway[collection] = []any{child}
					} else {
						child["ai_gateway"] = "gateway"
						document[rootCollection] = []any{child}
					}
					path := filepath.Join(t.TempDir(), "manifest.yaml")
					load := func() *resources.ResourceSet {
						data, err := json.Marshal(document)
						require.NoError(t, err)
						require.NoError(t, os.WriteFile(path, data, 0o600))
						rs, err := loader.New().LoadFile(path)
						require.NoError(t, err)
						return rs
					}
					rs := load()
					var payload map[string]any
					var requests []any
					if kind == "a2a" {
						require.Len(t, rs.AIGatewayAgents, 1)
						var err error
						payload, err = rs.AIGatewayAgents[0].MutablePayloadMap()
						require.NoError(t, err)
						adapter := NewAIGatewayAgentAdapter(nil)
						var create kkComps.CreateAIGatewayAgentRequest
						var update kkComps.UpdateAIGatewayAgentRequest
						require.NoError(t, adapter.MapCreateFields(t.Context(), nil, payload, &create))
						require.NoError(t, adapter.MapUpdateFields(t.Context(), nil, payload, &update, nil))
						requests = []any{create, update}
					} else {
						require.Len(t, rs.AIGatewayModels, 1)
						var err error
						payload, err = rs.AIGatewayModels[0].MutablePayloadMap()
						require.NoError(t, err)
						adapter := NewAIGatewayModelAdapter(nil)
						var create kkComps.CreateAIGatewayModelRequest
						var update kkComps.UpdateAIGatewayModelRequest
						require.NoError(t, adapter.MapCreateFields(t.Context(), nil, payload, &create))
						require.NoError(t, adapter.MapUpdateFields(t.Context(), nil, payload, &update, nil))
						requests = []any{create, update}
					}
					require.Equal(t, acls, payload[planner.FieldAccess].(map[string]any)[planner.FieldACLS])
					for _, request := range requests {
						data, err := json.Marshal(request)
						require.NoError(t, err)
						var actual map[string]any
						require.NoError(t, json.Unmarshal(data, &actual))
						require.Equal(t, acls, actual[planner.FieldAccess].(map[string]any)[planner.FieldACLS])
					}
					// Exercise the response conversion used by dump and reload its output.
					payload[planner.FieldID] = "77426bee-2bca-4005-81af-284868fd3038"
					payload["created_at"] = "2026-01-01T00:00:00Z"
					payload["updated_at"] = "2026-01-01T00:00:00Z"
					data, err := json.Marshal(payload)
					require.NoError(t, err)
					delete(gateway, collection)
					if kind == "a2a" {
						var response kkComps.AIGatewayAgent
						require.NoError(t, json.Unmarshal(data, &response))
						dumped, err := resources.AIGatewayAgentResourceFromResponse("gateway", response)
						require.NoError(t, err)
						document[rootCollection] = []any{dumped}
						payload, err = load().AIGatewayAgents[0].MutablePayloadMap()
						require.NoError(t, err)
					} else {
						var response kkComps.AIGatewayModel
						require.NoError(t, json.Unmarshal(data, &response))
						dumped, err := resources.AIGatewayModelResourceFromResponse("gateway", response)
						require.NoError(t, err)
						document[rootCollection] = []any{dumped}
						payload, err = load().AIGatewayModels[0].MutablePayloadMap()
						require.NoError(t, err)
					}
					require.Equal(t, acls, payload[planner.FieldAccess].(map[string]any)[planner.FieldACLS])
				})
			}
		}
	}
}
