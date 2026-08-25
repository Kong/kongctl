package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConfigStorePlannerCreatesAndUpdatesSparseFields(t *testing.T) {
	displayName := "Support-Store"
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{gateways: []kkComps.AIGateway{testAIGateway()}},
		AIGatewayConfigStoresAPI: &testAIGatewayConfigStoreAPI{
			stores: []kkComps.AIGatewayConfigStore{{
				ID:   "existing-id",
				Name: "existing-store",
			}},
		},
	})
	rs := testAIGatewayConfigStoreResourceSet(
		resources.AIGatewayConfigStoreResource{
			BaseResource: resources.BaseResource{Ref: "new-store"},
			AIGateway:    "support-gateway",
			Name:         "new-store",
		},
		resources.AIGatewayConfigStoreResource{
			BaseResource: resources.BaseResource{Ref: "existing-store"},
			AIGateway:    "support-gateway",
			Name:         "existing-store",
			DisplayName:  &displayName,
		},
	)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 2)
	require.Equal(t, ActionCreate, plan.Changes[0].Action)
	require.Equal(t, map[string]any{FieldName: "new-store"}, plan.Changes[0].Fields)
	require.Equal(t, ActionUpdate, plan.Changes[1].Action)
	require.Equal(t, map[string]any{FieldDisplayName: displayName}, plan.Changes[1].Fields)
	require.Equal(t, "existing-id", rs.GetAIGatewayConfigStoreByRef("existing-store").GetKonnectID())
}

func TestAIGatewayConfigStorePlannerRejectsIDMatchedRename(t *testing.T) {
	const storeID = "11111111-1111-1111-1111-111111111111"
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{gateways: []kkComps.AIGateway{testAIGateway()}},
		AIGatewayConfigStoresAPI: &testAIGatewayConfigStoreAPI{
			stores: []kkComps.AIGatewayConfigStore{{ID: storeID, Name: "old-name"}},
		},
	})
	rs := testAIGatewayConfigStoreResourceSet(resources.AIGatewayConfigStoreResource{
		BaseResource: resources.BaseResource{Ref: storeID},
		AIGateway:    "support-gateway",
		Name:         "new-name",
	})

	_, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.Error(t, err)
	require.Contains(t, err.Error(), "immutable name")
	require.Contains(t, err.Error(), "delete and recreate")
}

func TestAIGatewayConfigStorePlannerSyncDeletesScopedStores(t *testing.T) {
	scope := resources.NewSyncScope()
	scope.AddRoot(resources.ResourceTypeAIGateway)
	scope.AddChild(
		resources.ResourceTypeAIGateway,
		"support-gateway",
		resources.ResourceTypeAIGatewayConfigStore,
	)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{gateways: []kkComps.AIGateway{testAIGateway()}},
		AIGatewayConfigStoresAPI: &testAIGatewayConfigStoreAPI{
			stores: []kkComps.AIGatewayConfigStore{{ID: "store-id", Name: "stale-store"}},
		},
	})
	rs := testAIGatewayConfigStoreResourceSet()
	rs.SyncScope = scope

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ActionDelete, plan.Changes[0].Action)
	require.Equal(t, ResourceTypeAIGatewayConfigStore, plan.Changes[0].ResourceType)
	require.Equal(t, "store-id", plan.Changes[0].ResourceID)
}

