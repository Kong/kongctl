package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Minimal mocks for PortalPageAPI and PortalSnippetAPI
type mockPortalPageAPI struct {
	listData []kkComps.PortalPageInfo
}

func (m *mockPortalPageAPI) CreatePortalPage(
	_ context.Context,
	_ string,
	_ kkComps.CreatePortalPageRequest,
	_ ...kkOps.Option,
) (*kkOps.CreatePortalPageResponse, error) {
	return nil, nil
}

func (m *mockPortalPageAPI) UpdatePortalPage(
	_ context.Context,
	_ kkOps.UpdatePortalPageRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdatePortalPageResponse, error) {
	return nil, nil
}

func (m *mockPortalPageAPI) DeletePortalPage(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DeletePortalPageResponse, error) {
	return nil, nil
}

func (m *mockPortalPageAPI) ListPortalPages(
	_ context.Context,
	_ kkOps.ListPortalPagesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListPortalPagesResponse, error) {
	data := m.listData
	if data == nil {
		data = []kkComps.PortalPageInfo{}
	}
	return &kkOps.ListPortalPagesResponse{
		StatusCode: 200,
		ListPortalPagesResponse: &kkComps.ListPortalPagesResponse{
			Data: data,
		},
	}, nil
}

func (m *mockPortalPageAPI) GetPortalPage(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.GetPortalPageResponse, error) {
	return &kkOps.GetPortalPageResponse{StatusCode: 404}, nil
}

type mockPortalSnippetAPI struct{}

func (m *mockPortalSnippetAPI) CreatePortalSnippet(
	_ context.Context,
	_ string,
	_ kkComps.CreatePortalSnippetRequest,
	_ ...kkOps.Option,
) (*kkOps.CreatePortalSnippetResponse, error) {
	return nil, nil
}

func (m *mockPortalSnippetAPI) UpdatePortalSnippet(
	_ context.Context,
	_ kkOps.UpdatePortalSnippetRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdatePortalSnippetResponse, error) {
	return nil, nil
}

func (m *mockPortalSnippetAPI) DeletePortalSnippet(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DeletePortalSnippetResponse, error) {
	return nil, nil
}

func (m *mockPortalSnippetAPI) ListPortalSnippets(
	_ context.Context,
	_ kkOps.ListPortalSnippetsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListPortalSnippetsResponse, error) {
	// Return empty list
	return &kkOps.ListPortalSnippetsResponse{
		StatusCode: 200,
		ListPortalSnippetsResponse: &kkComps.ListPortalSnippetsResponse{
			Data: []kkComps.PortalSnippetInfo{},
		},
	}, nil
}

func (m *mockPortalSnippetAPI) GetPortalSnippet(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.GetPortalSnippetResponse, error) {
	return &kkOps.GetPortalSnippetResponse{StatusCode: 404}, nil
}

type countingAPIPublicationAPI struct {
	t         *testing.T
	listCalls int
}

func (c *countingAPIPublicationAPI) PublishAPIToPortal(
	_ context.Context,
	_ kkOps.PublishAPIToPortalRequest,
	_ ...kkOps.Option,
) (*kkOps.PublishAPIToPortalResponse, error) {
	c.t.Fatalf("unexpected PublishAPIToPortal call during planning")
	return nil, nil
}

func (c *countingAPIPublicationAPI) DeletePublication(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DeletePublicationResponse, error) {
	c.t.Fatalf("unexpected DeletePublication call during planning")
	return nil, nil
}

func (c *countingAPIPublicationAPI) ListAPIPublications(
	_ context.Context,
	_ kkOps.ListAPIPublicationsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAPIPublicationsResponse, error) {
	c.listCalls++
	return &kkOps.ListAPIPublicationsResponse{
		ListAPIPublicationResponse: &kkComps.ListAPIPublicationResponse{
			Data: []kkComps.APIPublicationListItem{},
			Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 0}},
		},
	}, nil
}

type countingPortalIPAllowListAPI struct {
	t         *testing.T
	listCalls int
	failList  bool
	entries   []kkComps.IPEntry
}

