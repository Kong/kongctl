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

func ownedProvider(
	t *testing.T,
	name, namespace string,
) (resources.AIGatewayProviderResource, kkComps.AIGatewayModelProvider) {
	t.Helper()
	desired := resources.AIGatewayProviderResource{
		BaseResource: resources.BaseResource{Ref: name, Kongctl: &resources.KongctlMeta{Namespace: new(namespace)}},
		AIGateway:    "shared", Name: name, DisplayName: name, Type: "openai",
		Config: map[string]any{
			"auth": map[string]any{"type": "basic", "headers": []any{map[string]any{"name": "Authorization"}}},
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
	var observed kkComps.AIGatewayModelProvider
	require.NoError(t, json.Unmarshal(data, &observed))
	return desired, observed
}

func providerResourceSet(providers ...resources.AIGatewayProviderResource) *resources.ResourceSet {
	rs := federatedResourceSet()
	rs.AIGatewayProviders = providers
	rs.SyncScope.AddChild(resources.ResourceTypeAIGateway, "shared", resources.ResourceTypeAIGatewayProvider)
	for _, provider := range providers {
		rs.SyncScope.AddChildNamespace(
			resources.ResourceTypeAIGatewayProvider,
			"shared",
			resources.GetNamespace(provider.Kongctl),
		)
	}
	return rs
}

func TestFederatedProvidersScopeAndCollisions(t *testing.T) {
	a, observedA := ownedProvider(t, "provider-a", "team-a")
	b, observedB := ownedProvider(t, "provider-b", "team-b")
	_, unowned := ownedProvider(t, "legacy", "")
	api := &nameIdentityProviderAPI{children: []kkComps.AIGatewayModelProvider{observedA, observedB, unowned}}
	client := state.NewClient(state.ClientConfig{AIGatewayProvidersAPI: api})
	for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
		rs := providerResourceSet(a, b)
		plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: mode})
		require.NoError(t, err)
		require.Empty(t, plan.Changes)
		empty := providerResourceSet()
		empty.SyncScope.AddChildNamespace(resources.ResourceTypeAIGatewayProvider, "shared", "team-a")
		// Deletion also observes retained foreign models, even if no models are declared.
		client = state.NewClient(
			state.ClientConfig{AIGatewayProvidersAPI: api, AIGatewayModelAPI: &providerOrderModelAPI{}},
		)
		plan, err = NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), empty, Options{Mode: mode})
		require.NoError(t, err)
		if mode == PlanModeApply {
			require.Empty(t, plan.Changes)
		} else {
			require.Len(t, plan.Changes, 1)
			require.Equal(t, "provider-a-id", plan.Changes[0].ResourceID)
			require.Equal(t, "team-a", plan.Changes[0].Namespace)
		}
	}
	for _, name := range []string{"provider-b", "legacy"} {
		desired, _ := ownedProvider(t, name, "team-a")
		plan, err := NewPlanner(
			client,
			slog.Default(),
		).GeneratePlan(t.Context(), providerResourceSet(desired), Options{Mode: PlanModeApply})
		require.Error(t, err)
		require.Nil(t, plan)
	}
}

func TestProviderProtectionAndCanonicalMetadata(t *testing.T) {
	desired, observed := ownedProvider(t, "provider-a", "team-a")
	observed.AIGatewayModelProviderAIGatewayModelProviderOpenai.Labels = map[string]string{"public": "retained"}
	desired.Kongctl.Protected = new(true)
	api := &nameIdentityProviderAPI{children: []kkComps.AIGatewayModelProvider{observed}}
	client := state.NewClient(state.ClientConfig{AIGatewayProvidersAPI: api})
	plan, err := NewPlanner(
		client,
		slog.Default(),
	).GeneratePlan(t.Context(), providerResourceSet(desired), Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.NotContains(t, plan.Changes[0].Fields, FieldLabels, "omitted labels are preserved at execution")
	metadata := plan.Changes[0].Fields[FieldManagedBy].(map[string]string)
	require.Equal(t, "true", metadata[resources.ChildProtectedKey])
	require.NotContains(t, metadata, "other-tool")
	config := plan.Changes[0].Fields[FieldConfig].(map[string]any)
	require.Equal(t, "Authorization", config["auth"].(map[string]any)["headers"].([]any)[0].(map[string]any)["name"])
	desired.ManagedBy = map[string]string{"second-tool": "overflow"}
	_, err = NewPlanner(
		client,
		slog.Default(),
	).GeneratePlan(t.Context(), providerResourceSet(desired), Options{Mode: PlanModeApply})
	require.NoError(t, err)
}

func TestProviderAliasesShareOneCompleteReconciliation(t *testing.T) {
	a, observedA := ownedProvider(t, "provider-a", "team-a")
	b, observedB := ownedProvider(t, "provider-b", "team-b")
	b.AIGateway = "alias-b"
	rs := providerResourceSet(a, b)
	alias := rs.AIGateways[0]
	alias.Ref = "alias-b"
	rs.AIGateways = append(rs.AIGateways, alias)
	api := &nameIdentityProviderAPI{children: []kkComps.AIGatewayModelProvider{observedA, observedB}}
	client := state.NewClient(state.ClientConfig{AIGatewayProvidersAPI: api, AIGatewayModelAPI: &providerOrderModelAPI{}})
	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	require.Len(t, api.lists, 1)
	a.Name = b.Name
	rs.AIGatewayProviders = []resources.AIGatewayProviderResource{a, b}
	_, err = NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.ErrorContains(t, err, "duplicate desired AI Gateway provider name")
}
