package planner

import (
	"context"
	"io"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dashboardAPITestStub struct {
	dashboards []kkComps.DashboardResponse
}

func (s *dashboardAPITestStub) DashboardsList(
	_ context.Context,
	_ kkOps.DashboardsListRequest,
	_ ...kkOps.Option,
) (*kkOps.DashboardsListResponse, error) {
	return &kkOps.DashboardsListResponse{
		Object: &kkOps.DashboardsListResponseBody{
			Meta: &kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: float64(len(s.dashboards))},
			},
			Data: s.dashboards,
		},
	}, nil
}

func (s *dashboardAPITestStub) DashboardsCreate(
	_ context.Context,
	_ kkComps.DashboardUpdateRequest,
	_ ...kkOps.Option,
) (*kkOps.DashboardsCreateResponse, error) {
	return nil, nil
}

func (s *dashboardAPITestStub) DashboardsGet(
	_ context.Context,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DashboardsGetResponse, error) {
	return nil, nil
}

func (s *dashboardAPITestStub) DashboardsUpdate(
	_ context.Context,
	_ string,
	_ kkComps.DashboardUpdateRequest,
	_ ...kkOps.Option,
) (*kkOps.DashboardsUpdateResponse, error) {
	return nil, nil
}

func (s *dashboardAPITestStub) DashboardsDelete(
	_ context.Context,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DashboardsDeleteResponse, error) {
	return nil, nil
}

func TestDashboardPlannerPlansCreate(t *testing.T) {
	planner := newDashboardTestPlanner(nil)
	desired := []resources.DashboardResource{
		newDashboardResource("traffic-summary", "Traffic Summary"),
	}

	plan := NewPlan("1.0", "test", PlanModeApply)
	err := planner.planDashboardChanges(t.Context(), NewConfig("analytics"), desired, plan)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	assert.Equal(t, ActionCreate, change.Action)
	assert.Equal(t, ResourceTypeDashboard, change.ResourceType)
	assert.Equal(t, "traffic-summary", change.ResourceRef)
	assert.Equal(t, "Traffic Summary", change.Fields[FieldName])
	assert.Equal(t, "analytics", change.Namespace)
	assert.NotNil(t, change.Fields[FieldDefinition])
}

func TestDashboardPlannerPlansUpdateByName(t *testing.T) {
	id := "dashboard-id"
	current := kkComps.DashboardResponse{
		ID:   &id,
		Name: "Traffic Summary",
		Definition: kkComps.Dashboard{
			Tiles: []kkComps.Tile{},
		},
		Labels: map[string]string{
			labels.NamespaceKey: "analytics",
			"team":              "platform",
		},
	}
	planner := newDashboardTestPlanner([]kkComps.DashboardResponse{current})
	desired := []resources.DashboardResource{
		newDashboardResource("traffic-summary", "Traffic Summary"),
	}

	plan := NewPlan("1.0", "test", PlanModeApply)
	err := planner.planDashboardChanges(t.Context(), NewConfig("analytics"), desired, plan)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	assert.Equal(t, ActionUpdate, change.Action)
	assert.Equal(t, "dashboard-id", change.ResourceID)
	assert.Equal(t, "traffic-summary", change.ResourceRef)
	assert.Equal(t, "Traffic Summary", change.Fields[FieldName])
	assert.Contains(t, change.ChangedFields, FieldDefinition)
}

