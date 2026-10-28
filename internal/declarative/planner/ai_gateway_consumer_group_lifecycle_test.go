package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConsumerGroupPlannerRejectsMissingDetail(t *testing.T) {
	api := &missingConsumerGroupDetailAPI{groupNameIdentityAPI: &groupNameIdentityAPI{
		testAIGatewayConsumerGroupAPI: &testAIGatewayConsumerGroupAPI{
			groups: []kkComps.AIGatewayConsumerGroup{testAIGatewayConsumerGroup(nil)},
		},
	}}
	desired := testAIGatewayConsumerGroupResourceWithConsumers(t, []string{"support-user"})
	desired.Ref = "local-group-ref"
	rs := testAIGatewayConsumerGroupResourceSet(desired)
	p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayConsumerGroupsAPI: api}), slog.Default())
	p.resources = rs
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)

	err := p.planAIGatewayConsumerGroupChanges(t.Context(), "test-namespace", "support-gateway",
		"Gateway", "gateway-id", "", nil, rs.AIGatewayConsumerGroups, plan)

	require.ErrorContains(t, err, "disappeared during planning")
	require.Empty(t, plan.Changes)
	require.Equal(t, [][2]string{{"gateway-id", "group-id"}}, api.reads)
	require.Empty(t, api.membershipReads)
}

func TestAIGatewayConsumerGroupPlannerStopsSyncAtProtectedGroup(t *testing.T) {
	api := &groupNameIdentityAPI{testAIGatewayConsumerGroupAPI: &testAIGatewayConsumerGroupAPI{
		groups: []kkComps.AIGatewayConsumerGroup{
			{
				ID: "before-id", Name: "before",
				ManagedBy: resources.BuildChildManagedBy(nil, nil, "default", false, "dev"),
			},
			{
				ID: "protected-id", Name: "protected-group",
				ManagedBy: resources.BuildChildManagedBy(nil, nil, "default", true, "dev"), Labels: map[string]string{
					labels.ProtectedKey: labels.TrueValue,
				},
			},
			{
				ID: "after-id", Name: "after",
				ManagedBy: resources.BuildChildManagedBy(nil, nil, "default", false, "dev"),
			},
		},
	}}
	p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayConsumerGroupsAPI: api}), slog.Default())
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)

	err := p.planAIGatewayConsumerGroupChanges(t.Context(), "test-namespace", "support-gateway",
		"Gateway", "gateway-id", "", nil, nil, plan)

	require.ErrorContains(t, err, "protected")
	require.ErrorContains(t, err, "protected-group")
	require.Equal(t, []string{"gateway-id"}, api.lists)
	require.Empty(t, api.reads)
	require.Empty(t, api.membershipReads)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionDelete, change.Action)
	require.Equal(t, ResourceTypeAIGatewayConsumerGroup, change.ResourceType)
	require.Equal(t, "before-id", change.ResourceID)
	require.Equal(t, "before", change.ResourceRef)
	require.Equal(t, DefaultNamespace, change.Namespace)
	require.Equal(t, &ParentInfo{Ref: "support-gateway", ID: "gateway-id"}, change.Parent)
}

type missingConsumerGroupDetailAPI struct {
	*groupNameIdentityAPI
}

func (a *missingConsumerGroupDetailAPI) GetAiGatewayConsumerGroup(
	_ context.Context, gatewayID, groupID string, _ ...kkOps.Option,
) (*kkOps.GetAiGatewayConsumerGroupResponse, error) {
	a.reads = append(a.reads, [2]string{gatewayID, groupID})
	return &kkOps.GetAiGatewayConsumerGroupResponse{}, nil
}
