package planner

import (
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

// Empty auth lists disappear during SDK serialization. Exercise the real diff
// builders to ensure the dependency resolver still recognizes full detachment.
func TestAIGatewayAuthStrategyFullDetachFromDesiredPayload(t *testing.T) {
	for _, kind := range []string{ResourceTypeAIGatewayAgent, ResourceTypeAIGatewayModel, ResourceTypeAIGatewayMCPServer} {
		for _, mode := range []string{"empty list", "omitted list", "omitted access"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				cfg := authStrategyOrderClientConfig(t, kind, "old-authStrategy")
				p := NewPlanner(state.NewClient(cfg), slog.Default())
				p.resources = &resources.ResourceSet{}
				var current any
				switch kind {
				case ResourceTypeAIGatewayAgent:
					current = cfg.AIGatewayAgentsAPI.(*testAIGatewayAgentAPI).agents[0]
				case ResourceTypeAIGatewayModel:
					current = cfg.AIGatewayModelAPI.(*testAIGatewayModelAPI).models[0]
				case ResourceTypeAIGatewayMCPServer:
					current = cfg.AIGatewayMCPServersAPI.(*testAIGatewayMCPServerAPI).servers[0]
				}
				data, err := json.Marshal(current)
				require.NoError(t, err)
				var payload map[string]any
				require.NoError(t, json.Unmarshal(data, &payload))
				delete(payload, "id")
				delete(payload, "created_at")
				delete(payload, "updated_at")
				payload["ref"] = "old-user"
				payload["ai_gateway"] = "support-gateway"
				access := payload[FieldAccess].(map[string]any)
				switch mode {
				case "empty list":
					access[FieldAuthStrategies] = []string{}
				case "omitted list":
					delete(access, FieldAuthStrategies)
				case "omitted access":
					delete(payload, FieldAccess)
				}
				data, err = json.Marshal(payload)
				require.NoError(t, err)
				var changed map[string]FieldChange
				var fields map[string]any
				var update bool
				switch kind {
				case ResourceTypeAIGatewayAgent:
					var desired resources.AIGatewayAgentResource
					require.NoError(t, json.Unmarshal(data, &desired))
					observed := cfg.AIGatewayAgentsAPI.(*testAIGatewayAgentAPI).agents[0]
					update, fields, changed, err = p.shouldUpdateAIGatewayAgent(
						state.AIGatewayAgent{AIGatewayAgent: observed},
						desired,
					)
				case ResourceTypeAIGatewayModel:
					var desired resources.AIGatewayModelResource
					require.NoError(t, json.Unmarshal(data, &desired))
					observed := cfg.AIGatewayModelAPI.(*testAIGatewayModelAPI).models[0]
					update, fields, changed, err = p.shouldUpdateAIGatewayModel(
						state.AIGatewayModel{AIGatewayModel: observed},
						desired,
					)
				case ResourceTypeAIGatewayMCPServer:
					var desired resources.AIGatewayMCPServerResource
					require.NoError(t, json.Unmarshal(data, &desired))
					observed := cfg.AIGatewayMCPServersAPI.(*testAIGatewayMCPServerAPI).servers[0]
					update, fields, changed, err = p.shouldUpdateAIGatewayMCPServer(
						state.AIGatewayMCPServer{AIGatewayMCPServer: observed},
						desired,
					)
				}
				require.NoError(t, err)
				require.True(t, update)
				plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
				plan.AddChange(PlannedChange{
					ID: "delete-auth", ResourceType: ResourceTypeAIGatewayAuthStrategy, Action: ActionDelete,
					ResourceID: "authStrategy-id", Fields: map[string]any{FieldName: "old-authStrategy"},
					Namespace: "default", Parent: &ParentInfo{Ref: "support-gateway"},
				})
				plan.AddChange(PlannedChange{
					ID: "detach-user", ResourceType: kind, Action: ActionUpdate, ResourceID: "user-id",
					Fields: fields, ChangedFields: changed, Namespace: "default", Parent: &ParentInfo{Ref: "support-gateway"},
				})
				require.NoError(
					t,
					p.resolveAIGatewayAuthStrategyDeletes(
						t.Context(),
						"default",
						"support-gateway",
						"gateway-id",
						plan,
					),
				)
				require.Equal(t, []string{"detach-user"}, plan.Changes[0].DependsOn)
			})
		}
	}
}
