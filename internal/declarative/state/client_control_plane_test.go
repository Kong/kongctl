package state

import (
	"errors"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newListControlPlanesResponse(data []kkComps.ControlPlane, total float64) *kkOps.ListControlPlanesResponse {
	return &kkOps.ListControlPlanesResponse{
		ListControlPlanesResponse: &kkComps.ListControlPlanesResponse{
			Data: data,
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: total},
			},
		},
	}
}

func TestListManagedControlPlanes(t *testing.T) {
	ctx := testContextWithLogger()
	mockAPI := helpers.NewMockControlPlaneAPI(t)

	managed := kkComps.ControlPlane{
		ID:   "cp-managed",
		Name: "managed-cp",
		Labels: map[string]string{
			labels.NamespaceKey: "team-a",
		},
	}

	unmanaged := kkComps.ControlPlane{
		ID:   "cp-unmanaged",
		Name: "unmanaged",
		Labels: map[string]string{
			"env": "dev",
		},
	}

	pageOne := newListControlPlanesResponse([]kkComps.ControlPlane{managed, unmanaged}, 2)

	mockAPI.EXPECT().
		ListControlPlanes(mock.Anything, mock.Anything).
		Return(pageOne, nil).
		Once()

	client := NewClient(ClientConfig{ControlPlaneAPI: mockAPI})

	result, err := client.ListManagedControlPlanes(ctx, []string{"team-a"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "cp-managed", result[0].ID)
	assert.Equal(t, "team-a", result[0].NormalizedLabels[labels.NamespaceKey])
}

func TestGetControlPlaneByNameFallback(t *testing.T) {
	ctx := testContextWithLogger()
	mockAPI := helpers.NewMockControlPlaneAPI(t)

	legacy := kkComps.ControlPlane{
		ID:   "cp-fallback",
		Name: "legacy-cp",
		Labels: map[string]string{
			labels.ProtectedKey: labels.TrueValue,
		},
	}

	managedCall := newListControlPlanesResponse([]kkComps.ControlPlane{legacy}, 1)
	allCall := newListControlPlanesResponse([]kkComps.ControlPlane{legacy}, 1)

	mockAPI.EXPECT().
		ListControlPlanes(mock.Anything, mock.Anything).
		Return(managedCall, nil).
		Once()

	mockAPI.EXPECT().
		ListControlPlanes(mock.Anything, mock.Anything).
		Return(allCall, nil).
		Once()

	client := NewClient(ClientConfig{ControlPlaneAPI: mockAPI})

	cp, err := client.GetControlPlaneByName(ctx, "legacy-cp")
	require.NoError(t, err)
	require.NotNil(t, cp)
	assert.Equal(t, "cp-fallback", cp.ID)
	assert.True(t, client.hasAnyKongctlLabels(cp.Labels))
}

func TestCreateControlPlane(t *testing.T) {
	ctx := testContextWithLogger()
	mockAPI := helpers.NewMockControlPlaneAPI(t)

	req := kkComps.CreateControlPlaneRequest{Name: "new-cp"}
	resp := &kkOps.CreateControlPlaneResponse{
		ControlPlane: &kkComps.ControlPlane{ID: "cp-123", Name: "new-cp"},
	}

	mockAPI.EXPECT().
		CreateControlPlane(mock.Anything, req).
		Return(resp, nil).
		Once()

	client := NewClient(ClientConfig{ControlPlaneAPI: mockAPI})

	created, err := client.CreateControlPlane(ctx, req, "team-a")
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "cp-123", created.ID)
}

func TestCreateControlPlaneError(t *testing.T) {
	ctx := testContextWithLogger()
	mockAPI := helpers.NewMockControlPlaneAPI(t)

	req := kkComps.CreateControlPlaneRequest{Name: "broken-cp"}
	mockAPI.EXPECT().
		CreateControlPlane(mock.Anything, req).
		Return(nil, errors.New("boom")).
		Once()

	client := NewClient(ClientConfig{ControlPlaneAPI: mockAPI})

	created, err := client.CreateControlPlane(ctx, req, "team-a")
	require.Error(t, err)
	assert.Nil(t, created)
	assert.Contains(t, err.Error(), "create control plane")
}

