package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

type stubAuditLogDestinationsAPI struct {
	destinations []helpers.AuditLogDestination
}

func (s *stubAuditLogDestinationsAPI) ListAuditLogDestinations(
	_ context.Context,
) ([]helpers.AuditLogDestination, error) {
	return s.destinations, nil
}

type stubPortalAuditLogsAPI struct {
	current *kkComps.PortalAuditLogWebhook
}

func (s *stubPortalAuditLogsAPI) GetPortalAuditLogWebhook(
	_ context.Context,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.GetPortalAuditLogWebhookResponse, error) {
	return &kkOps.GetPortalAuditLogWebhookResponse{PortalAuditLogWebhook: s.current}, nil
}

func (s *stubPortalAuditLogsAPI) UpdatePortalAuditLogWebhook(
	_ context.Context,
	_ string,
	_ *kkComps.UpdatePortalAuditLogWebhook,
	_ ...kkOps.Option,
) (*kkOps.UpdatePortalAuditLogWebhookResponse, error) {
	return &kkOps.UpdatePortalAuditLogWebhookResponse{PortalAuditLogWebhook: &kkComps.PortalAuditLogWebhook{}}, nil
}

func (s *stubPortalAuditLogsAPI) DeletePortalAuditLogWebhook(
	_ context.Context,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DeletePortalAuditLogWebhookResponse, error) {
	return &kkOps.DeletePortalAuditLogWebhookResponse{}, nil
}

func TestResolveAuditLogWebhookDestinationIdentitiesByName(t *testing.T) {
	client := state.NewClient(state.ClientConfig{
		AuditLogDestinationsAPI: &stubAuditLogDestinationsAPI{
			destinations: []helpers.AuditLogDestination{
				{ID: "dest-id", Name: "foo", Endpoint: "https://example.test/audit-logs"},
			},
		},
	})
	planner := NewPlanner(client, slog.Default())

	destinations := []resources.AuditLogWebhookDestinationResource{
		{
			BaseResource: resources.BaseResource{Ref: "foo"},
			External: &resources.ExternalBlock{
				Selector: &resources.ExternalSelector{
					MatchFields: map[string]string{"name": "foo"},
				},
			},
		},
	}

	err := planner.resolveAuditLogWebhookDestinationIdentities(context.Background(), destinations)
	require.NoError(t, err)
	require.Equal(t, "dest-id", destinations[0].GetKonnectID())
}

func TestPlanPortalAuditLogWebhookCreateUsesResolvedDestinationReference(t *testing.T) {
	destination := resources.AuditLogWebhookDestinationResource{
		BaseResource: resources.BaseResource{Ref: "foo"},
		External: &resources.ExternalBlock{
			Selector: &resources.ExternalSelector{
				MatchFields: map[string]string{"name": "foo"},
			},
		},
	}
	destination.SetKonnectID("dest-id")

	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{BaseResource: resources.BaseResource{Ref: "portal"}},
		},
		AuditLogs: &resources.AuditLogsResource{
			Destinations: []resources.AuditLogWebhookDestinationResource{destination},
		},
	}
	enabled := true
	desired := []resources.PortalAuditLogWebhookResource{
		{
			Ref:                   "portal-audit-log-webhook",
			Portal:                "portal",
			Enabled:               &enabled,
			AuditLogDestinationID: tags.RefPlaceholderPrefix + "foo#id",
		},
	}

	client := state.NewClient(state.ClientConfig{
		PortalAuditLogsAPI: &stubPortalAuditLogsAPI{},
	})
	planner := NewPlanner(client, slog.Default())
	planner.resources = rs
	planner.desiredPortals = rs.Portals

	plan := NewPlan("1.0", "test", PlanModeApply)
	err := planner.planPortalAuditLogWebhooksChanges(
		context.Background(),
		DefaultNamespace,
		"portal-id",
		"portal",
		desired,
		plan,
	)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	require.Equal(t, ResourceTypePortalAuditLogWebhook, change.ResourceType)
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, "dest-id", change.Fields[FieldAuditLogDestinationID])
	require.Equal(t, true, change.Fields[FieldEnabled])
	require.Equal(t, "portal", change.References[FieldPortalID].Ref)
	require.Equal(t, "portal-id", change.References[FieldPortalID].ID)
	require.Equal(t, "foo", change.References[FieldAuditLogDestinationID].Ref)
	require.Equal(t, "dest-id", change.References[FieldAuditLogDestinationID].ID)
}
