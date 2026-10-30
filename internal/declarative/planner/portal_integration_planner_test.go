package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubPortalIntegrationsAPI struct {
	current *kkComps.PortalIntegrations
}

func (s *stubPortalIntegrationsAPI) GetPortalIntegrations(
	_ context.Context,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.GetPortalIntegrationsResponse, error) {
	return &kkOps.GetPortalIntegrationsResponse{PortalIntegrations: s.current}, nil
}

func (s *stubPortalIntegrationsAPI) UpsertPortalIntegrations(
	_ context.Context,
	_ string,
	_ *kkComps.PortalIntegrations,
	_ ...kkOps.Option,
) (*kkOps.UpsertPortalIntegrationsResponse, error) {
	return &kkOps.UpsertPortalIntegrationsResponse{}, nil
}

func TestPlanPortalIntegrations_UpdateWhenStateDiffers(t *testing.T) {
	current := &kkComps.PortalIntegrations{
		GoogleAnalytics4: new(googleAnalytics4Integration("G-OLD", true)),
	}
	desired := resources.PortalIntegrationResource{
		Ref:    "portal-integrations",
		Portal: "portal",
		PortalIntegrations: kkComps.PortalIntegrations{
			GoogleTagManager: new(googleTagManagerIntegration("GTM-ABC123", true)),
			GoogleAnalytics4: new(googleAnalytics4Integration("G-NEW123", false)),
		},
	}

	planner := NewPlanner(
		state.NewClient(state.ClientConfig{PortalIntegrationsAPI: &stubPortalIntegrationsAPI{current: current}}),
		slog.Default(),
	)
	planner.desiredPortals = []resources.PortalResource{{
		BaseResource: resources.BaseResource{Ref: "portal"},
		CreatePortal: kkComps.CreatePortal{Name: "Portal"},
	}}
	plan := NewPlan("1.0", "test", PlanModeApply)

	err := planner.planPortalIntegrationsChanges(
		context.Background(), "default", "portal-id", "portal", []resources.PortalIntegrationResource{desired}, plan,
	)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	assert.Equal(t, ResourceTypePortalIntegration, change.ResourceType)
	assert.Equal(t, ActionUpdate, change.Action)
	assert.Equal(t, "portal-integrations", change.ResourceRef)
	assert.Equal(t, "portal-id", change.References[FieldPortalID].ID)
	assert.Contains(t, change.Fields, FieldGoogleTagManager)
	assert.Contains(t, change.Fields, FieldGoogleAnalytics4)
	assert.Contains(t, change.ChangedFields, FieldGoogleTagManager)
	assert.Contains(t, change.ChangedFields, FieldGoogleAnalytics4)
}

func TestPlanPortalIntegrations_NoChangeWhenStateMatches(t *testing.T) {
	current := &kkComps.PortalIntegrations{
		GoogleTagManager: new(googleTagManagerIntegration("GTM-ABC123", true)),
	}
	desired := resources.PortalIntegrationResource{
		Ref:                "portal-integrations",
		Portal:             "portal",
		PortalIntegrations: *current,
	}

	planner := NewPlanner(
		state.NewClient(state.ClientConfig{PortalIntegrationsAPI: &stubPortalIntegrationsAPI{current: current}}),
		slog.Default(),
	)
	plan := NewPlan("1.0", "test", PlanModeApply)

	err := planner.planPortalIntegrationsChanges(
		context.Background(), "default", "portal-id", "portal", []resources.PortalIntegrationResource{desired}, plan,
	)
	require.NoError(t, err)
	assert.Empty(t, plan.Changes)
}

func googleTagManagerIntegration(id string, enabled bool) kkComps.GoogleTagManagerIntegration {
	return kkComps.GoogleTagManagerIntegration{
		Enabled: enabled,
		Type:    kkComps.GoogleTagManagerIntegrationTypeTracking,
		ConfigData: kkComps.ConfigData{
			ID: id,
		},
	}
}

func googleAnalytics4Integration(id string, enabled bool) kkComps.GoogleAnalytics4Integration {
	return kkComps.GoogleAnalytics4Integration{
		Enabled: enabled,
		Type:    kkComps.GoogleAnalytics4IntegrationTypeAnalytics,
		ConfigData: kkComps.GoogleAnalytics4IntegrationConfigData{
			ID: id,
		},
	}
}
