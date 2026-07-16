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

func TestAIGatewayVaultPlannerCreatesChildForExistingGateway(t *testing.T) {
	vault := testAIGatewayVaultResource(t)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayVaultsAPI: &testAIGatewayVaultAPI{},
	})
	rs := &resources.ResourceSet{
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
		AIGatewayVaults: []resources.AIGatewayVaultResource{vault},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayVault, change.ResourceType)
	require.Equal(t, "support-env", change.ResourceRef)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
	require.Equal(t, "support-gateway", change.Parent.Ref)
	require.Equal(t, "env", change.Fields[FieldType])
}

func TestAIGatewayVaultPlannerUpdatesChangedVault(t *testing.T) {
	vault := testAIGatewayVaultResource(t)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayVaultsAPI: &testAIGatewayVaultAPI{
			vaults: []kkComps.AIGatewayVault{testAIGatewayVault(t, "vault-id", "support-env", "OLD_")},
		},
	})
	rs := &resources.ResourceSet{
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
		AIGatewayVaults: []resources.AIGatewayVaultResource{vault},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionUpdate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayVault, change.ResourceType)
	require.Equal(t, "vault-id", change.ResourceID)
	require.Contains(t, change.ChangedFields, FieldConfig)
}

func TestAIGatewayVaultPlannerIgnoresWriteOnlySecretDrift(t *testing.T) {
	currentPayload := map[string]any{
		FieldType: "hcv",
		FieldName: "support-hcv",
		FieldConfig: map[string]any{
			"auth_method": "token",
			"host":        "vault.example.test",
			"port":        float64(8200),
		},
	}
	desiredPayload := map[string]any{
		FieldType: "hcv",
		FieldName: "support-hcv",
		FieldConfig: map[string]any{
			"auth_method": "token",
			"host":        "vault.example.test",
			"port":        float64(8200),
			"token":       "super-secret-token",
		},
	}

	currentCompare, desiredCompare := normalizeAIGatewayPayloadsForComparison(currentPayload, desiredPayload)
	currentCompare = scrubAIGatewayVaultWriteOnlyFields(currentCompare).(map[string]any)
	desiredCompare = scrubAIGatewayVaultWriteOnlyFields(desiredCompare).(map[string]any)
	currentPlanPayload := scrubAIGatewayVaultWriteOnlyFields(currentPayload).(map[string]any)
	desiredPlanPayload := scrubAIGatewayVaultWriteOnlyFields(desiredPayload).(map[string]any)

	changedFields := diffAIGatewayPayloads(currentPlanPayload, desiredPlanPayload, currentCompare, desiredCompare)
	require.Empty(t, changedFields)
	require.NotContains(t, desiredPlanPayload[FieldConfig].(map[string]any), "token")
}

func TestAIGatewayVaultPlannerSendsWriteOnlySecretsOnObservableUpdate(t *testing.T) {
	var currentVault kkComps.AIGatewayVault
	require.NoError(t, json.Unmarshal([]byte(`{
		"id": "vault-id",
		"type": "hcv",
		"name": "support-hcv",
		"description": "Support Hashicorp Vault",
		"config": {"auth_method": "token", "host": "old-vault.example.test", "port": 8200},
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-01T00:00:00Z"
	}`), &currentVault))
	current := state.AIGatewayVault{AIGatewayVault: currentVault}
	var desired resources.AIGatewayVaultResource
	require.NoError(t, json.Unmarshal([]byte(`{
		"ref": "support-hcv",
		"ai_gateway": "support-gateway",
		"type": "hcv",
		"name": "support-hcv",
		"description": "Support Hashicorp Vault",
		"config": {
			"auth_method": "token",
			"host": "vault.example.test",
			"port": 8200,
			"token": "super-secret-token"
		}
	}`), &desired))

	needsUpdate, fields, changedFields, err := shouldUpdateAIGatewayVault(current, desired)
	require.NoError(t, err)
	require.True(t, needsUpdate)
	require.Equal(t, "super-secret-token", fields[FieldConfig].(map[string]any)["token"])

	configChange := changedFields[FieldConfig]
	require.NotContains(t, configChange.New.(map[string]any), "token")
}

func TestAIGatewayVaultPlannerSyncDeletesScopedVaults(t *testing.T) {
	scope := resources.NewSyncScope()
	scope.AddRoot(resources.ResourceTypeAIGateway)
	scope.AddChild(resources.ResourceTypeAIGateway, "support-gateway", resources.ResourceTypeAIGatewayVault)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayVaultsAPI: &testAIGatewayVaultAPI{
			vaults: []kkComps.AIGatewayVault{testAIGatewayVault(t, "vault-id", "support-env", "SUPPORT_")},
		},
	})
	rs := &resources.ResourceSet{
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
		SyncScope: scope,
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionDelete, change.Action)
	require.Equal(t, ResourceTypeAIGatewayVault, change.ResourceType)
	require.Equal(t, "vault-id", change.ResourceID)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
}

func testAIGatewayVaultResource(t *testing.T) resources.AIGatewayVaultResource {
	t.Helper()
	payload := `{
		"ref": "support-env",
		"ai_gateway": "support-gateway",
		"type": "env",
		"name": "support-env",
		"description": "Support environment variables",
		"config": {"prefix": "SUPPORT_", "base64_decode": false}
	}`
	var vault resources.AIGatewayVaultResource
	require.NoError(t, json.Unmarshal([]byte(payload), &vault))
	return vault
}

func testAIGatewayVault(t *testing.T, id string, name string, prefix string) kkComps.AIGatewayVault {
	t.Helper()
	payload := `{
		"id": "` + id + `",
		"type": "env",
		"name": "` + name + `",
		"description": "Support environment variables",
		"config": {"prefix": "` + prefix + `", "base64_decode": false},
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-01T00:00:00Z"
	}`
	var vault kkComps.AIGatewayVault
	require.NoError(t, json.Unmarshal([]byte(payload), &vault))
	return vault
}

type testAIGatewayVaultAPI struct {
	vaults []kkComps.AIGatewayVault
}

func (t *testAIGatewayVaultAPI) ListAiGatewayVaults(
	context.Context,
	kkOps.ListAiGatewayVaultsRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayVaultsResponse, error) {
	return &kkOps.ListAiGatewayVaultsResponse{
		ListAIGatewayVaultsResponse: &kkComps.ListAIGatewayVaultsResponse{
			Data: t.vaults,
		},
	}, nil
}

func (t *testAIGatewayVaultAPI) CreateAiGatewayVault(
	context.Context,
	string,
	kkComps.CreateAIGatewayVaultRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayVaultResponse, error) {
	return nil, nil
}

func (t *testAIGatewayVaultAPI) GetAiGatewayVault(
	_ context.Context,
	_ string,
	vaultID string,
	_ ...kkOps.Option,
) (*kkOps.GetAiGatewayVaultResponse, error) {
	for _, vault := range t.vaults {
		if resources.AIGatewayVaultID(vault) == vaultID || resources.AIGatewayVaultName(vault) == vaultID {
			return &kkOps.GetAiGatewayVaultResponse{AIGatewayVault: &vault}, nil
		}
	}
	return &kkOps.GetAiGatewayVaultResponse{}, nil
}

func (t *testAIGatewayVaultAPI) UpdateAiGatewayVault(
	context.Context,
	kkOps.UpdateAiGatewayVaultRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayVaultResponse, error) {
	return nil, nil
}

func (t *testAIGatewayVaultAPI) DeleteAiGatewayVault(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayVaultResponse, error) {
	return nil, nil
}
