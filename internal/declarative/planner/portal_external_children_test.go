package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Minimal mocks for PortalPageAPI and PortalSnippetAPI
type mockPortalPageAPI struct{}

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
	// Return empty list
	return &kkOps.ListPortalPagesResponse{
		StatusCode: 200,
		ListPortalPagesResponse: &kkComps.ListPortalPagesResponse{
			Data: []kkComps.PortalPageInfo{},
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

// Ensure mocks satisfy interfaces
var (
	_ helpers.PortalPageAPI    = (*mockPortalPageAPI)(nil)
	_ helpers.PortalSnippetAPI = (*mockPortalSnippetAPI)(nil)
)

func TestPlanner_ExternalPortal_PlansChildren(t *testing.T) {
	ctx := context.Background()

	// Mock portal API to return an existing portal (external reference by name)
	mockPortalAPI := new(MockPortalAPI)
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.ListPortalsResponsePortal{
				{ID: "portal-123", Name: ptrString("ext-portal")},
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
				CreatePortal: kkComps.CreatePortal{Name: ptrString("ext-portal")},
				Ref:          "ext-portal-ref",
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
			if ref, ok := c.References["portal_id"]; ok {
				assert.Equal(t, "ext-portal-ref", ref.Ref)
			} else {
				t.Errorf("portal_id reference missing on portal_page change")
			}
		}
		if c.ResourceType == ResourceTypePortalSnippet && c.Action == ActionCreate {
			foundSnippet = true
			if ref, ok := c.References["portal_id"]; ok {
				assert.Equal(t, "ext-portal-ref", ref.Ref)
			} else {
				t.Errorf("portal_id reference missing on portal_snippet change")
			}
		}
	}

	assert.True(t, foundPage, "expected a portal_page create change")
	assert.True(t, foundSnippet, "expected a portal_snippet create change")
}
