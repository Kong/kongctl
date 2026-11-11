package executor

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

type consumeOrderingAPI struct {
	helpers.EventGatewayConsumePolicyAPI
	policies []kkComps.EventGatewayPolicy
	chain    []string
	moves    []string
	fail     bool
}

func (a *consumeOrderingAPI) ListEventGatewayVirtualClusterConsumePolicies(context.Context,
	kkOps.ListEventGatewayVirtualClusterConsumePoliciesRequest, ...kkOps.Option,
) (*kkOps.ListEventGatewayVirtualClusterConsumePoliciesResponse, error) {
	return &kkOps.ListEventGatewayVirtualClusterConsumePoliciesResponse{ListConsumePoliciesResponse: a.policies}, nil
}

func (a *consumeOrderingAPI) GetEventGatewayVirtualClusterConsumePolicyChain(context.Context,
	kkOps.GetEventGatewayVirtualClusterConsumePolicyChainRequest, ...kkOps.Option,
) (*kkOps.GetEventGatewayVirtualClusterConsumePolicyChainResponse, error) {
	return &kkOps.GetEventGatewayVirtualClusterConsumePolicyChainResponse{
		EventGatewayPolicyChainResponse: &kkComps.EventGatewayPolicyChainResponse{Policies: a.chain},
	}, nil
}

func (a *consumeOrderingAPI) MoveEventGatewayVirtualClusterConsumePolicy(_ context.Context,
	r kkOps.MoveEventGatewayVirtualClusterConsumePolicyRequest, _ ...kkOps.Option,
) (*kkOps.MoveEventGatewayVirtualClusterConsumePolicyResponse, error) {
	if a.fail {
		return nil, errors.New("ordering failed")
	}
	a.moves = append(a.moves, r.PolicyID)
	pos := slices.Index(a.chain, r.PolicyID)
	a.chain = slices.Delete(a.chain, pos, pos+1)
	a.chain = slices.Insert(a.chain, int(r.MoveEventGatewayPolicy.Index), r.PolicyID)
	return &kkOps.MoveEventGatewayVirtualClusterConsumePolicyResponse{}, nil
}

func TestConsumeOrderSavedPlanExecution(t *testing.T) {
	for _, parent := range []string{"", "parent-new-id"} {
		for _, fail := range []bool{false, true} {
			t.Run(parent+"/fail="+map[bool]string{false: "false", true: "true"}[fail], func(t *testing.T) {
				api := &consumeOrderingAPI{chain: []string{"extra-a", "second-new-id", "extra-b", "first-new-id"}, fail: fail}
				for _, item := range []struct{ id, name string }{
					{"extra-a", "extra-a"}, {"second-new-id", "second"}, {"extra-b", "extra-b"}, {"first-new-id", "first"},
				} {
					policy := kkComps.EventGatewayPolicy{ID: item.id, Name: new(item.name)}
					if parent != "" {
						policy.ParentPolicyID = new(parent)
					}
					api.policies = append(api.policies, policy)
				}
				intent := &planner.ConsumePolicyOrder{Names: []string{"first", "second"}}
				if parent != "" {
					api.policies = append(api.policies, kkComps.EventGatewayPolicy{ID: parent, Name: new("schema")})
					api.chain = append(api.chain, parent)
					intent.ParentName = "schema"
				}
				plan := planner.NewPlan(planner.CurrentPlanVersion, "test", planner.PlanModeApply)
				plan.AddChange(planner.PlannedChange{
					ID: "order", ResourceType: planner.ResourceTypeEventGatewayConsumePolicy,
					ResourceID: "vc", ResourceRef: "order", Action: planner.ActionUpdate, ConsumePolicyOrder: intent,
					References: map[string]planner.ReferenceInfo{
						planner.FieldEventGatewayID: {ID: "gw"}, planner.FieldEventGatewayVirtualClusterID: {ID: "vc"},
					},
				})
				plan.ExecutionOrder = []string{"order"}
				data, err := json.Marshal(plan)
				require.NoError(t, err)
				var saved planner.Plan
				require.NoError(t, json.Unmarshal(data, &saved))
				e := New(state.NewClient(state.ClientConfig{EventGatewayConsumePolicyAPI: api}), nil, false)
				result := e.Execute(t.Context(), &saved)
				if fail {
					require.Equal(t, 1, result.FailureCount)
					require.Zero(t, result.SuccessCount)
					require.Contains(t, result.Errors[0].Error, "ordering failed")
					return
				}
				require.Zero(t, result.FailureCount, "%+v", result.Errors)
				require.Equal(t, []string{"first-new-id", "second-new-id", "extra-a", "extra-b"}, api.chain[:4])
				require.Equal(t, []string{"first-new-id", "second-new-id"}, api.moves)
				api.moves = nil
				result = e.Execute(t.Context(), &saved)
				require.Zero(t, result.FailureCount)
				require.Empty(t, api.moves)
			})
		}
	}
}
