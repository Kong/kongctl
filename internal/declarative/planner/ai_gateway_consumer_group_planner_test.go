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
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConsumerGroupPlannerCreatesChildForExistingGateway(t *testing.T) {
	group := testAIGatewayConsumerGroupResource(t, nil)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayConsumerGroupsAPI: &testAIGatewayConsumerGroupAPI{},
	})
	rs := testAIGatewayConsumerGroupResourceSet(group)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayConsumerGroup, change.ResourceType)
	require.Equal(t, "premium-support-users", change.ResourceRef)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
	require.Equal(t, "support-gateway", change.Parent.Ref)
	require.Equal(t, "Premium Support Users", change.Fields[FieldDisplayName])
}

func TestAIGatewayConsumerGroupPlannerUpdatesExistingGroup(t *testing.T) {
	group := testAIGatewayConsumerGroupResource(t, nil)
	group.DisplayName = "Premium Support Users Updated"
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayConsumerGroupsAPI: &testAIGatewayConsumerGroupAPI{
			groups: []kkComps.AIGatewayConsumerGroup{
				testAIGatewayConsumerGroup("group-id", "premium-support-users", nil),
			},
		},
	})
	rs := testAIGatewayConsumerGroupResourceSet(group)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionUpdate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayConsumerGroup, change.ResourceType)
	require.Equal(t, "group-id", change.ResourceID)
	require.Equal(t, "Premium Support Users Updated", change.Fields[FieldDisplayName])
	require.Contains(t, change.ChangedFields, FieldDisplayName)
}

func TestAIGatewayConsumerGroupPlannerSyncDeletesScopedGroups(t *testing.T) {
	scope := resources.NewSyncScope()
	scope.AddRoot(resources.ResourceTypeAIGateway)
	scope.AddChild(resources.ResourceTypeAIGateway, "support-gateway", resources.ResourceTypeAIGatewayConsumerGroup)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayConsumerGroupsAPI: &testAIGatewayConsumerGroupAPI{
			groups: []kkComps.AIGatewayConsumerGroup{
				testAIGatewayConsumerGroup("group-id", "premium-support-users", nil),
			},
		},
	})
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{testAIGatewayResource()},
		SyncScope:  scope,
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionDelete, change.Action)
	require.Equal(t, ResourceTypeAIGatewayConsumerGroup, change.ResourceType)
	require.Equal(t, "group-id", change.ResourceID)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
}

func TestAIGatewayConsumerGroupPlannerDependsOnPolicyCreate(t *testing.T) {
	policy := testAIGatewayPolicyResource(t)
	group := testAIGatewayConsumerGroupResource(t, []string{tags.RefPlaceholderPrefix + "mask-sensitive-data#id"})
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{},
	})
	rs := &resources.ResourceSet{
		AIGateways:              []resources.AIGatewayResource{testAIGatewayResource()},
		AIGatewayPolicies:       []resources.AIGatewayPolicyResource{policy},
		AIGatewayConsumerGroups: []resources.AIGatewayConsumerGroupResource{group},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)

	gatewayCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGateway, "support-gateway")
	policyCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayPolicy, "mask-sensitive-data")
	groupCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayConsumerGroup, "premium-support-users")

	require.Contains(t, policyCreate.DependsOn, gatewayCreate.ID)
	require.Contains(t, groupCreate.DependsOn, policyCreate.ID)
	require.Equal(t, resources.UnknownReferenceID, groupCreate.References[FieldPolicies+".0"].ID)
	require.Equal(t, tags.RefPlaceholderPrefix+"mask-sensitive-data#id", groupCreate.References[FieldPolicies+".0"].Ref)
}

func TestAIGatewayConsumerGroupPlannerResolvesExistingPolicyRef(t *testing.T) {
	policy := testAIGatewayPolicyResource(t)
	group := testAIGatewayConsumerGroupResource(t, []string{tags.RefPlaceholderPrefix + "mask-sensitive-data#id"})
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayPoliciesAPI: &testAIGatewayPolicyAPI{
			policies: []kkComps.AIGatewayPolicy{testAIGatewayPolicy()},
		},
		AIGatewayConsumerGroupsAPI: &testAIGatewayConsumerGroupAPI{},
	})
	rs := &resources.ResourceSet{
		AIGateways:              []resources.AIGatewayResource{testAIGatewayResource()},
		AIGatewayPolicies:       []resources.AIGatewayPolicyResource{policy},
		AIGatewayConsumerGroups: []resources.AIGatewayConsumerGroupResource{group},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)

	groupCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayConsumerGroup, "premium-support-users")
	require.NotContains(t, groupCreate.DependsOn, "policy-id")
	require.Equal(t, "policy-id", groupCreate.References[FieldPolicies+".0"].ID)
}