func (c *countingPortalIPAllowListAPI) CreatePortalIPAllowList(
	_ context.Context,
	_ string,
	_ *kkComps.CreatePortalSourceIPRestriction,
	_ ...kkOps.Option,
) (*kkOps.CreatePortalIPAllowListResponse, error) {
	c.t.Fatalf("unexpected CreatePortalIPAllowList call during planning")
	return nil, nil
}

func (c *countingPortalIPAllowListAPI) ListPortalIPAllowList(
	_ context.Context,
	_ kkOps.ListPortalIPAllowListRequest,
	_ ...kkOps.Option,
) (*kkOps.ListPortalIPAllowListResponse, error) {
	c.listCalls++
	if c.failList {
		c.t.Fatalf("unexpected ListPortalIPAllowList call")
	}
	return &kkOps.ListPortalIPAllowListResponse{
		PortalSourceIPRestrictionPaginatedResponse: &kkComps.PortalSourceIPRestrictionPaginatedResponse{
			Meta: kkComps.CursorMetaPage{Size: 100},
			Data: c.entries,
		},
	}, nil
}

func (c *countingPortalIPAllowListAPI) PutPortalIPAllowList(
	_ context.Context,
	_ kkOps.PutPortalIPAllowListRequest,
	_ ...kkOps.Option,
) (*kkOps.PutPortalIPAllowListResponse, error) {
	c.t.Fatalf("unexpected PutPortalIPAllowList call during planning")
	return nil, nil
}

func (c *countingPortalIPAllowListAPI) UpdatePortalIPAllowList(
	_ context.Context,
	_ kkOps.UpdatePortalIPAllowListRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdatePortalIPAllowListResponse, error) {
	c.t.Fatalf("unexpected UpdatePortalIPAllowList call during planning")
	return nil, nil
}

func (c *countingPortalIPAllowListAPI) DeletePortalIPAllowList(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DeletePortalIPAllowListResponse, error) {
	c.t.Fatalf("unexpected DeletePortalIPAllowList call during planning")
	return nil, nil
}

// Ensure mocks satisfy interfaces
var (
	_ helpers.PortalPageAPI    = (*mockPortalPageAPI)(nil)
	_ helpers.PortalSnippetAPI = (*mockPortalSnippetAPI)(nil)
)

func TestPlanner_ExternalPortalReferenceOnlyPublicationDoesNotListIPAllowLists(t *testing.T) {
	ctx := context.Background()
	portalID := "portal-123"
	portalName := "shared.portal"
	apiID := "api-123"
	apiName := "Example API"
	visibility := kkComps.APIPublicationVisibility("private")

	mockPortalAPI := new(MockPortalAPI)
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.ListPortalsResponsePortal{
				newListPortal(portalID, portalName, nil),
			},
			Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 1}},
		},
		StatusCode: 200,
	}, nil)

	mockAPIAPI := new(MockAPIAPI)
	mockAPIAPI.On("ListApis", mock.Anything, mock.Anything).Return(&kkOps.ListApisResponse{
		ListAPIResponse: &kkComps.ListAPIResponse{
			Data: []kkComps.APIResponseSchema{
				{
					ID:   apiID,
					Name: apiName,
					Labels: map[string]string{
						labels.NamespaceKey: DefaultNamespace,
					},
				},
			},
			Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 1}},
		},
	}, nil)

	publicationsAPI := &countingAPIPublicationAPI{t: t}
	allowListAPI := &countingPortalIPAllowListAPI{t: t, failList: true}
	client := state.NewClient(state.ClientConfig{
		PortalAPI:            mockPortalAPI,
		APIAPI:               mockAPIAPI,
		APIPublicationAPI:    publicationsAPI,
		PortalIPAllowListAPI: allowListAPI,
	})

	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{Name: portalName},
				BaseResource: resources.BaseResource{
					Ref: "shared-portal",
				},
				External: &resources.ExternalBlock{
					Selector: &resources.ExternalSelector{MatchFields: map[string]string{FieldName: portalName}},
				},
			},
		},
		APIs: []resources.APIResource{
			{
				BaseResource: resources.BaseResource{Ref: "example-api"},
				CreateAPIRequest: kkComps.CreateAPIRequest{
					Name: apiName,
				},
			},
		},
		APIPublications: []resources.APIPublicationResource{
			{
				Ref:      "example-api-publication",
				API:      "example-api",
				PortalID: tags.RefPlaceholderPrefix + "shared-portal#id",
				APIPublication: kkComps.APIPublication{
					Visibility: &visibility,
				},
			},
		},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(ctx, rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)

	require.Equal(t, 1, publicationsAPI.listCalls, "expected API publications to be listed")
	require.Zero(t, allowListAPI.listCalls, "reference-only external portal must not list IP allow lists")
	requirePlanChange(t, plan, ResourceTypeAPIPublication, ActionCreate)
	mockPortalAPI.AssertExpectations(t)
	mockAPIAPI.AssertExpectations(t)
}

