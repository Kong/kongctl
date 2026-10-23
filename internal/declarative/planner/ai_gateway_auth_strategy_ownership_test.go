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

func ownedAuthStrategy(
	t *testing.T,
	name, namespace string,
) (resources.AIGatewayAuthStrategyResource, kkComps.AIGatewayAuthStrategy) {
	t.Helper()
	desired := resources.AIGatewayAuthStrategyResource{
		BaseResource: resources.BaseResource{Ref: name, Kongctl: &resources.KongctlMeta{Namespace: new(namespace)}},
		AIGateway:    "shared", Name: name, DisplayName: name, Type: "key-auth",
		Config: map[string]any{"key_names": []any{"apikey"}},
	}
	payload, err := desired.PayloadMap()
	require.NoError(t, err)
	payload[FieldID] = name + "-id"
	payload[FieldManagedBy] = resources.BuildChildManagedBy(
		nil,
		map[string]string{"other-tool": "retained"},
		namespace,
		false,
		"dev",
	)
	payload["created_at"], payload["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var observed kkComps.AIGatewayAuthStrategy
	require.NoError(t, json.Unmarshal(data, &observed))
	return desired, observed
}

func authStrategyResourceSet(strategies ...resources.AIGatewayAuthStrategyResource) *resources.ResourceSet {
	rs := federatedResourceSet()
	rs.AIGatewayAuthStrategies = strategies
	rs.SyncScope.AddChild(resources.ResourceTypeAIGateway, "shared", resources.ResourceTypeAIGatewayAuthStrategy)
	for _, strategy := range strategies {
		rs.SyncScope.AddChildNamespace(
			resources.ResourceTypeAIGatewayAuthStrategy,
			"shared",
			resources.GetNamespace(strategy.Kongctl),
		)
	}
	return rs
}

func TestFederatedAuthStrategiesScopeAndCollisions(t *testing.T) {
	a, observedA := ownedAuthStrategy(t, "auth-a", "team-a")
	b, observedB := ownedAuthStrategy(t, "auth-b", "team-b")
	_, unowned := ownedAuthStrategy(t, "legacy", "")
	api := &nameIdentityAuthStrategyAPI{children: []kkComps.AIGatewayAuthStrategy{observedA, observedB, unowned}}
	client := state.NewClient(authStrategyEmptyUsersConfig(api))
	for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
		rs := authStrategyResourceSet(a, b)
		plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: mode})
		require.NoError(t, err)
		require.Empty(t, plan.Changes)
		empty := authStrategyResourceSet()
		empty.SyncScope.AddChildNamespace(resources.ResourceTypeAIGatewayAuthStrategy, "shared", "team-a")
		// Deletion also observes retained foreign models, even if no models are declared.
		client = state.NewClient(
			authStrategyEmptyUsersConfig(api),
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
		desired, _ := ownedAuthStrategy(t, name, "team-a")
		plan, err := NewPlanner(
			client,
			slog.Default(),
		).GeneratePlan(t.Context(), authStrategyResourceSet(desired), Options{Mode: PlanModeApply})
		require.Error(t, err)
		require.Nil(t, plan)
	}
}

func TestAuthStrategyProtectionAndMetadataCapacity(t *testing.T) {
	desired, observed := ownedAuthStrategy(t, "auth-a", "team-a")
	observed.AIGatewayAuthStrategyKeyAuthResponse.Labels = map[string]string{"public": "retained"}
	desired.Kongctl.Protected = new(true)
	api := &nameIdentityAuthStrategyAPI{children: []kkComps.AIGatewayAuthStrategy{observed}}
	client := state.NewClient(authStrategyEmptyUsersConfig(api))
	plan, err := NewPlanner(
		client,
		slog.Default(),
	).GeneratePlan(t.Context(), authStrategyResourceSet(desired), Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.NotContains(t, plan.Changes[0].Fields, FieldLabels, "omitted labels are preserved at execution")
	metadata := plan.Changes[0].Fields[FieldManagedBy].(map[string]string)
	require.Equal(t, "true", metadata[resources.ChildProtectedKey])
	require.Equal(t, "retained", metadata["other-tool"])
	config := plan.Changes[0].Fields[FieldConfig].(map[string]any)
	require.Equal(t, []any{"apikey"}, config["key_names"])
	desired.ManagedBy = map[string]string{"second-tool": "overflow"}
	_, err = NewPlanner(
		client,
		slog.Default(),
	).GeneratePlan(t.Context(), authStrategyResourceSet(desired), Options{Mode: PlanModeApply})
	require.ErrorContains(t, err, "5-entry")
}

func TestAuthStrategyAliasesShareOneCompleteReconciliation(t *testing.T) {
	a, observedA := ownedAuthStrategy(t, "auth-a", "team-a")
	b, observedB := ownedAuthStrategy(t, "auth-b", "team-b")
	b.AIGateway = "alias-b"
	rs := authStrategyResourceSet(a, b)
	alias := rs.AIGateways[0]
	alias.Ref = "alias-b"
	rs.AIGateways = append(rs.AIGateways, alias)
	api := &nameIdentityAuthStrategyAPI{children: []kkComps.AIGatewayAuthStrategy{observedA, observedB}}
	client := state.NewClient(authStrategyEmptyUsersConfig(api))
	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	require.Len(t, api.lists, 1)
	a.Name = b.Name
	rs.AIGatewayAuthStrategies = []resources.AIGatewayAuthStrategyResource{a, b}
	_, err = NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.ErrorContains(t, err, "duplicate desired AI Gateway auth strategy name")
}

func authStrategyEmptyUsersConfig(api *nameIdentityAuthStrategyAPI) state.ClientConfig {
	return state.ClientConfig{
		AIGatewayAuthStrategiesAPI: api, AIGatewayModelAPI: &providerOrderModelAPI{},
		AIGatewayAgentsAPI: &testAIGatewayAgentAPI{}, AIGatewayMCPServersAPI: &testAIGatewayMCPServerAPI{},
	}
}
