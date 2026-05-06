package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

type stubPortalIPAllowListAPI struct {
	entries []kkComps.IPEntry
}

func (s *stubPortalIPAllowListAPI) CreatePortalIPAllowList(
	_ context.Context,
	_ string,
	_ *kkComps.CreatePortalSourceIPRestriction,
	_ ...kkOps.Option,
) (*kkOps.CreatePortalIPAllowListResponse, error) {
	return &kkOps.CreatePortalIPAllowListResponse{IPEntry: &kkComps.IPEntry{ID: "created-entry"}}, nil
}

func (s *stubPortalIPAllowListAPI) ListPortalIPAllowList(
	_ context.Context,
	_ kkOps.ListPortalIPAllowListRequest,
	_ ...kkOps.Option,
) (*kkOps.ListPortalIPAllowListResponse, error) {
	return &kkOps.ListPortalIPAllowListResponse{
		PortalSourceIPRestrictionPaginatedResponse: &kkComps.PortalSourceIPRestrictionPaginatedResponse{
			Meta: kkComps.CursorMetaPage{Size: 100},
			Data: s.entries,
		},
	}, nil
}

func (s *stubPortalIPAllowListAPI) PutPortalIPAllowList(
	_ context.Context,
	_ kkOps.PutPortalIPAllowListRequest,
	_ ...kkOps.Option,
) (*kkOps.PutPortalIPAllowListResponse, error) {
	return nil, nil
}

func (s *stubPortalIPAllowListAPI) UpdatePortalIPAllowList(
	_ context.Context,
	_ kkOps.UpdatePortalIPAllowListRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdatePortalIPAllowListResponse, error) {
	return &kkOps.UpdatePortalIPAllowListResponse{IPEntry: &kkComps.IPEntry{ID: "updated-entry"}}, nil
}

func (s *stubPortalIPAllowListAPI) DeletePortalIPAllowList(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DeletePortalIPAllowListResponse, error) {
	return &kkOps.DeletePortalIPAllowListResponse{}, nil
}

func TestPlanPortalIPAllowListsChangesCreatesWhenMissing(t *testing.T) {
	planner := newPortalIPAllowListPlanner(nil)
	plan := NewPlan("1.0", "test", PlanModeApply)

	err := planner.planPortalIPAllowListsChanges(
		context.Background(),
		DefaultNamespace,
		"portal-id",
		"portal-ref",
		[]resources.PortalIPAllowListResource{{
			Ref:        "portal-allow-list",
			Portal:     "portal-ref",
			AllowedIPs: []string{"198.51.100.0/24", "192.0.2.10"},
		}},
		plan,
	)

	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypePortalIPAllowList, change.ResourceType)
	require.Equal(t, []string{"192.0.2.10", "198.51.100.0/24"}, change.Fields[FieldAllowedIPs])
	require.Equal(t, "portal-id", change.References[FieldPortalID].ID)
}

func TestPlanPortalIPAllowListsChangesSkipsEquivalentEntry(t *testing.T) {
	planner := newPortalIPAllowListPlanner([]kkComps.IPEntry{{
		ID:         "entry-1",
		AllowedIps: []string{"198.51.100.0/24", "192.0.2.10"},
	}})
	plan := NewPlan("1.0", "test", PlanModeApply)

	err := planner.planPortalIPAllowListsChanges(
		context.Background(),
		DefaultNamespace,
		"portal-id",
		"portal-ref",
		[]resources.PortalIPAllowListResource{{
			Ref:        "portal-allow-list",
			Portal:     "portal-ref",
			AllowedIPs: []string{"192.0.2.10", "198.51.100.0/24"},
		}},
		plan,
	)

	require.NoError(t, err)
	require.Empty(t, plan.Changes)
}

func TestPlanPortalIPAllowListsChangesUpdatesDifferentEntry(t *testing.T) {
	planner := newPortalIPAllowListPlanner([]kkComps.IPEntry{{
		ID:         "entry-1",
		AllowedIps: []string{"203.0.113.7"},
	}})
	plan := NewPlan("1.0", "test", PlanModeApply)

	err := planner.planPortalIPAllowListsChanges(
		context.Background(),
		DefaultNamespace,
		"portal-id",
		"portal-ref",
		[]resources.PortalIPAllowListResource{{
			Ref:        "portal-allow-list",
			Portal:     "portal-ref",
			AllowedIPs: []string{"192.0.2.10"},
		}},
		plan,
	)

	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionUpdate, change.Action)
	require.Equal(t, "entry-1", change.ResourceID)
	require.Equal(t, []string{"192.0.2.10"}, change.Fields[FieldAllowedIPs])
}

func TestPlanPortalIPAllowListsChangesSyncDeletesOmittedEntries(t *testing.T) {
	planner := newPortalIPAllowListPlanner([]kkComps.IPEntry{{
		ID:         "entry-1",
		AllowedIps: []string{"192.0.2.10"},
	}})
	plan := NewPlan("1.0", "test", PlanModeSync)

	err := planner.planPortalIPAllowListsChanges(
		context.Background(),
		DefaultNamespace,
		"portal-id",
		"portal-ref",
		nil,
		plan,
	)

	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionDelete, change.Action)
	require.Equal(t, "entry-1", change.ResourceID)
	require.Equal(t, []string{"192.0.2.10"}, change.Fields[FieldAllowedIPs])
}

func TestPlanPortalIPAllowListsChangesErrorsWhenClientMissingForDesiredResource(t *testing.T) {
	client := state.NewClient(state.ClientConfig{})
	planner := NewPlanner(client, slog.Default())
	planner.desiredPortals = []resources.PortalResource{{
		BaseResource: resources.BaseResource{Ref: "portal-ref"},
		CreatePortal: kkComps.CreatePortal{
			Name: "Portal",
		},
	}}
	plan := NewPlan("1.0", "test", PlanModeApply)

	err := planner.planPortalIPAllowListsChanges(
		context.Background(),
		DefaultNamespace,
		"portal-id",
		"portal-ref",
		[]resources.PortalIPAllowListResource{{
			Ref:        "portal-allow-list",
			Portal:     "portal-ref",
			AllowedIPs: []string{"192.0.2.10"},
		}},
		plan,
	)

	require.EqualError(
		t,
		err,
		"cannot manage portal IP allow list \"portal-allow-list\" for portal \"portal-ref\": "+
			"portal IP allow list API client not configured",
	)
	require.Empty(t, plan.Changes)
}

func newPortalIPAllowListPlanner(entries []kkComps.IPEntry) *Planner {
	client := state.NewClient(state.ClientConfig{
		PortalIPAllowListAPI: &stubPortalIPAllowListAPI{entries: entries},
	})
	planner := NewPlanner(client, slog.Default())
	planner.desiredPortals = []resources.PortalResource{{
		BaseResource: resources.BaseResource{Ref: "portal-ref"},
		CreatePortal: kkComps.CreatePortal{
			Name: "Portal",
		},
	}}
	return planner
}