func TestPlanner_ExternalPortalExplicitIPAllowListStillListsIPAllowLists(t *testing.T) {
	ctx := context.Background()
	allowListAPI := &countingPortalIPAllowListAPI{t: t}
	plan := generateExternalPortalPlanWithIPAllowListAPI(ctx, t, PlanModeApply, allowListAPI, true)

	require.Equal(t, 1, allowListAPI.listCalls, "explicit external portal IP allow list must list current entries")
	requirePlanChange(t, plan, ResourceTypePortalIPAllowList, ActionCreate)
}

func TestPlanner_ExternalPortalSyncScopedIPAllowListStillListsIPAllowLists(t *testing.T) {
	ctx := context.Background()
	allowListAPI := &countingPortalIPAllowListAPI{t: t}
	plan := generateExternalPortalPlanWithIPAllowListAPI(ctx, t, PlanModeSync, allowListAPI, false)

	require.Equal(t, 1, allowListAPI.listCalls, "explicit sync scope must list current IP allow-list entries")
	require.Empty(t, plan.Changes)
}

func generateExternalPortalPlanWithIPAllowListAPI(
	ctx context.Context,
	t *testing.T,
	mode PlanMode,
	allowListAPI *countingPortalIPAllowListAPI,
	includeDesiredAllowList bool,
) *Plan {
	t.Helper()

	portalID := "portal-123"
	portalName := "shared.portal"
	mockPortalAPI := new(MockPortalAPI)
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.ListPortalsResponsePortal{
				newListPortal(portalID, portalName, nil),
			},
			Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 1}},
		},
		StatusCode: 200,
	}, nil)

	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{Name: portalName},
				BaseResource: resources.BaseResource{
					Ref: "shared-portal",
				},
				External: &resources.ExternalBlock{
					Selector: &resources.ExternalSelector{MatchFields: map[string]string{FieldName: portalName}},
				},
			},
		},
	}
	if includeDesiredAllowList {
		rs.PortalIPAllowLists = []resources.PortalIPAllowListResource{
			{
				Ref:        "shared-portal-ip-allow-list",
				Portal:     "shared-portal",
				AllowedIPs: []string{"192.0.2.10"},
			},
		}
	}
	if mode == PlanModeSync {
		scope := resources.NewSyncScope()
		scope.AddRoot(resources.ResourceTypePortal)
		scope.AddChild(resources.ResourceTypePortal, "shared-portal", resources.ResourceTypePortalIPAllowList)
		rs.SyncScope = scope
	}

	client := state.NewClient(state.ClientConfig{
		PortalAPI:            mockPortalAPI,
		PortalIPAllowListAPI: allowListAPI,
	})
	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(ctx, rs, Options{Mode: mode})
	require.NoError(t, err)
	mockPortalAPI.AssertExpectations(t)
	return plan
}

func requirePlanChange(t *testing.T, plan *Plan, resourceType string, action ActionType) {
	t.Helper()

	for _, change := range plan.Changes {
		if change.ResourceType == resourceType && change.Action == action {
			return
		}
	}
	require.Failf(t, "missing planned change", "expected %s %s in %+v", action, resourceType, plan.Changes)
}

