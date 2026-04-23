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

type mockAPIVersionAPI struct {
	listAPIVersionsFunc func(
		context.Context, kkOps.ListAPIVersionsRequest,
	) (*kkOps.ListAPIVersionsResponse, error)
}

func (m *mockAPIVersionAPI) CreateAPIVersion(
	context.Context,
	string,
	kkComps.CreateAPIVersionRequest,
	...kkOps.Option,
) (*kkOps.CreateAPIVersionResponse, error) {
	return nil, fmt.Errorf("CreateAPIVersion not implemented")
}

func (m *mockAPIVersionAPI) ListAPIVersions(
	ctx context.Context,
	request kkOps.ListAPIVersionsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAPIVersionsResponse, error) {
	if m.listAPIVersionsFunc != nil {
		return m.listAPIVersionsFunc(ctx, request)
	}
	return nil, fmt.Errorf("ListAPIVersions not implemented")
}

func (m *mockAPIVersionAPI) UpdateAPIVersion(
	context.Context,
	kkOps.UpdateAPIVersionRequest,
	...kkOps.Option,
) (*kkOps.UpdateAPIVersionResponse, error) {
	return nil, fmt.Errorf("UpdateAPIVersion not implemented")
}

func (m *mockAPIVersionAPI) DeleteAPIVersion(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAPIVersionResponse, error) {
	return nil, fmt.Errorf("DeleteAPIVersion not implemented")
}

func (m *mockAPIVersionAPI) FetchAPIVersion(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.FetchAPIVersionResponse, error) {
	return nil, fmt.Errorf("FetchAPIVersion not implemented")
}

type mockAPIPublicationAPI struct {
	listAPIPublicationsFunc func(
		context.Context, kkOps.ListAPIPublicationsRequest,
	) (*kkOps.ListAPIPublicationsResponse, error)
}

func (m *mockAPIPublicationAPI) PublishAPIToPortal(
	context.Context,
	kkOps.PublishAPIToPortalRequest,
	...kkOps.Option,
) (*kkOps.PublishAPIToPortalResponse, error) {
	return nil, fmt.Errorf("PublishAPIToPortal not implemented")
}

func (m *mockAPIPublicationAPI) DeletePublication(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeletePublicationResponse, error) {
	return nil, fmt.Errorf("DeletePublication not implemented")
}

func (m *mockAPIPublicationAPI) ListAPIPublications(
	ctx context.Context,
	request kkOps.ListAPIPublicationsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAPIPublicationsResponse, error) {
	if m.listAPIPublicationsFunc != nil {
		return m.listAPIPublicationsFunc(ctx, request)
	}
	return nil, fmt.Errorf("ListAPIPublications not implemented")
}

type mockAPIImplementationAPI struct {
	listAPIImplementationsFunc func(
		context.Context, kkOps.ListAPIImplementationsRequest,
	) (*kkOps.ListAPIImplementationsResponse, error)
}

func (m *mockAPIImplementationAPI) ListAPIImplementations(
	ctx context.Context,
	request kkOps.ListAPIImplementationsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAPIImplementationsResponse, error) {
	if m.listAPIImplementationsFunc != nil {
		return m.listAPIImplementationsFunc(ctx, request)
	}
	return nil, fmt.Errorf("ListAPIImplementations not implemented")
}

func (m *mockAPIImplementationAPI) CreateAPIImplementation(
	context.Context,
	string,
	kkComps.APIImplementation,
	...kkOps.Option,
) (*kkOps.CreateAPIImplementationResponse, error) {
	return nil, fmt.Errorf("CreateAPIImplementation not implemented")
}

func (m *mockAPIImplementationAPI) DeleteAPIImplementation(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAPIImplementationResponse, error) {
	return nil, fmt.Errorf("DeleteAPIImplementation not implemented")
}

type mockPortalSnippetAPI struct {
	listPortalSnippetsFunc func(
		context.Context, kkOps.ListPortalSnippetsRequest,
	) (*kkOps.ListPortalSnippetsResponse, error)
}

func (m *mockPortalSnippetAPI) CreatePortalSnippet(
	context.Context,
	string,
	kkComps.CreatePortalSnippetRequest,
	...kkOps.Option,
) (*kkOps.CreatePortalSnippetResponse, error) {
	return nil, fmt.Errorf("CreatePortalSnippet not implemented")
}