func TestUpdateControlPlane(t *testing.T) {
	ctx := testContextWithLogger()
	mockAPI := helpers.NewMockControlPlaneAPI(t)

	updateReq := kkComps.UpdateControlPlaneRequest{}
	resp := &kkOps.UpdateControlPlaneResponse{
		ControlPlane: &kkComps.ControlPlane{ID: "cp-123", Name: "updated"},
	}

	mockAPI.EXPECT().
		UpdateControlPlane(mock.Anything, "cp-123", updateReq).
		Return(resp, nil).
		Once()

	client := NewClient(ClientConfig{ControlPlaneAPI: mockAPI})

	updated, err := client.UpdateControlPlane(ctx, "cp-123", updateReq, "team-a")
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "updated", updated.Name)
}

func TestDeleteControlPlane(t *testing.T) {
	ctx := testContextWithLogger()
	mockAPI := helpers.NewMockControlPlaneAPI(t)

	mockAPI.EXPECT().
		DeleteControlPlane(mock.Anything, "cp-123").
		Return(&kkOps.DeleteControlPlaneResponse{}, nil).
		Once()

	client := NewClient(ClientConfig{ControlPlaneAPI: mockAPI})

	require.NoError(t, client.DeleteControlPlane(ctx, "cp-123"))
}

func TestListControlPlaneGroupMemberships(t *testing.T) {
	ctx := testContextWithLogger()
	mockGroups := &helpers.MockControlPlaneGroupsAPI{}

	groupID := "group-1"
	nextURL := "/v2/control-planes/group-1/group-memberships?page[after]=cursor-1"
	total := 2.0

	firstResp := &kkOps.GetControlPlanesIDGroupMembershipsResponse{
		ListGroupMemberships: &kkComps.ListGroupMemberships{
			Data: []kkComps.ControlPlane{
				{ID: "member-1"},
			},
			Meta: kkComps.CursorPaginatedMetaWithSizeAndTotal{
				Page: kkComps.CursorMetaWithSizeAndTotal{
					Next:  &nextURL,
					Size:  100,
					Total: &total,
				},
			},
		},
	}

	secondResp := &kkOps.GetControlPlanesIDGroupMembershipsResponse{
		ListGroupMemberships: &kkComps.ListGroupMemberships{
			Data: []kkComps.ControlPlane{
				{ID: "member-2"},
			},
			Meta: kkComps.CursorPaginatedMetaWithSizeAndTotal{
				Page: kkComps.CursorMetaWithSizeAndTotal{
					Size:  100,
					Total: kkSDK.Float64(2),
				},
			},
		},
	}

	mockGroups.On(
		"GetControlPlanesIDGroupMemberships",
		mock.Anything,
		mock.MatchedBy(func(req kkOps.GetControlPlanesIDGroupMembershipsRequest) bool {
			if req.ID != groupID {
				return false
			}
			return req.PageAfter == nil || (req.PageAfter != nil && *req.PageAfter == "")
		}),
	).Return(firstResp, nil).Once()

	mockGroups.On(
		"GetControlPlanesIDGroupMemberships",
		mock.Anything,
		mock.MatchedBy(func(req kkOps.GetControlPlanesIDGroupMembershipsRequest) bool {
			if req.ID != groupID {
				return false
			}
			return req.PageAfter != nil && *req.PageAfter == "cursor-1"
		}),
	).Return(secondResp, nil).Once()

	client := NewClient(ClientConfig{
		ControlPlaneGroupsAPI: mockGroups,
	})

	members, err := client.ListControlPlaneGroupMemberships(ctx, groupID)
	require.NoError(t, err)
	assert.Equal(t, []string{"member-1", "member-2"}, members)
	mockGroups.AssertExpectations(t)
}

func TestUpsertControlPlaneGroupMemberships(t *testing.T) {
	ctx := testContextWithLogger()
	mockGroups := &helpers.MockControlPlaneGroupsAPI{}

	mockGroups.On(
		"PutControlPlanesIDGroupMemberships",
		mock.Anything,
		"group-1",
		mock.MatchedBy(func(req *kkComps.GroupMembership) bool {
			require.NotNil(t, req)
			require.Len(t, req.Members, 2)
			assert.Equal(t, "cp-1", req.Members[0].ID)
			assert.Equal(t, "cp-2", req.Members[1].ID)
			return true
		}),
	).Return(&kkOps.PutControlPlanesIDGroupMembershipsResponse{}, nil).Once()

	client := NewClient(ClientConfig{
		ControlPlaneGroupsAPI: mockGroups,
	})

	err := client.UpsertControlPlaneGroupMemberships(ctx, "group-1", []string{"cp-1", "cp-2"})
	require.NoError(t, err)
	mockGroups.AssertExpectations(t)
}