func TestAIGatewayConfigStorePlannerOrdersVaultReferenceAfterStoreCreate(t *testing.T) {
	vaultRequest := testAIGatewayConfigStoreVaultRequest(t)

	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI:             &testAIGatewayAPI{gateways: []kkComps.AIGateway{testAIGateway()}},
		AIGatewayConfigStoresAPI: &testAIGatewayConfigStoreAPI{},
		AIGatewayVaultsAPI:       &testAIGatewayVaultAPI{},
	})
	rs := testAIGatewayConfigStoreResourceSet(resources.AIGatewayConfigStoreResource{
		BaseResource: resources.BaseResource{Ref: "support-store"},
		AIGateway:    "support-gateway",
		Name:         "support-store",
	})
	rs.AIGatewayVaults = []resources.AIGatewayVaultResource{{
		BaseResource:                resources.BaseResource{Ref: "support-vault"},
		AIGateway:                   "support-gateway",
		CreateAIGatewayVaultRequest: vaultRequest,
	}}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 2)

	var storeChange, vaultChange *PlannedChange
	for i := range plan.Changes {
		switch plan.Changes[i].ResourceType {
		case ResourceTypeAIGatewayConfigStore:
			storeChange = &plan.Changes[i]
		case ResourceTypeAIGatewayVault:
			vaultChange = &plan.Changes[i]
		}
	}
	require.NotNil(t, storeChange)
	require.NotNil(t, vaultChange)
	reference, ok := vaultChange.References[FieldConfig+"."+FieldConfigStoreID]
	require.True(t, ok)
	require.Equal(t, "__REF__:support-store#id", reference.Ref)
	require.Equal(t, resources.UnknownReferenceID, reference.ID)
	require.Contains(t, vaultChange.DependsOn, storeChange.ID)
}

func TestAIGatewayConfigStorePlannerResolvesExistingStoreForVault(t *testing.T) {
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{gateways: []kkComps.AIGateway{testAIGateway()}},
		AIGatewayConfigStoresAPI: &testAIGatewayConfigStoreAPI{
			stores: []kkComps.AIGatewayConfigStore{{ID: "existing-store-id", Name: "support-store"}},
		},
		AIGatewayVaultsAPI: &testAIGatewayVaultAPI{},
	})
	rs := testAIGatewayConfigStoreResourceSet(resources.AIGatewayConfigStoreResource{
		BaseResource: resources.BaseResource{Ref: "support-store"},
		AIGateway:    "support-gateway",
		Name:         "support-store",
	})
	rs.AIGatewayVaults = []resources.AIGatewayVaultResource{{
		BaseResource:                resources.BaseResource{Ref: "support-vault"},
		AIGateway:                   "support-gateway",
		CreateAIGatewayVaultRequest: testAIGatewayConfigStoreVaultRequest(t),
	}}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ResourceTypeAIGatewayVault, plan.Changes[0].ResourceType)
	reference := plan.Changes[0].References[FieldConfig+"."+FieldConfigStoreID]
	require.Equal(t, "existing-store-id", reference.ID)
	require.Empty(t, plan.Changes[0].DependsOn)
}