func (m *mockPortalSnippetAPI) UpdatePortalSnippet(
	context.Context,
	kkOps.UpdatePortalSnippetRequest,
	...kkOps.Option,
) (*kkOps.UpdatePortalSnippetResponse, error) {
	return nil, fmt.Errorf("UpdatePortalSnippet not implemented")
}

func (m *mockPortalSnippetAPI) DeletePortalSnippet(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeletePortalSnippetResponse, error) {
	return nil, fmt.Errorf("DeletePortalSnippet not implemented")
}

func (m *mockPortalSnippetAPI) ListPortalSnippets(
	ctx context.Context,
	request kkOps.ListPortalSnippetsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListPortalSnippetsResponse, error) {
	if m.listPortalSnippetsFunc != nil {
		return m.listPortalSnippetsFunc(ctx, request)
	}
	return nil, fmt.Errorf("ListPortalSnippets not implemented")
}

func (m *mockPortalSnippetAPI) GetPortalSnippet(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetPortalSnippetResponse, error) {
	return nil, fmt.Errorf("GetPortalSnippet not implemented")
}

type mockPortalTeamAPI struct {
	listPortalTeamsFunc func(
		context.Context, kkOps.ListPortalTeamsRequest,
	) (*kkOps.ListPortalTeamsResponse, error)
}

func (m *mockPortalTeamAPI) ListPortalTeams(
	ctx context.Context,
	request kkOps.ListPortalTeamsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListPortalTeamsResponse, error) {
	if m.listPortalTeamsFunc != nil {
		return m.listPortalTeamsFunc(ctx, request)
	}
	return nil, fmt.Errorf("ListPortalTeams not implemented")
}

func (m *mockPortalTeamAPI) GetPortalTeam(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetPortalTeamResponse, error) {
	return nil, fmt.Errorf("GetPortalTeam not implemented")
}

func (m *mockPortalTeamAPI) CreatePortalTeam(
	context.Context,
	string,
	*kkComps.PortalCreateTeamRequest,
	...kkOps.Option,
) (*kkOps.CreatePortalTeamResponse, error) {
	return nil, fmt.Errorf("CreatePortalTeam not implemented")
}

func (m *mockPortalTeamAPI) UpdatePortalTeam(
	context.Context,
	kkOps.UpdatePortalTeamRequest,
	...kkOps.Option,
) (*kkOps.UpdatePortalTeamResponse, error) {
	return nil, fmt.Errorf("UpdatePortalTeam not implemented")
}

