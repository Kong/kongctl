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
	stores []kkComps.AIGatewayConfigStore
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
