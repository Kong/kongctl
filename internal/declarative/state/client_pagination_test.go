package state

import (
	"context"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockPortalTeamRolesAPI struct {
	listPortalTeamRolesFunc func(
		context.Context, kkOps.ListPortalTeamRolesRequest,
	) (*kkOps.ListPortalTeamRolesResponse, error)
}

func (m *mockPortalTeamRolesAPI) ListPortalTeamRoles(
	ctx context.Context,
	request kkOps.ListPortalTeamRolesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListPortalTeamRolesResponse, error) {
	if m.listPortalTeamRolesFunc != nil {
		return m.listPortalTeamRolesFunc(ctx, request)
	}
	return nil, fmt.Errorf("ListPortalTeamRoles not implemented")
}

func (m *mockPortalTeamRolesAPI) AssignRoleToPortalTeams(
	context.Context,
	kkOps.AssignRoleToPortalTeamsRequest,
	...kkOps.Option,
) (*kkOps.AssignRoleToPortalTeamsResponse, error) {
	return nil, fmt.Errorf("AssignRoleToPortalTeams not implemented")
}

func (m *mockPortalTeamRolesAPI) RemoveRoleFromPortalTeam(
	context.Context,
	kkOps.RemoveRoleFromPortalTeamRequest,
	...kkOps.Option,
) (*kkOps.RemoveRoleFromPortalTeamResponse, error) {
	return nil, fmt.Errorf("RemoveRoleFromPortalTeam not implemented")
}

func TestListManagedPortals_ContinuesPastFilteredEmptyPage(t *testing.T) {
	ctx := testContextWithLogger()
	var requestedPages []int64

	client := NewClient(ClientConfig{
		PortalAPI: &mockPortalAPI{
			listPortalsFunc: func(
				_ context.Context, req kkOps.ListPortalsRequest,
			) (*kkOps.ListPortalsResponse, error) {
				require.NotNil(t, req.PageNumber)
				requestedPages = append(requestedPages, *req.PageNumber)

				switch *req.PageNumber {
				case 1:
					return &kkOps.ListPortalsResponse{
						ListPortalsResponse: &kkComps.ListPortalsResponse{
							Data: []kkComps.ListPortalsResponsePortal{
								newListPortal(
									"portal-unmanaged",
									"Unmanaged Portal",
									map[string]string{"env": "dev"},
								),
							},
							Meta: kkComps.PaginatedMeta{
								Page: kkComps.PageMeta{Total: 200},
							},
						},
					}, nil
				case 2:
					return &kkOps.ListPortalsResponse{
						ListPortalsResponse: &kkComps.ListPortalsResponse{
							Data: []kkComps.ListPortalsResponsePortal{
								newListPortal(
									"portal-managed",
									"Managed Portal",
									map[string]string{labels.NamespaceKey: "team-a"},
								),
							},
							Meta: kkComps.PaginatedMeta{
								Page: kkComps.PageMeta{Total: 200},
							},
						},
					}, nil
				default:
					return nil, fmt.Errorf("unexpected page request: %d", *req.PageNumber)
				}
			},
		},
	})

	portals, err := client.ListManagedPortals(ctx, []string{"team-a"})
	require.NoError(t, err)
	require.Len(t, portals, 1)
	assert.Equal(t, "portal-managed", portals[0].ID)
	assert.Equal(t, []int64{1, 2}, requestedPages)
}

func TestListManagedControlPlanes_ContinuesPastFilteredEmptyPage(t *testing.T) {
	ctx := testContextWithLogger()
	mockAPI := helpers.NewMockControlPlaneAPI(t)

	unmanaged := kkComps.ControlPlane{
		ID:   "cp-unmanaged",
		Name: "unmanaged",
		Labels: map[string]string{
			"env": "dev",
		},
	}

	managed := kkComps.ControlPlane{
		ID:   "cp-managed",
		Name: "managed",
		Labels: map[string]string{
			labels.NamespaceKey: "team-a",
		},
	}

	mockAPI.EXPECT().
		ListControlPlanes(mock.Anything, mock.MatchedBy(func(req kkOps.ListControlPlanesRequest) bool {
			return req.PageNumber != nil && *req.PageNumber == 1
		})).
		Return(newListControlPlanesResponse([]kkComps.ControlPlane{unmanaged}, 200), nil).
		Once()

	mockAPI.EXPECT().
		ListControlPlanes(mock.Anything, mock.MatchedBy(func(req kkOps.ListControlPlanesRequest) bool {
			return req.PageNumber != nil && *req.PageNumber == 2
		})).
		Return(newListControlPlanesResponse([]kkComps.ControlPlane{managed}, 200), nil).
		Once()

	client := NewClient(ClientConfig{ControlPlaneAPI: mockAPI})

	result, err := client.ListManagedControlPlanes(ctx, []string{"team-a"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "cp-managed", result[0].ID)
	assert.Equal(t, "team-a", result[0].NormalizedLabels[labels.NamespaceKey])
}

func TestListPortalTeamRoles_ExactPageBoundaryDoesNotOverFetch(t *testing.T) {
	ctx := testContextWithLogger()
	var requestedPages []int64

	client := NewClient(ClientConfig{
		PortalTeamRolesAPI: &mockPortalTeamRolesAPI{
			listPortalTeamRolesFunc: func(
				_ context.Context, req kkOps.ListPortalTeamRolesRequest,
			) (*kkOps.ListPortalTeamRolesResponse, error) {
				require.NotNil(t, req.PageNumber)
				requestedPages = append(requestedPages, *req.PageNumber)

				switch *req.PageNumber {
				case 1:
					return newListPortalTeamRolesResponse(1, 100, 200), nil
				case 2:
					return newListPortalTeamRolesResponse(2, 100, 200), nil
				default:
					return nil, fmt.Errorf("unexpected page request: %d", *req.PageNumber)
				}
			},
		},
	})

	roles, err := client.ListPortalTeamRoles(ctx, "portal-1", "team-1")
	require.NoError(t, err)
	require.Len(t, roles, 200)
	assert.Equal(t, []int64{1, 2}, requestedPages)
	assert.Equal(t, "team-1", roles[0].TeamID)
	assert.Equal(t, "portal-1", roles[0].PortalID)
	assert.Equal(t, "role-2-100", roles[199].ID)
}

func newListPortalTeamRolesResponse(
	pageNumber int,
	count int,
	total float64,
) *kkOps.ListPortalTeamRolesResponse {
	data := make([]kkComps.PortalAssignedRoleResponse, count)
	for i := range count {
		data[i] = kkComps.PortalAssignedRoleResponse{
			ID:             fmt.Sprintf("role-%d-%d", pageNumber, i+1),
			RoleName:       fmt.Sprintf("Role %d-%d", pageNumber, i+1),
			EntityID:       fmt.Sprintf("entity-%d-%d", pageNumber, i+1),
			EntityTypeName: "api",
			EntityRegion:   kkComps.EntityRegionUs,
		}
	}

	return &kkOps.ListPortalTeamRolesResponse{
		AssignedPortalRoleCollectionResponse: &kkComps.AssignedPortalRoleCollectionResponse{
			Data: data,
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: total},
			},
		},
	}
}
