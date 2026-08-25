package executor

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConfigStoreSecretAdapterMapsSDKRequests(t *testing.T) {
	adapter := NewAIGatewayConfigStoreSecretAdapter(nil)

	var create kkComps.CreateAIGatewayConfigStoreSecretRequest
	require.NoError(t, adapter.MapCreateFields(t.Context(), nil, map[string]any{
		planner.FieldKey:   "openai-auth-header",
		planner.FieldValue: "secret-value",
	}, &create))
	require.Equal(t, "openai-auth-header", create.Key)
	require.Equal(t, "secret-value", create.Value)

	var update kkComps.UpdateAIGatewayConfigStoreSecretRequest
	require.NoError(t, adapter.MapUpdateFields(t.Context(), nil, map[string]any{
		planner.FieldValue: "rotated-secret-value",
	}, &update, nil))
	require.Equal(t, "rotated-secret-value", update.Value)
}

func TestAIGatewayConfigStoreSecretParentIDs(t *testing.T) {
	change := &planner.PlannedChange{
		Parent: &planner.ParentInfo{ID: "store-id"},
		References: map[string]planner.ReferenceInfo{
			planner.FieldAIGatewayID: {ID: "gateway-id"},
		},
	}

	gatewayID, storeID, err := aiGatewayConfigStoreSecretParentIDs(&ExecutionContext{PlannedChange: change})
	require.NoError(t, err)
	require.Equal(t, "gateway-id", gatewayID)
	require.Equal(t, "store-id", storeID)
}

func TestHydrateKnownReferenceIDsResolvesConfigStoreSecretParents(t *testing.T) {
	exec := New(nil, nil, false)
	exec.createdResources["1-create-gateway"] = "gateway-id"
	exec.createdResources["2-create-store"] = "store-id"

	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
	plan.AddChange(planner.PlannedChange{
		ID:           "1-create-gateway",
		ResourceType: planner.ResourceTypeAIGateway,
		ResourceRef:  "support-gateway",
		Action:       planner.ActionCreate,
	})
	plan.AddChange(planner.PlannedChange{
		ID:           "2-create-store",
		ResourceType: planner.ResourceTypeAIGatewayConfigStore,
		ResourceRef:  "support-store",
		Action:       planner.ActionCreate,
	})
	change := planner.PlannedChange{
		ID:           "3-create-secret",
		ResourceType: planner.ResourceTypeAIGatewayConfigStoreSecret,
		ResourceRef:  "support-api-key",
		Action:       planner.ActionCreate,
		DependsOn:    []string{"1-create-gateway", "2-create-store"},
		References: map[string]planner.ReferenceInfo{
			planner.FieldAIGatewayID: {
				Ref: "support-gateway",
				ID:  resources.UnknownReferenceID,
			},
			planner.FieldConfigStoreID: {
				Ref: "support-store",
				ID:  resources.UnknownReferenceID,
			},
		},
	}
	plan.AddChange(change)

	exec.hydrateKnownReferenceIDs(&change, plan)

	require.Equal(t, "gateway-id", change.References[planner.FieldAIGatewayID].ID)
	require.Equal(t, "store-id", change.References[planner.FieldConfigStoreID].ID)
}
