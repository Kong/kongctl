package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

type consumeOrderAPI struct {
	helpers.EventGatewayConsumePolicyAPI
	policies []kkComps.EventGatewayPolicy
	chain    []string
}

func (a *consumeOrderAPI) ListEventGatewayVirtualClusterConsumePolicies(context.Context,
	kkOps.ListEventGatewayVirtualClusterConsumePoliciesRequest, ...kkOps.Option,
) (*kkOps.ListEventGatewayVirtualClusterConsumePoliciesResponse, error) {
	return &kkOps.ListEventGatewayVirtualClusterConsumePoliciesResponse{ListConsumePoliciesResponse: a.policies}, nil
}

func (a *consumeOrderAPI) GetEventGatewayVirtualClusterConsumePolicyChain(context.Context,
	kkOps.GetEventGatewayVirtualClusterConsumePolicyChainRequest, ...kkOps.Option,
) (*kkOps.GetEventGatewayVirtualClusterConsumePolicyChainResponse, error) {
	return &kkOps.GetEventGatewayVirtualClusterConsumePolicyChainResponse{
		EventGatewayPolicyChainResponse: &kkComps.EventGatewayPolicyChainResponse{Policies: a.chain},
	}, nil
}

func TestConsumeOrderAuthoritativeDriftAndConvergence(t *testing.T) {
	for _, parent := range []string{"", "schema-id"} {
		t.Run("parent="+parent, func(t *testing.T) {
			api := &consumeOrderAPI{}
			var desired []resources.EventGatewayConsumePolicyResource
			for _, name := range []string{"first", "second"} {
				desired = append(desired, consumePolicyResourceFromJSON(t, fmt.Sprintf(
					`{"ref":%q,"name":%q,"type":"skip_record","parent_policy_id":%q,"config":{}}`, name, name, parent,
				)))
				policy := kkComps.EventGatewayPolicy{ID: name + "-id", Name: new(name), Type: "skip_record"}
				if parent != "" {
					policy.ParentPolicyID = new(parent)
				}
				api.policies = append(api.policies, policy)
			}
			// The ordinary resource list is in desired order; only the authoritative
			// chain reveals the drift.
			api.chain = []string{"second-id", "first-id"}
			p := newTestPlanner()
			p.client = state.NewClient(state.ClientConfig{EventGatewayConsumePolicyAPI: api})
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
			require.NoError(t, p.planConsumePolicyChangesForExistingVirtualCluster(
				t.Context(), "default", "gw", "gw", "vc", "vc", "vc", desired, plan,
			))
			require.Len(t, plan.Changes, 1)
			require.Equal(t, ActionUpdate, plan.Changes[0].Action)
			require.Equal(t, []string{"second", "first"}, plan.Changes[0].ChangedFields[FieldPolicyOrder].Old)
			require.Equal(t, []string{"first", "second"}, plan.Changes[0].ChangedFields[FieldPolicyOrder].New)
			data, err := json.Marshal(plan)
			require.NoError(t, err)
			var saved Plan
			require.NoError(t, json.Unmarshal(data, &saved))
			require.Equal(t, plan.Changes[0].ConsumePolicyOrder, saved.Changes[0].ConsumePolicyOrder)
			api.chain = []string{"first-id", "second-id"}
			plan = NewPlan(CurrentPlanVersion, "test", PlanModeApply)
			require.NoError(t, p.planConsumePolicyChangesForExistingVirtualCluster(
				t.Context(), "default", "gw", "gw", "vc", "vc", "vc", desired, plan,
			))
			require.Empty(t, plan.Changes)
			api.chain = []string{"first-id"}
			require.ErrorContains(t, p.planConsumePolicyChangesForExistingVirtualCluster(
				t.Context(), "default", "gw", "gw", "vc", "vc", "vc", desired, plan,
			), "missing policies")
		})
	}
}

func TestConsumeCreatesSerializeOnlySiblings(t *testing.T) {
	p := newTestPlanner()
	var desired []resources.EventGatewayConsumePolicyResource
	for _, item := range []struct{ name, parent string }{
		{"a", ""}, {"child-a", "schema-id"}, {"b", ""}, {"child-b", "schema-id"},
	} {
		desired = append(desired, consumePolicyResourceFromJSON(t, fmt.Sprintf(
			`{"ref":%q,"name":%q,"type":"skip_record","parent_policy_id":%q,"config":{}}`, item.name, item.name, item.parent,
		)))
	}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
	require.NoError(t, p.planConsumePolicyCreatesForNewVirtualCluster("default", "gw", "vc", "vc", "", desired, plan))
	result, err := NewDependencyResolver().ResolveDependenciesWithGroups(plan.Changes)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{plan.Changes[0].ID, plan.Changes[1].ID}, result.ExecutionGroups[0])
	require.ElementsMatch(t, []string{plan.Changes[2].ID, plan.Changes[3].ID}, result.ExecutionGroups[1])
	require.Len(t, result.ExecutionGroups[2], 2)
	for _, change := range plan.Changes[4:] {
		require.NotNil(t, change.ConsumePolicyOrder)
		for _, create := range plan.Changes[:4] {
			require.Contains(t, change.DependsOn, create.ID)
		}
	}
}
