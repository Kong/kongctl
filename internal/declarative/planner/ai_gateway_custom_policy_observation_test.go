package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

type countingCustomPolicyUsersAPI struct {
	testAIGatewayPolicyAPI
	reads int
}

func (a *countingCustomPolicyUsersAPI) ListAiGatewayPolicies(
	ctx context.Context, request kkOps.ListAiGatewayPoliciesRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewayPoliciesResponse, error) {
	a.reads++
	return a.testAIGatewayPolicyAPI.ListAiGatewayPolicies(ctx, request, opts...)
}

func TestCustomPolicyDeletionReusesPolicyObservation(t *testing.T) {
	policy := testAIGatewayPolicy()
	policy.Type = "plugin"
	api := &countingCustomPolicyUsersAPI{testAIGatewayPolicyAPI: testAIGatewayPolicyAPI{
		policies: []kkComps.AIGatewayPolicy{policy},
	}}
	p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayPoliciesAPI: api}), slog.Default())
	p.resourceCache = newPlanningResourceCache()
	_, err := p.listAIGatewayPolicies(t.Context(), "gateway-id")
	require.NoError(t, err)
	plan := &Plan{Changes: []PlannedChange{
		{
			ID: "definition", ResourceType: ResourceTypeAIGatewayCustomPolicy, Action: ActionDelete,
			Namespace: "default", Parent: &ParentInfo{Ref: "gateway"}, Fields: map[string]any{FieldName: "plugin"},
		},
		{
			ID: "instance", ResourceType: ResourceTypeAIGatewayPolicy, ResourceID: policy.ID, Action: ActionDelete,
			Namespace: "default", Parent: &ParentInfo{Ref: "gateway"},
		},
	}}
	for range 2 {
		require.NoError(
			t,
			p.resolveAIGatewayCustomPolicyDependencies(t.Context(), "default", "gateway", "gateway-id", plan),
		)
	}
	require.Equal(t, 1, api.reads)
	require.Equal(t, []string{"instance"}, plan.Changes[0].DependsOn)
	// Cache keys must not conflate separate gateways, even with identical names.
	_, err = p.listAIGatewayPolicies(t.Context(), "other-gateway-id")
	require.NoError(t, err)
	require.Equal(t, 2, api.reads)
}