func TestAIGatewayConfigStoreSecretPlannerCreateNoOpAndRotation(t *testing.T) {
	t.Run("create includes deferred write", func(t *testing.T) {
		client := state.NewClient(state.ClientConfig{
			AIGatewayAPI: &testAIGatewayAPI{gateways: []kkComps.AIGateway{testAIGateway()}},
			AIGatewayConfigStoresAPI: &testAIGatewayConfigStoreAPI{
				stores: []kkComps.AIGatewayConfigStore{{ID: "store-id", Name: "support-store"}},
			},
		})
		rs := testAIGatewayConfigStoreResourceSet(resources.AIGatewayConfigStoreResource{
			BaseResource: resources.BaseResource{Ref: "support-store"},
			AIGateway:    "support-gateway",
			Name:         "support-store",
		})
		rs.AIGatewayConfigStoreSecrets = []resources.AIGatewayConfigStoreSecretResource{{
			BaseResource:         resources.BaseResource{Ref: "support-openai-header"},
			AIGatewayConfigStore: "support-store",
			Key:                  "openai-auth-header",
			Value:                testSecretPlaceholder(t),
		}}
		addTestConfigStoreSecretSource(rs)

		plan, err := NewPlanner(client, slog.Default()).GeneratePlan(
			t.Context(), rs, Options{Mode: PlanModeApply},
		)
		require.NoError(t, err)
		require.Len(t, plan.Changes, 1)
		change := plan.Changes[0]
		require.Equal(t, ResourceTypeAIGatewayConfigStoreSecret, change.ResourceType)
		require.Equal(t, ActionCreate, change.Action)
		require.Equal(t, "openai-auth-header", change.Fields[FieldKey])
		require.Len(t, change.SecretWrites, 1)
		require.NotContains(t, fmt.Sprint(change.Fields), "configured-secret-value")
	})

	t.Run("existing is no-op without selector", func(t *testing.T) {
		client, rs := testExistingConfigStoreSecret(t)
		plan, err := NewPlanner(client, slog.Default()).GeneratePlan(
			t.Context(), rs, Options{Mode: PlanModeApply},
		)
		require.NoError(t, err)
		require.Empty(t, plan.Changes)
	})

	t.Run("existing declaration without value is no-op", func(t *testing.T) {
		client, rs := testExistingConfigStoreSecret(t)
		rs.AIGatewayConfigStoreSecrets[0].Value = ""
		delete(rs.SecretSources, "support-openai-header")

		plan, err := NewPlanner(client, slog.Default()).GeneratePlan(
			t.Context(), rs, Options{Mode: PlanModeApply},
		)
		require.NoError(t, err)
		require.Empty(t, plan.Changes)
	})

	t.Run("selector plans secret-only update", func(t *testing.T) {
		client, rs := testExistingConfigStoreSecret(t)
		plan, err := NewPlanner(client, slog.Default()).GeneratePlan(
			t.Context(),
			rs,
			Options{Mode: PlanModeApply, WriteSecretSelectors: []string{"support-openai-header#value"}},
		)
		require.NoError(t, err)
		require.Len(t, plan.Changes, 1)
		change := plan.Changes[0]
		require.Equal(t, ActionUpdate, change.Action)
		require.Equal(t, "openai-auth-header", change.ResourceID)
		require.NotContains(t, change.Fields, FieldKey)
		require.Len(t, change.SecretWrites, 1)
		require.Equal(t, "store-id", change.Parent.ID)
		require.Equal(t, "gateway-id", change.References[FieldAIGatewayID].ID)
	})

	t.Run("write-secrets plans secret-only update", func(t *testing.T) {
		client, rs := testExistingConfigStoreSecret(t)
		plan, err := NewPlanner(client, slog.Default()).GeneratePlan(
			t.Context(),
			rs,
			Options{Mode: PlanModeApply, WriteSecrets: true},
		)
		require.NoError(t, err)
		require.Len(t, plan.Changes, 1)
		require.Equal(t, ActionUpdate, plan.Changes[0].Action)
		require.Len(t, plan.Changes[0].SecretWrites, 1)
	})
}

func TestAIGatewayConfigStoreSecretPlannerRequiresValueForMissingSecret(t *testing.T) {
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{gateways: []kkComps.AIGateway{testAIGateway()}},
		AIGatewayConfigStoresAPI: &testAIGatewayConfigStoreAPI{
			stores: []kkComps.AIGatewayConfigStore{{ID: "store-id", Name: "support-store"}},
		},
	})
	rs := testAIGatewayConfigStoreResourceSet(resources.AIGatewayConfigStoreResource{
		BaseResource: resources.BaseResource{Ref: "support-store"},
		AIGateway:    "support-gateway",
		Name:         "support-store",
	})
	rs.AIGatewayConfigStoreSecrets = []resources.AIGatewayConfigStoreSecretResource{{
		BaseResource:         resources.BaseResource{Ref: "support-openai-header"},
		AIGatewayConfigStore: "support-store",
		Key:                  "openai-auth-header",
	}}

	_, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.ErrorContains(t, err, "requires value: !secret")
}

