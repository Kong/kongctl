package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestManagedByProvenanceInPlans(t *testing.T) {
	ctx := context.WithValue(t.Context(), build.InfoKey, &build.Info{Version: "1.2.3-preview"})
	for _, kind := range []string{
		ResourceTypeAIGatewayModel, ResourceTypeAIGatewayProvider,
		ResourceTypeAIGatewayConsumerCredential, ResourceTypeAIGatewayCertificate,
	} {
		t.Run(kind, func(t *testing.T) {
			for _, action := range []ActionType{ActionCreate, ActionUpdate} {
				plan := NewPlan(CurrentPlanVersion, "kongctl/old-planner", PlanModeApply)
				plan.AddChange(PlannedChange{
					ResourceType: kind, Action: action,
					Fields: map[string]any{FieldManagedBy: map[string]string{"external": "preserved"}},
				})
				require.NoError(t, stampPlanManagedBy(ctx, plan))
				require.Equal(t, map[string]string{"tool": "kongctl", "version": "1.2.3-preview", "external": "preserved"},
					plan.Changes[0].Fields[FieldManagedBy])
			}
		})
	}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	for _, kind := range []string{ResourceTypeAIGatewayConfigStore, ResourceTypeAIGatewayDataPlaneCertificate} {
		plan.AddChange(PlannedChange{ResourceType: kind, Action: ActionCreate, Fields: map[string]any{}})
	}
	plan.AddChange(PlannedChange{ResourceType: ResourceTypeAIGatewayModel, Action: ActionDelete})
	require.NoError(t, stampPlanManagedBy(ctx, plan))
	for _, change := range plan.Changes {
		require.NotContains(t, change.Fields, FieldManagedBy)
	}
}

func TestManagedByVersionUpgradeConvergesAndKeepsProtection(t *testing.T) {
	for _, protected := range []bool{false, true} {
		model := federatedModel(t, "owned", "team-a", "model")
		model.Kongctl.Protected = new(protected)
		observed := observedFederatedModel(t, model)
		metadata := resources.AIGatewayModelManagedBy(observed)
		metadata[resources.ManagedByVersionKey] = "old-version"
		api := &providerOrderModelAPI{
			testAIGatewayModelAPI: testAIGatewayModelAPI{models: []kkComps.AIGatewayModel{observed}},
		}
		p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayModelAPI: api}), slog.Default())
		ctx := context.WithValue(t.Context(), build.InfoKey, &build.Info{Version: "new-version"})
		plan, err := p.GeneratePlan(ctx, federatedResourceSet(model), Options{Mode: PlanModeApply})
		require.NoError(t, err)
		require.Empty(t, plan.Changes)
		if protected {
			model.Kongctl.Protected = new(false)
		} else {
			model.AIGatewayModelModel.DisplayName = "Updated"
		}
		plan, err = p.GeneratePlan(ctx, federatedResourceSet(model), Options{Mode: PlanModeApply})
		require.NoError(t, err)
		require.Len(t, plan.Changes, 1)
		data := normalizeAIGatewayJSONMap(plan.Changes[0].Fields)[FieldManagedBy].(map[string]any)
		require.Equal(t, "new-version", data[resources.ManagedByVersionKey])
	}
}

func TestGeneratedManagedByDoesNotCreatePayloadDrift(t *testing.T) {
	current := map[string]any{FieldManagedBy: map[string]string{"tool": "kongctl", "version": "older"}}
	desired := map[string]any{}
	require.Empty(t, diffAIGatewayPayloads(current, desired, current, desired))
	require.Empty(t, diffAIGatewayTLSPayloads(current, desired))
	current[FieldManagedBy].(map[string]string)["external"] = "preserve-real-drift"
	require.Contains(t, diffAIGatewayPayloads(current, desired, current, desired), FieldManagedBy)
	require.Contains(t, diffAIGatewayTLSPayloads(current, desired), FieldManagedBy)
}
