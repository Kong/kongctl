package planner

import (
	"encoding/json"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func ownedPolicy(
	t *testing.T,
	name, namespace string,
) (resources.AIGatewayPolicyResource, kkComps.AIGatewayPolicy) {
	t.Helper()
	desired := resources.AIGatewayPolicyResource{
		BaseResource: resources.BaseResource{Ref: name, Kongctl: &resources.KongctlMeta{Namespace: new(namespace)}},
		AIGateway:    "shared", CreateAIGatewayPolicyRequest: kkComps.CreateAIGatewayPolicyRequest{
			Name: name, DisplayName: name, Type: "ai-sanitizer", Enabled: new(true), Global: new(false),
			Config: map[string]any{"anonymize": []any{"email"}},
		},
	}

	payload, err := desired.PayloadMap()
	require.NoError(t, err)
	payload[FieldID] = name + "-id"
	payload[FieldManagedBy] = resources.BuildChildManagedBy(
		namespace,
		false,
		"dev",
	)
	payload["created_at"], payload["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var observed kkComps.AIGatewayPolicy
	require.NoError(t, json.Unmarshal(data, &observed))
	return desired, observed
}

func policyResourceSet(policies ...resources.AIGatewayPolicyResource) *resources.ResourceSet {
	rs := federatedResourceSet()
	rs.AIGatewayPolicies = policies
	rs.SyncScope.AddChild(resources.ResourceTypeAIGateway, "shared", resources.ResourceTypeAIGatewayPolicy)
	for _, policy := range policies {
		rs.SyncScope.AddChildNamespace(
			resources.ResourceTypeAIGatewayPolicy,
			"shared",
			resources.GetNamespace(policy.Kongctl),
		)
	}
	return rs
}

func TestFederatedPoliciesScopeAndCollisions(t *testing.T) {
	a, observedA := ownedPolicy(t, "auth-a", "team-a")
	b, observedB := ownedPolicy(t, "auth-b", "team-b")
	_, unowned := ownedPolicy(t, "legacy", "")
	unowned.Labels = map[string]string{"KONGCTL-namespace": "team-a", "KONGCTL-protected": "true"}
	api := &testAIGatewayPolicyAPI{policies: []kkComps.AIGatewayPolicy{observedA, observedB, unowned}}
	client := state.NewClient(policyEmptyUsersConfig(api))
	for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
		rs := policyResourceSet(a, b)
		plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: mode})
		require.NoError(t, err)
		require.Empty(t, plan.Changes)
		empty := policyResourceSet()
		empty.SyncScope.AddChildNamespace(resources.ResourceTypeAIGatewayPolicy, "shared", "team-a")
		// Deletion also observes retained foreign models, even if no models are declared.
		client = state.NewClient(
			policyEmptyUsersConfig(api),
		)
		plan, err = NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), empty, Options{Mode: mode})
		require.NoError(t, err)
		if mode == PlanModeApply {
			require.Empty(t, plan.Changes)
		} else {
			require.Len(t, plan.Changes, 1)
			require.Equal(t, "auth-a-id", plan.Changes[0].ResourceID)
			require.Equal(t, "team-a", plan.Changes[0].Namespace)
		}
	}
	for _, name := range []string{"auth-b", "legacy"} {
		desired, _ := ownedPolicy(t, name, "team-a")
		plan, err := NewPlanner(
			client,
			slog.Default(),
		).GeneratePlan(t.Context(), policyResourceSet(desired), Options{Mode: PlanModeApply})
		require.Error(t, err)
		require.Nil(t, plan)
	}
}

func TestPolicyProtectionAndCanonicalMetadata(t *testing.T) {
	desired, observed := ownedPolicy(t, "auth-a", "team-a")
	observed.Labels = map[string]string{"public": "retained"}
	observed.Config["backend_only_credential"] = "synthetic-observed-credential"
	desired.Kongctl.Protected = new(true)
	api := &testAIGatewayPolicyAPI{policies: []kkComps.AIGatewayPolicy{observed}}
	client := state.NewClient(policyEmptyUsersConfig(api))
	plan, err := NewPlanner(
		client,
		slog.Default(),
	).GeneratePlan(t.Context(), policyResourceSet(desired), Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	encoded, err := json.Marshal(plan)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "synthetic-observed-credential")
	require.NotContains(t, plan.Changes[0].Fields, FieldLabels, "omitted labels are preserved at execution")
	metadata := plan.Changes[0].Fields[FieldManagedBy].(map[string]any)
	require.Equal(t, "true", metadata[resources.ChildProtectedKey])
	require.NotContains(t, metadata, "other-tool")
	config := plan.Changes[0].Fields[FieldConfig].(map[string]any)
	require.Equal(t, []any{"email"}, config["anonymize"])
	desired.ManagedBy = map[string]string{"second-tool": "overflow"}
	_, err = NewPlanner(
		client,
		slog.Default(),
	).GeneratePlan(t.Context(), policyResourceSet(desired), Options{Mode: PlanModeApply})
	require.NoError(t, err)
}

func TestPolicyAliasesShareOneCompleteReconciliation(t *testing.T) {
	a, observedA := ownedPolicy(t, "policy-a", "team-a")
	b, observedB := ownedPolicy(t, "policy-b", "team-b")
	b.AIGateway = "alias-b"
	rs := policyResourceSet(a, b)
	alias := rs.AIGateways[0]
	alias.Ref = "alias-b"
	rs.AIGateways = append(rs.AIGateways, alias)
	api := &testAIGatewayPolicyAPI{policies: []kkComps.AIGatewayPolicy{observedA, observedB}}
	client := state.NewClient(policyEmptyUsersConfig(api))
	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)

	a.Name = b.Name
	rs.AIGatewayPolicies = []resources.AIGatewayPolicyResource{a, b}
	_, err = NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.ErrorContains(t, err,
		`duplicate desired AI Gateway policy name "policy-b" on gateway "shared" `+
			`(refs "policy-a" and "policy-b")`)
}

func policyEmptyUsersConfig(api *testAIGatewayPolicyAPI) state.ClientConfig {
	return state.ClientConfig{
		AIGatewayPoliciesAPI: api, AIGatewayConsumersAPI: &testAIGatewayConsumerAPI{},
		AIGatewayConsumerGroupsAPI: &testAIGatewayConsumerGroupAPI{}, AIGatewayModelAPI: &providerOrderModelAPI{},
		AIGatewayAgentsAPI: &testAIGatewayAgentAPI{}, AIGatewayMCPServersAPI: &testAIGatewayMCPServerAPI{},
	}
}

func TestPolicyExplicitEmptyLabelsSurviveSavedPlan(t *testing.T) {
	desired, observed := ownedPolicy(t, "shared-policy", "team-a")
	desired.Labels = map[string]string{}
	observed.Labels = map[string]string{"public": "remove"}
	api := &testAIGatewayPolicyAPI{policies: []kkComps.AIGatewayPolicy{observed}}
	plan, err := NewPlanner(state.NewClient(policyEmptyUsersConfig(api)), slog.Default()).GeneratePlan(
		t.Context(), policyResourceSet(desired), Options{Mode: PlanModeApply},
	)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	encoded, err := json.Marshal(plan)
	require.NoError(t, err)
	var saved Plan
	require.NoError(t, json.Unmarshal(encoded, &saved))
	require.Contains(t, saved.Changes[0].Fields, FieldLabels)
	require.Empty(t, saved.Changes[0].Fields[FieldLabels])
	require.Contains(t, saved.Changes[0].ChangedFields, FieldLabels)
}