func TestDashboardPlannerRejectsAmbiguousDashboardName(t *testing.T) {
	firstID := "11111111-1111-1111-1111-111111111111"
	secondID := "22222222-2222-2222-2222-222222222222"
	current := []kkComps.DashboardResponse{
		{
			ID:   &firstID,
			Name: "Traffic Summary",
			Definition: kkComps.Dashboard{
				Tiles: []kkComps.Tile{},
			},
			Labels: map[string]string{
				labels.NamespaceKey: "analytics",
			},
		},
		{
			ID:   &secondID,
			Name: "Traffic Summary",
			Definition: kkComps.Dashboard{
				Tiles: []kkComps.Tile{},
			},
			Labels: map[string]string{
				labels.NamespaceKey: "analytics",
			},
		},
	}
	planner := newDashboardTestPlanner(current)
	desired := []resources.DashboardResource{
		newDashboardResource("traffic-summary", "Traffic Summary"),
	}

	plan := NewPlan("1.0", "test", PlanModeApply)
	err := planner.planDashboardChanges(t.Context(), NewConfig("analytics"), desired, plan)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `multiple managed dashboards named "Traffic Summary"`)
}

func TestDashboardPlannerMatchesDashboardByUUIDRef(t *testing.T) {
	firstID := "11111111-1111-1111-1111-111111111111"
	secondID := "22222222-2222-2222-2222-222222222222"
	current := []kkComps.DashboardResponse{
		{
			ID:   &firstID,
			Name: "Traffic Summary",
			Definition: kkComps.Dashboard{
				Tiles: []kkComps.Tile{},
			},
			Labels: map[string]string{
				labels.NamespaceKey: "analytics",
			},
		},
		{
			ID:   &secondID,
			Name: "Traffic Summary",
			Definition: kkComps.Dashboard{
				Tiles: []kkComps.Tile{},
			},
			Labels: map[string]string{
				labels.NamespaceKey: "analytics",
			},
		},
	}
	planner := newDashboardTestPlanner(current)
	desired := []resources.DashboardResource{
		newDashboardResource(secondID, "Renamed Traffic Summary"),
	}

	plan := NewPlan("1.0", "test", PlanModeApply)
	err := planner.planDashboardChanges(t.Context(), NewConfig("analytics"), desired, plan)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	assert.Equal(t, ActionUpdate, change.Action)
	assert.Equal(t, secondID, change.ResourceID)
	assert.Equal(t, secondID, change.ResourceRef)
	assert.Equal(t, "Renamed Traffic Summary", change.Fields[FieldName])
	assert.Contains(t, change.ChangedFields, FieldName)
}

func TestDashboardPlannerSyncDeletesOmittedManagedDashboard(t *testing.T) {
	id := "dashboard-id"
	current := kkComps.DashboardResponse{
		ID:   &id,
		Name: "Traffic Summary",
		Definition: kkComps.Dashboard{
			Tiles: []kkComps.Tile{},
		},
		Labels: map[string]string{
			labels.NamespaceKey: "analytics",
		},
	}
	planner := newDashboardTestPlanner([]kkComps.DashboardResponse{current})

	plan := NewPlan("1.0", "test", PlanModeSync)
	err := planner.planDashboardChanges(t.Context(), NewConfig("analytics"), nil, plan)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	assert.Equal(t, ActionDelete, change.Action)
	assert.Equal(t, ResourceTypeDashboard, change.ResourceType)
	assert.Equal(t, "Traffic Summary", change.ResourceRef)
	assert.Equal(t, "dashboard-id", change.ResourceID)
}

func newDashboardTestPlanner(current []kkComps.DashboardResponse) *Planner {
	return &Planner{
		client: state.NewClient(state.ClientConfig{
			DashboardsAPI: &dashboardAPITestStub{dashboards: current},
		}),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func newDashboardResource(ref, name string) resources.DashboardResource {
	namespace := "analytics"
	return resources.DashboardResource{
		BaseResource: resources.BaseResource{
			Ref: ref,
			Kongctl: &resources.KongctlMeta{
				Namespace: &namespace,
			},
		},
		Name: name,
		Definition: kkComps.Dashboard{
			Tiles: []kkComps.Tile{},
			PresetFilters: []kkComps.AllFilterItems{
				{
					Field:    kkComps.AllFilterItemsFieldControlPlane,
					Operator: kkComps.AllFilterItemsOperatorIn,
					Value:    []any{"cp-id"},
				},
			},
		},
		Labels: map[string]string{"team": "platform"},
	}
}