func (m *mockPortalTeamAPI) DeletePortalTeam(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeletePortalTeamResponse, error) {
	return nil, fmt.Errorf("DeletePortalTeam not implemented")
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

func TestListAPIVersions_ExactPageBoundaryRequestsSecondPage(t *testing.T) {
	ctx := testContextWithLogger()
	var requestedPages []int64

	client := NewClient(ClientConfig{
		APIVersionAPI: &mockAPIVersionAPI{
			listAPIVersionsFunc: func(
				_ context.Context, req kkOps.ListAPIVersionsRequest,
			) (*kkOps.ListAPIVersionsResponse, error) {
				require.NotNil(t, req.PageNumber)
				requestedPages = append(requestedPages, *req.PageNumber)

				switch *req.PageNumber {
				case 1:
					return newListAPIVersionsResponse(1, 100, 200), nil
				case 2:
					return newListAPIVersionsResponse(2, 100, 200), nil
				default:
					return nil, fmt.Errorf("unexpected page request: %d", *req.PageNumber)
				}
			},
		},
	})

	versions, err := client.ListAPIVersions(ctx, "api-1")
	require.NoError(t, err)
	require.Len(t, versions, 200)
	assert.Equal(t, []int64{1, 2}, requestedPages)
	assert.Equal(t, "version-1-1", versions[0].ID)
	assert.Equal(t, "2.100.0", versions[199].Version)
}

func TestListAPIPublications_ExactPageBoundaryRequestsSecondPage(t *testing.T) {
	ctx := testContextWithLogger()
	var requestedPages []int64

	client := NewClient(ClientConfig{
		APIPublicationAPI: &mockAPIPublicationAPI{
			listAPIPublicationsFunc: func(
				_ context.Context, req kkOps.ListAPIPublicationsRequest,
			) (*kkOps.ListAPIPublicationsResponse, error) {
				require.NotNil(t, req.PageNumber)
				requestedPages = append(requestedPages, *req.PageNumber)

				switch *req.PageNumber {
				case 1:
					return newListAPIPublicationsResponse(1, 100, 200), nil
				case 2:
					return newListAPIPublicationsResponse(2, 100, 200), nil
				default:
					return nil, fmt.Errorf("unexpected page request: %d", *req.PageNumber)
				}
			},
		},
	})

	publications, err := client.ListAPIPublications(ctx, "api-1")
	require.NoError(t, err)
	require.Len(t, publications, 200)
	assert.Equal(t, []int64{1, 2}, requestedPages)
	assert.Equal(t, "portal-1-1", publications[0].PortalID)
	assert.Equal(t, "portal-2-100", publications[199].PortalID)
}

func TestListAPIImplementations_ExactPageBoundaryRequestsSecondPage(t *testing.T) {
	ctx := testContextWithLogger()
	var requestedPages []int64

	client := NewClient(ClientConfig{
		APIImplementationAPI: &mockAPIImplementationAPI{
			listAPIImplementationsFunc: func(
				_ context.Context, req kkOps.ListAPIImplementationsRequest,
			) (*kkOps.ListAPIImplementationsResponse, error) {
				require.NotNil(t, req.PageNumber)
				requestedPages = append(requestedPages, *req.PageNumber)

				switch *req.PageNumber {
				case 1:
					return newListAPIImplementationsResponse(1, 100, 200), nil
				case 2:
					return newListAPIImplementationsResponse(2, 100, 200), nil
				default:
					return nil, fmt.Errorf("unexpected page request: %d", *req.PageNumber)
				}
			},
		},
	})

	implementations, err := client.ListAPIImplementations(ctx, "api-1")
	require.NoError(t, err)
	require.Len(t, implementations, 200)
	assert.Equal(t, []int64{1, 2}, requestedPages)
	assert.Equal(t, "impl-1-1", implementations[0].ID)
	require.NotNil(t, implementations[199].Service)
	assert.Equal(t, "service-2-100", implementations[199].Service.ID)
}

func TestListPortalSnippets_ExactPageBoundaryRequestsSecondPage(t *testing.T) {
	ctx := testContextWithLogger()
	var requestedPages []int64

	client := NewClient(ClientConfig{
		PortalSnippetAPI: &mockPortalSnippetAPI{
			listPortalSnippetsFunc: func(
				_ context.Context, req kkOps.ListPortalSnippetsRequest,
			) (*kkOps.ListPortalSnippetsResponse, error) {
				require.NotNil(t, req.PageNumber)
				requestedPages = append(requestedPages, *req.PageNumber)

				switch *req.PageNumber {
				case 1:
					return newListPortalSnippetsResponse(1, 100, 200), nil
				case 2:
					return newListPortalSnippetsResponse(2, 100, 200), nil
				default:
					return nil, fmt.Errorf("unexpected page request: %d", *req.PageNumber)
				}
			},
		},
	})

	snippets, err := client.ListPortalSnippets(ctx, "portal-1")
	require.NoError(t, err)
	require.Len(t, snippets, 200)
	assert.Equal(t, []int64{1, 2}, requestedPages)
	assert.Equal(t, "snippet-1-1", snippets[0].ID)
	assert.Equal(t, "snippet-2-100", snippets[199].Name)
}

func TestListPortalTeams_ExactPageBoundaryRequestsSecondPage(t *testing.T) {
	ctx := testContextWithLogger()
	var requestedPages []int64

	client := NewClient(ClientConfig{
		PortalTeamAPI: &mockPortalTeamAPI{
			listPortalTeamsFunc: func(
				_ context.Context, req kkOps.ListPortalTeamsRequest,
			) (*kkOps.ListPortalTeamsResponse, error) {
				require.NotNil(t, req.PageNumber)
				requestedPages = append(requestedPages, *req.PageNumber)

				switch *req.PageNumber {
				case 1:
					return newListPortalTeamsResponse(1, 100, 200), nil
				case 2:
					return newListPortalTeamsResponse(2, 100, 200), nil
				default:
					return nil, fmt.Errorf("unexpected page request: %d", *req.PageNumber)
				}
			},
		},
	})

	teams, err := client.ListPortalTeams(ctx, "portal-1")
	require.NoError(t, err)
	require.Len(t, teams, 200)
	assert.Equal(t, []int64{1, 2}, requestedPages)
	assert.Equal(t, "team-1-1", teams[0].ID)
	assert.Equal(t, "team-2-100", teams[199].Name)
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

func newListAPIVersionsResponse(pageNumber, count int, total float64) *kkOps.ListAPIVersionsResponse {
	data := make([]kkComps.ListAPIVersionResponseAPIVersionSummary, count)
	for i := range count {
		data[i] = kkComps.ListAPIVersionResponseAPIVersionSummary{
			ID:      fmt.Sprintf("version-%d-%d", pageNumber, i+1),
			Version: fmt.Sprintf("%d.%d.0", pageNumber, i+1),
		}
	}

	return &kkOps.ListAPIVersionsResponse{
		ListAPIVersionResponse: &kkComps.ListAPIVersionResponse{
			Data: data,
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: total},
			},
		},
	}
}

