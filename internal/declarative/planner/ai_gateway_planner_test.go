package planner

import (
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayPlannerCreateUsesExplicitNameNotRef(t *testing.T) {
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{},
	})
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{Ref: "local-support-gateway"},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "support-gateway",
				DisplayName: "Support Gateway",
			},
		}},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypeAIGateway, change.ResourceType)
	require.Equal(t, "local-support-gateway", change.ResourceRef)
	require.Equal(t, "support-gateway", change.Fields[FieldName])
	require.Equal(t, "Support Gateway", change.Fields[FieldDisplayName])
}

func TestAIGatewayPlannerMatchesByNameBeforeDisplayName(t *testing.T) {
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{{
				ID:          "gateway-id",
				Name:        "support-gateway",
				DisplayName: "Old Support Gateway",
				Labels:      map[string]string{labels.NamespaceKey: "default"},
			}},
		},
	})
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{Ref: "local-support-gateway"},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "support-gateway",
				DisplayName: "New Support Gateway",
			},
		}},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	require.Equal(t, ActionUpdate, change.Action)
	require.Equal(t, ResourceTypeAIGateway, change.ResourceType)
	require.Equal(t, "local-support-gateway", change.ResourceRef)
	require.Equal(t, "gateway-id", change.ResourceID)
	require.Equal(t, "support-gateway", change.Fields[FieldName])
	require.Equal(t, "New Support Gateway", change.Fields[FieldDisplayName])
	require.Contains(t, change.ChangedFields, FieldDisplayName)
}

func TestAIGatewayPlannerCanMigrateLegacyRefNameToExplicitName(t *testing.T) {
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{{
				ID:          "gateway-id",
				Name:        "local-support-gateway",
				DisplayName: "Support Gateway",
				Labels:      map[string]string{labels.NamespaceKey: "default"},
			}},
		},
	})
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{Ref: "local-support-gateway"},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "support-gateway",
				DisplayName: "Support Gateway",
			},
		}},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	require.Equal(t, ActionUpdate, change.Action)
	require.Equal(t, ResourceTypeAIGateway, change.ResourceType)
	require.Equal(t, "gateway-id", change.ResourceID)
	require.Equal(t, "support-gateway", change.Fields[FieldName])
	require.Equal(t, FieldChange{Old: "local-support-gateway", New: "support-gateway"}, change.ChangedFields[FieldName])
}

func TestAIGatewayPlannerDeleteUsesNameBeforeDisplayName(t *testing.T) {
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{{
				ID:          "gateway-id",
				Name:        "support-gateway",
				DisplayName: "Support Gateway With Spaces",
				Labels:      map[string]string{labels.NamespaceKey: "default"},
			}},
		},
	})
	rs := &resources.ResourceSet{}
	rs.EnsureSyncScope().AddRoot(resources.ResourceTypeAIGateway)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	require.Equal(t, ActionDelete, change.Action)
	require.Equal(t, ResourceTypeAIGateway, change.ResourceType)
	require.Equal(t, "support-gateway", change.ResourceRef)
	require.Contains(t, change.ID, "support-gateway")
}

func TestAIGatewayChildCreateLooksUpNewGatewayByName(t *testing.T) {
	agent := testAIGatewayAgentResource(t, nil)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{},
	})
	rs := testAIGatewayAgentResourceSet(agent)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)

	change := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayAgent, "booking-agent")
	require.NotNil(t, change.References)
	refInfo := change.References[FieldAIGatewayID]
	require.Equal(t, "support-gateway", refInfo.Ref)
	require.Equal(t, map[string]string{FieldName: "support-gateway"}, refInfo.LookupFields)
}