func TestPlanner_ExternalPortal_PlansChildren(t *testing.T) {
	ctx := context.Background()

	// Mock portal API to return an existing portal (external reference by name)
	mockPortalAPI := new(MockPortalAPI)
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.ListPortalsResponsePortal{
				newListPortal("portal-123", "ext-portal", nil),
			},
			Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 1}},
		},
		StatusCode: 200,
	}, nil)

	client := state.NewClient(state.ClientConfig{
		PortalAPI:        mockPortalAPI,
		PortalPageAPI:    &mockPortalPageAPI{},
		PortalSnippetAPI: &mockPortalSnippetAPI{},
	})

	planner := NewPlanner(client, slog.Default())

	// Desired resources: external portal + one page + one snippet
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{Name: "ext-portal"},
				BaseResource: resources.BaseResource{
					Ref: "ext-portal-ref",
				},
				External: &resources.ExternalBlock{
					Selector: &resources.ExternalSelector{MatchFields: map[string]string{"name": "ext-portal"}},
				},
			},
		},
		PortalPages: []resources.PortalPageResource{
			{
				CreatePortalPageRequest: kkComps.CreatePortalPageRequest{
					Slug:    "home",
					Content: "Hello",
				},
				Ref:    "page-home",
				Portal: "ext-portal-ref",
			},
		},
		PortalSnippets: []resources.PortalSnippetResource{
			{
				Ref:     "snippet-1",
				Portal:  "ext-portal-ref",
				Name:    "welcome",
				Content: "Hello World",
			},
		},
	}

	plan, err := planner.GeneratePlan(ctx, rs, Options{Mode: PlanModeApply})
	assert.NoError(t, err)
	assert.NotNil(t, plan)

	// Expect page and snippet CREATE changes present
	var foundPage, foundSnippet bool
	for _, c := range plan.Changes {
		if c.ResourceType == ResourceTypePortalPage && c.Action == ActionCreate {
			foundPage = true
			// Reference should include resolved portal ID
			if ref, ok := c.References["portal_id"]; ok {
				assert.Equal(t, "portal-123", ref.ID)
			} else {
				t.Errorf("portal_id reference missing on portal_page change")
			}
		}
		if c.ResourceType == ResourceTypePortalSnippet && c.Action == ActionCreate {
			foundSnippet = true
			// Reference should include resolved portal ID
			if ref, ok := c.References["portal_id"]; ok {
				// Snippet create uses helper reference; ID should be present when known
				assert.Equal(t, "portal-123", ref.ID)
			} else {
				t.Errorf("portal_id reference missing on portal_snippet change")
			}
		}
	}

	assert.True(t, foundPage, "expected a portal_page create change")
	assert.True(t, foundSnippet, "expected a portal_snippet create change")
}

func TestPlanner_ExternalPortal_SyncDoesNotDeleteExistingPages(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	mockPortalAPI := new(MockPortalAPI)
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.ListPortalsResponsePortal{
				newListPortal("portal-123", "ext-portal", nil),
			},
			Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 1}},
		},
		StatusCode: 200,
	}, nil)

	client := state.NewClient(state.ClientConfig{
		PortalAPI: mockPortalAPI,
		PortalPageAPI: &mockPortalPageAPI{
			listData: []kkComps.PortalPageInfo{
				{ID: "page-root", Slug: "/"},
				{ID: "page-guides", Slug: "guides"},
			},
		},
	})

	planner := NewPlanner(client, slog.Default())

	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{Name: "ext-portal"},
				BaseResource: resources.BaseResource{
					Ref: "ext-portal-ref",
				},
				External: &resources.ExternalBlock{
					Selector: &resources.ExternalSelector{MatchFields: map[string]string{"name": "ext-portal"}},
				},
			},
		},
	}

	plan, err := planner.GeneratePlan(ctx, rs, Options{Mode: PlanModeSync})
	assert.NoError(t, err)
	assert.NotNil(t, plan)

	for _, change := range plan.Changes {
		if change.ResourceType == ResourceTypePortalPage && change.Action == ActionDelete {
			t.Fatalf("unexpected portal page delete planned for external portal: %+v", change)
		}
	}
}
