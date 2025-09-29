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
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestControlPlanePlanner_PlanCreate(t *testing.T) {
	mockAPI := helpers.NewMockControlPlaneAPI(t)
	mockAPI.EXPECT().
		ListControlPlanes(mock.Anything, mock.Anything).
		Return(newListControlPlaneResponse(nil, 0), nil).
		Once()

	client := state.NewClient(state.ClientConfig{ControlPlaneAPI: mockAPI})
	planner := &Planner{
		client: client,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	planner.genericPlanner = NewGenericPlanner(planner)
	planner.resources = &resources.ResourceSet{
		ControlPlanes: []resources.ControlPlaneResource{
			{
				CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{
					Name:        "cp-create",
					Description: strPtr("Control Plane"),
				},
				Ref: "cp-create",
				Kongctl: &resources.KongctlMeta{
					Namespace: strPtr("default"),
				},
			},
		},
	}

	base := NewBasePlanner(planner)
	cpPlanner := NewControlPlanePlanner(base)

	plan := NewPlan("1.0", "test", PlanModeApply)
	err := cpPlanner.PlanChanges(context.Background(), NewConfig("default"), plan)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	assert.Equal(t, ActionCreate, change.Action)
	assert.Equal(t, "control_plane", change.ResourceType)
	assert.Equal(t, "default", change.Namespace)
	assert.Equal(t, "cp-create", change.Fields["name"])
	assert.Equal(t, false, change.Protection)
}

func TestControlPlanePlanner_PlanUpdate(t *testing.T) {
	mockAPI := helpers.NewMockControlPlaneAPI(t)
	current := kkComps.ControlPlane{
		ID:          "cp-1",
		Name:        "cp-update",
		Description: strPtr("old"),
		Labels: map[string]string{
			labels.NamespaceKey: "default",
		},
		Config: kkComps.Config{
			AuthType:    kkComps.ControlPlaneAuthTypePinnedClientCerts,
			ProxyUrls:   []kkComps.ProxyURL{{Host: "example.com", Port: 443, Protocol: "https"}},
			ClusterType: kkComps.ControlPlaneClusterTypeClusterTypeControlPlane,
		},
	}

	resp := newListControlPlaneResponse([]kkComps.ControlPlane{current}, 1)
	mockAPI.EXPECT().
		ListControlPlanes(mock.Anything, mock.Anything).
		Return(resp, nil).
		Once()

	client := state.NewClient(state.ClientConfig{ControlPlaneAPI: mockAPI})
	planner := &Planner{
		client: client,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	planner.genericPlanner = NewGenericPlanner(planner)
	planner.resources = &resources.ResourceSet{
		ControlPlanes: []resources.ControlPlaneResource{
			{
				CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{
					Name:        "cp-update",
					Description: strPtr("new"),
					ProxyUrls:   []kkComps.ProxyURL{{Host: "example.com", Port: 8443, Protocol: "https"}},
				},
				Ref: "cp-update",
				Kongctl: &resources.KongctlMeta{
					Namespace: strPtr("default"),
				},
			},
		},
	}

	base := NewBasePlanner(planner)
	cpPlanner := NewControlPlanePlanner(base)

	plan := NewPlan("1.0", "test", PlanModeApply)
	err := cpPlanner.PlanChanges(context.Background(), NewConfig("default"), plan)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	assert.Equal(t, ActionUpdate, change.Action)
	assert.Equal(t, "cp-update", change.ResourceRef)
	assert.Equal(t, "cp-1", change.ResourceID)
	assert.Equal(t, "new", change.Fields["description"])
	assert.Equal(
		t,
		[]kkComps.ProxyURL{{Host: "example.com", Port: 8443, Protocol: "https"}},
		change.Fields["proxy_urls"],
	)
	assert.Nil(t, change.Protection)
}

func TestControlPlanePlanner_PlanDeleteSync(t *testing.T) {
	mockAPI := helpers.NewMockControlPlaneAPI(t)
	current := kkComps.ControlPlane{
		ID:   "cp-delete",
		Name: "cp-delete",
		Labels: map[string]string{
			labels.NamespaceKey: "default",
		},
	}

	resp := newListControlPlaneResponse([]kkComps.ControlPlane{current}, 1)
	mockAPI.EXPECT().
		ListControlPlanes(mock.Anything, mock.Anything).
		Return(resp, nil).
		Once()

	client := state.NewClient(state.ClientConfig{ControlPlaneAPI: mockAPI})
	planner := &Planner{
		client: client,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	planner.genericPlanner = NewGenericPlanner(planner)
	planner.resources = &resources.ResourceSet{ControlPlanes: []resources.ControlPlaneResource{}}

	base := NewBasePlanner(planner)
	cpPlanner := NewControlPlanePlanner(base)

	plan := NewPlan("1.0", "test", PlanModeSync)
	err := cpPlanner.PlanChanges(context.Background(), NewConfig("default"), plan)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	assert.Equal(t, ActionDelete, plan.Changes[0].Action)
	assert.Equal(t, "cp-delete", plan.Changes[0].ResourceRef)
}

func TestControlPlanePlanner_ProtectionChange(t *testing.T) {
	mockAPI := helpers.NewMockControlPlaneAPI(t)
	current := kkComps.ControlPlane{
		ID:   "cp-protect",
		Name: "cp-protect",
		Labels: map[string]string{
			labels.NamespaceKey: "default",
		},
	}

	resp := newListControlPlaneResponse([]kkComps.ControlPlane{current}, 1)
	mockAPI.EXPECT().
		ListControlPlanes(mock.Anything, mock.Anything).
		Return(resp, nil).
		Once()

	client := state.NewClient(state.ClientConfig{ControlPlaneAPI: mockAPI})
	planner := &Planner{
		client: client,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	planner.genericPlanner = NewGenericPlanner(planner)
	planner.resources = &resources.ResourceSet{
		ControlPlanes: []resources.ControlPlaneResource{
			{
				CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{
					Name: "cp-protect",
				},
				Ref: "cp-protect",
				Kongctl: &resources.KongctlMeta{
					Namespace: strPtr("default"),
					Protected: boolPtr(true),
				},
			},
		},
	}

	base := NewBasePlanner(planner)
	cpPlanner := NewControlPlanePlanner(base)

	plan := NewPlan("1.0", "test", PlanModeApply)
	err := cpPlanner.PlanChanges(context.Background(), NewConfig("default"), plan)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	assert.Equal(t, ActionUpdate, change.Action)
	assert.IsType(t, ProtectionChange{}, change.Protection)
	pc := change.Protection.(ProtectionChange)
	assert.False(t, pc.Old)
	assert.True(t, pc.New)
	assert.Equal(t, "cp-protect", change.Fields["name"])
}

func newListControlPlaneResponse(data []kkComps.ControlPlane, total float64) *kkOps.ListControlPlanesResponse {
	return &kkOps.ListControlPlanesResponse{
		ListControlPlanesResponse: &kkComps.ListControlPlanesResponse{
			Data: data,
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: total},
			},
		},
	}
}

func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