func TestAIGatewayConfigStoreSecretPlannerOrdersSamePlanParentCreates(t *testing.T) {
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI:             &testAIGatewayAPI{},
		AIGatewayConfigStoresAPI: &testAIGatewayConfigStoreAPI{},
	})
	rs := testAIGatewayConfigStoreResourceSet(resources.AIGatewayConfigStoreResource{
		BaseResource: resources.BaseResource{Ref: "support-store"},
		AIGateway:    "support-gateway",
		Name:         "support-store",
	})
	rs.AIGatewayConfigStoreSecrets = []resources.AIGatewayConfigStoreSecretResource{{
		BaseResource:         resources.BaseResource{Ref: "support-openai-header"},
		AIGatewayConfigStore: "support-store",
		Key:                  "openai-auth-header",
		Value:                testSecretPlaceholder(t),
	}}
	addTestConfigStoreSecretSource(rs)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 3)

	changes := make(map[string]*PlannedChange, len(plan.Changes))
	for i := range plan.Changes {
		changes[plan.Changes[i].ResourceType] = &plan.Changes[i]
	}
	gatewayChange := changes[ResourceTypeAIGateway]
	storeChange := changes[ResourceTypeAIGatewayConfigStore]
	secretChange := changes[ResourceTypeAIGatewayConfigStoreSecret]
	require.NotNil(t, gatewayChange)
	require.NotNil(t, storeChange)
	require.NotNil(t, secretChange)
	require.Contains(t, storeChange.DependsOn, gatewayChange.ID)
	require.Contains(t, secretChange.DependsOn, storeChange.ID)
	require.Nil(t, secretChange.Parent)
	require.Equal(t, "support-store", secretChange.References[FieldConfigStoreID].Ref)
	require.Equal(t, "support-gateway", secretChange.References[FieldAIGatewayID].Ref)
	require.Empty(t, secretChange.References[FieldAIGatewayID].ID)
}

func TestAIGatewayConfigStoreSecretPlannerSyncDeletesExplicitEmptyScope(t *testing.T) {
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{gateways: []kkComps.AIGateway{testAIGateway()}},
		AIGatewayConfigStoresAPI: &testAIGatewayConfigStoreAPI{
			stores:  []kkComps.AIGatewayConfigStore{{ID: "store-id", Name: "support-store"}},
			secrets: []kkComps.AIGatewayConfigStoreSecret{{Key: "stale-key"}},
		},
	})
	rs := testAIGatewayConfigStoreResourceSet(resources.AIGatewayConfigStoreResource{
		BaseResource: resources.BaseResource{Ref: "support-store"},
		AIGateway:    "support-gateway",
		Name:         "support-store",
	})
	rs.SyncScope = resources.NewSyncScope()
	rs.SyncScope.AddRoot(resources.ResourceTypeAIGateway)
	rs.SyncScope.AddChild(
		resources.ResourceTypeAIGateway,
		"support-gateway",
		resources.ResourceTypeAIGatewayConfigStore,
	)
	rs.SyncScope.AddChild(
		resources.ResourceTypeAIGatewayConfigStore,
		"support-store",
		resources.ResourceTypeAIGatewayConfigStoreSecret,
	)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ResourceTypeAIGatewayConfigStoreSecret, plan.Changes[0].ResourceType)
	require.Equal(t, ActionDelete, plan.Changes[0].Action)
}

func testExistingConfigStoreSecret(
	t *testing.T,
) (*state.Client, *resources.ResourceSet) {
	t.Helper()
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{gateways: []kkComps.AIGateway{testAIGateway()}},
		AIGatewayConfigStoresAPI: &testAIGatewayConfigStoreAPI{
			stores:  []kkComps.AIGatewayConfigStore{{ID: "store-id", Name: "support-store"}},
			secrets: []kkComps.AIGatewayConfigStoreSecret{{Key: "openai-auth-header"}},
		},
	})
	rs := testAIGatewayConfigStoreResourceSet(resources.AIGatewayConfigStoreResource{
		BaseResource: resources.BaseResource{Ref: "support-store"},
		AIGateway:    "support-gateway",
		Name:         "support-store",
	})
	rs.AIGatewayConfigStoreSecrets = []resources.AIGatewayConfigStoreSecretResource{{
		BaseResource:         resources.BaseResource{Ref: "support-openai-header"},
		AIGatewayConfigStore: "support-store",
		Key:                  "openai-auth-header",
		Value:                testSecretPlaceholder(t),
	}}
	addTestConfigStoreSecretSource(rs)
	return client, rs
}

func addTestConfigStoreSecretSource(rs *resources.ResourceSet) {
	rs.AddSecretSource("support-openai-header", "/value", tags.SecretExpression{Parts: []tags.SecretPart{{
		Source: &tags.SecretSource{Kind: "env", Reference: "OPENAI_AUTH_HEADER"},
	}}}, false)
}

