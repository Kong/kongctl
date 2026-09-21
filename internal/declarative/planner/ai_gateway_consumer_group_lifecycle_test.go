package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConsumerGroupPlannerRecreatesMissingDetail(t *testing.T) {
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

	require.NoError(t, err)
	require.Equal(t, []string{"gateway-id"}, api.lists)
	require.Equal(t, [][2]string{{"gateway-id", "group-id"}}, api.reads)
	require.Empty(t, api.membershipReads)
	require.Equal(t, "group-id", rs.AIGatewayConsumerGroups[0].GetKonnectID())
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayConsumerGroup, change.ResourceType)
	require.Equal(t, desired.Ref, change.ResourceRef)
	require.Empty(t, change.ResourceID)
	require.Equal(t, "test-namespace", change.Namespace)
	require.Equal(t, &ParentInfo{Ref: "support-gateway", ID: "gateway-id"}, change.Parent)
	require.Equal(t, desired.Name, change.Fields[FieldName])
	require.Equal(t, desired.DisplayName, change.Fields[FieldDisplayName])
	require.Equal(t, []string{"support-user"}, change.Fields[FieldConsumers])
}

func TestAIGatewayConsumerGroupPlannerStopsSyncAtProtectedGroup(t *testing.T) {
	api := &groupNameIdentityAPI{testAIGatewayConsumerGroupAPI: &testAIGatewayConsumerGroupAPI{
		groups: []kkComps.AIGatewayConsumerGroup{
			{ID: "before-id", Name: "before"},
			{ID: "protected-id", Name: "protected-group", Labels: map[string]string{
				labels.ProtectedKey: labels.TrueValue,
			}},
			{ID: "after-id", Name: "after"},
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
	require.Equal(t, "test-namespace", change.Namespace)
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