func TestAIGatewayConsumerGroupPlannerPolicyRefNoopForExistingGroup(t *testing.T) {
	for _, currentPolicyRef := range []string{"policy-id", "mask-sensitive-data"} {
		t.Run(currentPolicyRef, func(t *testing.T) {
			policy := testAIGatewayPolicyResource(t)
			group := testAIGatewayConsumerGroupResource(t, []string{tags.RefPlaceholderPrefix + "mask-sensitive-data#id"})
			client := state.NewClient(state.ClientConfig{
				AIGatewayAPI: &testAIGatewayAPI{
					gateways: []kkComps.AIGateway{testAIGateway()},
				},
				AIGatewayPoliciesAPI: &testAIGatewayPolicyAPI{
					policies: []kkComps.AIGatewayPolicy{testAIGatewayPolicy()},
				},
				AIGatewayConsumerGroupsAPI: &testAIGatewayConsumerGroupAPI{
					groups: []kkComps.AIGatewayConsumerGroup{
						testAIGatewayConsumerGroup("group-id", "premium-support-users", []string{currentPolicyRef}),
					},
				},
			})
			rs := &resources.ResourceSet{
				AIGateways:              []resources.AIGatewayResource{testAIGatewayResource()},
				AIGatewayPolicies:       []resources.AIGatewayPolicyResource{policy},
				AIGatewayConsumerGroups: []resources.AIGatewayConsumerGroupResource{group},
			}

			plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
			require.NoError(t, err)
			require.Empty(t, plan.Changes)
		})
	}
}

func testAIGatewayResource() resources.AIGatewayResource {
	return resources.AIGatewayResource{
		BaseResource: resources.BaseResource{
			Ref:     "support-gateway",
			Kongctl: &resources.KongctlMeta{Namespace: new("default")},
		},
		CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
			Name:        "support-gateway",
			DisplayName: "Support Gateway",
		},
	}
}

func testAIGatewayConsumerGroupResourceSet(
	group resources.AIGatewayConsumerGroupResource,
) *resources.ResourceSet {
	return &resources.ResourceSet{
		AIGateways:              []resources.AIGatewayResource{testAIGatewayResource()},
		AIGatewayConsumerGroups: []resources.AIGatewayConsumerGroupResource{group},
	}
}

func testAIGatewayConsumerGroupResource(
	t *testing.T,
	policies []string,
) resources.AIGatewayConsumerGroupResource {
	t.Helper()
	payload := map[string]any{
		"ref":          "premium-support-users",
		"ai_gateway":   "support-gateway",
		"name":         "premium-support-users",
		"display_name": "Premium Support Users",
	}
	if policies != nil {
		payload[FieldPolicies] = policies
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var group resources.AIGatewayConsumerGroupResource
	require.NoError(t, json.Unmarshal(data, &group))
	return group
}

func testAIGatewayConsumerGroup(id string, name string, policies []string) kkComps.AIGatewayConsumerGroup {
	return kkComps.AIGatewayConsumerGroup{
		ID:          id,
		Name:        name,
		DisplayName: "Premium Support Users",
		Policies:    policies,
	}
}

type testAIGatewayConsumerGroupAPI struct {
	groups []kkComps.AIGatewayConsumerGroup
}

func (t *testAIGatewayConsumerGroupAPI) ListAiGatewayConsumerGroups(
	context.Context,
	kkOps.ListAiGatewayConsumerGroupsRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayConsumerGroupsResponse, error) {
	return &kkOps.ListAiGatewayConsumerGroupsResponse{
		ListAIGatewayConsumerGroupsResponse: &kkComps.ListAIGatewayConsumerGroupsResponse{
			Data: t.groups,
		},
	}, nil
}

func (t *testAIGatewayConsumerGroupAPI) CreateAiGatewayConsumerGroup(
	context.Context,
	string,
	kkComps.CreateAIGatewayConsumerGroupRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayConsumerGroupResponse, error) {
	return nil, nil
}

func (t *testAIGatewayConsumerGroupAPI) GetAiGatewayConsumerGroup(
	_ context.Context,
	_ string,
	groupID string,
	_ ...kkOps.Option,
) (*kkOps.GetAiGatewayConsumerGroupResponse, error) {
	for _, group := range t.groups {
		if group.ID == groupID || group.Name == groupID {
			return &kkOps.GetAiGatewayConsumerGroupResponse{AIGatewayConsumerGroup: &group}, nil
		}
	}
	return &kkOps.GetAiGatewayConsumerGroupResponse{}, nil
}

func (t *testAIGatewayConsumerGroupAPI) UpdateAiGatewayConsumerGroup(
	context.Context,
	kkOps.UpdateAiGatewayConsumerGroupRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayConsumerGroupResponse, error) {
	return nil, nil
}

func (t *testAIGatewayConsumerGroupAPI) DeleteAiGatewayConsumerGroup(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayConsumerGroupResponse, error) {
	return nil, nil
}