func testSecretPlaceholder(t *testing.T) string {
	t.Helper()
	value, err := tags.BuildSecretPlaceholder(tags.SecretExpression{Parts: []tags.SecretPart{{
		Source: &tags.SecretSource{Kind: "env", Reference: "OPENAI_AUTH_HEADER"},
	}}})
	require.NoError(t, err)
	return value
}

func testAIGatewayConfigStoreResourceSet(
	stores ...resources.AIGatewayConfigStoreResource,
) *resources.ResourceSet {
	return &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{
				Ref:     "support-gateway",
				Kongctl: &resources.KongctlMeta{Namespace: new("default")},
			},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "support-gateway",
				DisplayName: "Support Gateway",
			},
		}},
		AIGatewayConfigStores: stores,
	}
}

func testAIGatewayConfigStoreVaultRequest(t *testing.T) kkComps.CreateAIGatewayVaultRequest {
	t.Helper()
	var request kkComps.CreateAIGatewayVaultRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"type": "konnect",
		"name": "support-vault",
		"config": {"config_store_id": "__REF__:support-store#id"}
	}`), &request))
	return request
}

type testAIGatewayConfigStoreAPI struct {
	stores  []kkComps.AIGatewayConfigStore
	secrets []kkComps.AIGatewayConfigStoreSecret
}

func (t *testAIGatewayConfigStoreAPI) ListAiGatewayConfigStores(
	context.Context,
	kkOps.ListAiGatewayConfigStoresRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayConfigStoresResponse, error) {
	return &kkOps.ListAiGatewayConfigStoresResponse{
		ListAIGatewayConfigStoresResponse: &kkComps.ListAIGatewayConfigStoresResponse{Data: t.stores},
	}, nil
}

func (t *testAIGatewayConfigStoreAPI) CreateAiGatewayConfigStore(
	context.Context,
	string,
	kkComps.CreateAIGatewayConfigStoreRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayConfigStoreResponse, error) {
	return nil, nil
}

func (t *testAIGatewayConfigStoreAPI) GetAiGatewayConfigStore(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetAiGatewayConfigStoreResponse, error) {
	return nil, nil
}

func (t *testAIGatewayConfigStoreAPI) UpdateAiGatewayConfigStore(
	context.Context,
	kkOps.UpdateAiGatewayConfigStoreRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayConfigStoreResponse, error) {
	return nil, nil
}

func (t *testAIGatewayConfigStoreAPI) DeleteAiGatewayConfigStore(
	context.Context,
	kkOps.DeleteAiGatewayConfigStoreRequest,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayConfigStoreResponse, error) {
	return nil, nil
}

func (t *testAIGatewayConfigStoreAPI) ListAiGatewayConfigStoreSecrets(
	_ context.Context,
	_ kkOps.ListAiGatewayConfigStoreSecretsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAiGatewayConfigStoreSecretsResponse, error) {
	return &kkOps.ListAiGatewayConfigStoreSecretsResponse{
		ListAIGatewayConfigStoreSecretsResponse: &kkComps.ListAIGatewayConfigStoreSecretsResponse{Data: t.secrets},
	}, nil
}

func (t *testAIGatewayConfigStoreAPI) CreateAiGatewayConfigStoreSecret(
	context.Context,
	kkOps.CreateAiGatewayConfigStoreSecretRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayConfigStoreSecretResponse, error) {
	return nil, nil
}

func (t *testAIGatewayConfigStoreAPI) GetAiGatewayConfigStoreSecret(
	context.Context,
	kkOps.GetAiGatewayConfigStoreSecretRequest,
	...kkOps.Option,
) (*kkOps.GetAiGatewayConfigStoreSecretResponse, error) {
	return nil, nil
}

func (t *testAIGatewayConfigStoreAPI) UpdateAiGatewayConfigStoreSecret(
	context.Context,
	kkOps.UpdateAiGatewayConfigStoreSecretRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayConfigStoreSecretResponse, error) {
	return nil, nil
}

func (t *testAIGatewayConfigStoreAPI) DeleteAiGatewayConfigStoreSecret(
	context.Context,
	kkOps.DeleteAiGatewayConfigStoreSecretRequest,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayConfigStoreSecretResponse, error) {
	return nil, nil
}
