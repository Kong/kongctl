package planner

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConditionalACLPlans(t *testing.T) {
	for _, field := range []string{"allow_when", "deny_when"} {
		for _, action := range []ActionType{ActionCreate, ActionUpdate} {
			t.Run(field+"/"+string(action), func(t *testing.T) {
				acls := map[string]any{field: []any{"http.headers.x_end_user_id == null"}}
				access, err := json.Marshal(map[string]any{"acls": acls})
				require.NoError(t, err)
				model := testAIGatewayModelResource(t)
				agent := testAIGatewayAgentResource(t, nil)
				require.NoError(t, json.Unmarshal(access, &model.AIGatewayModelModel.Access))
				require.NoError(t, json.Unmarshal(access, &agent.Access))
				models := &conditionalACLModelAPI{}
				agents := &testAIGatewayAgentAPI{}
				if action == ActionUpdate {
					models.models = []kkComps.AIGatewayModel{testAIGatewayModel("model-id", "support-gpt")}
					agents.agents = []kkComps.AIGatewayAgent{testAIGatewayAgent(nil)}
				}
				client := state.NewClient(state.ClientConfig{
					AIGatewayAPI:      &testAIGatewayAPI{gateways: []kkComps.AIGateway{testAIGateway()}},
					AIGatewayModelAPI: models, AIGatewayAgentsAPI: agents,
				})
				rs := testAIGatewayAgentResourceSet(agent)
				rs.AIGatewayModels = []resources.AIGatewayModelResource{model}
				plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
				require.NoError(t, err)
				require.Len(t, plan.Changes, 2)
				for _, change := range plan.Changes {
					require.Equal(t, action, change.Action)
					require.Contains(t, []string{ResourceTypeAIGatewayModel, ResourceTypeAIGatewayAgent}, change.ResourceType)
					require.Equal(t, acls, change.Fields[FieldAccess].(map[string]any)[FieldACLS])
					if action == ActionUpdate {
						require.Contains(t, change.ChangedFields, FieldAccess)
					}
				}
			})
		}
	}
}

type conditionalACLModelAPI struct {
	testAIGatewayModelAPI
}

func (a *conditionalACLModelAPI) GetAiGatewayModel(
	_ context.Context,
	_ string,
	id string,
	_ ...kkOps.Option,
) (*kkOps.GetAiGatewayModelResponse, error) {
	for _, model := range a.models {
		if resources.AIGatewayModelID(model) == id {
			return &kkOps.GetAiGatewayModelResponse{AIGatewayModel: &model}, nil
		}
	}
	return &kkOps.GetAiGatewayModelResponse{}, nil
}