func newListAPIPublicationsResponse(pageNumber, count int, total float64) *kkOps.ListAPIPublicationsResponse {
	visibility := kkComps.APIPublicationVisibility("public")
	data := make([]kkComps.APIPublicationListItem, count)
	for i := range count {
		data[i] = kkComps.APIPublicationListItem{
			APIID:      "api-1",
			PortalID:   fmt.Sprintf("portal-%d-%d", pageNumber, i+1),
			Visibility: &visibility,
		}
	}

	return &kkOps.ListAPIPublicationsResponse{
		ListAPIPublicationResponse: &kkComps.ListAPIPublicationResponse{
			Data: data,
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: total},
			},
		},
	}
}

func newListAPIImplementationsResponse(
	pageNumber, count int,
	total float64,
) *kkOps.ListAPIImplementationsResponse {
	data := make([]kkComps.APIImplementationListItem, count)
	for i := range count {
		data[i] = kkComps.APIImplementationListItem{
			APIImplementationListItemGatewayServiceEntity: &kkComps.APIImplementationListItemGatewayServiceEntity{
				ID:    fmt.Sprintf("impl-%d-%d", pageNumber, i+1),
				APIID: "api-1",
				Service: &kkComps.APIImplementationService{
					ID:             fmt.Sprintf("service-%d-%d", pageNumber, i+1),
					ControlPlaneID: fmt.Sprintf("cp-%d", pageNumber),
				},
			},
		}
	}

	return &kkOps.ListAPIImplementationsResponse{
		ListAPIImplementationsResponse: &kkComps.ListAPIImplementationsResponse{
			Data: data,
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: total},
			},
		},
	}
}

func newListPortalSnippetsResponse(pageNumber, count int, total float64) *kkOps.ListPortalSnippetsResponse {
	data := make([]kkComps.PortalSnippetInfo, count)
	for i := range count {
		data[i] = kkComps.PortalSnippetInfo{
			ID:         fmt.Sprintf("snippet-%d-%d", pageNumber, i+1),
			Name:       fmt.Sprintf("snippet-%d-%d", pageNumber, i+1),
			Title:      fmt.Sprintf("Snippet %d-%d", pageNumber, i+1),
			Visibility: kkComps.VisibilityStatus("public"),
			Status:     kkComps.PublishedStatus("published"),
		}
	}

	return &kkOps.ListPortalSnippetsResponse{
		ListPortalSnippetsResponse: &kkComps.ListPortalSnippetsResponse{
			Data: data,
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: total},
			},
		},
	}
}

func newListPortalTeamsResponse(pageNumber, count int, total float64) *kkOps.ListPortalTeamsResponse {
	data := make([]kkComps.PortalTeamResponse, count)
	for i := range count {
		data[i] = kkComps.PortalTeamResponse{
			ID:                 stringPtr(fmt.Sprintf("team-%d-%d", pageNumber, i+1)),
			Name:               stringPtr(fmt.Sprintf("team-%d-%d", pageNumber, i+1)),
			CanOwnApplications: boolPtr(true),
		}
	}

	return &kkOps.ListPortalTeamsResponse{
		ListPortalTeamsResponse: &kkComps.ListPortalTeamsResponse{
			Data: data,
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: total},
			},
		},
	}
}

func stringPtr(v string) *string {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}
