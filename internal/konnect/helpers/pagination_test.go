package helpers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAPIVersionAPI struct {
	listAPIVersionsFunc func(context.Context, kkOps.ListAPIVersionsRequest) (*kkOps.ListAPIVersionsResponse, error)
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

func TestPaginateAllPageNumber_DoesNotStopOnEmptyPageWhenTotalIndicatesMore(t *testing.T) {
	var requestedPages []int64

	items, err := paginateAllPageNumber(func(_ int64, pageNumber int64) ([]string, float64, error) {
		requestedPages = append(requestedPages, pageNumber)

		switch pageNumber {
		case 1:
			return []string{}, 200, nil
		case 2:
			return []string{"item-2"}, 200, nil
		default:
			return nil, 0, fmt.Errorf("unexpected page request: %d", pageNumber)
		}
	})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, []string{"item-2"}, items)
	assert.Equal(t, []int64{1, 2}, requestedPages)
}

func TestPaginateAllPageNumber_ReturnsExplicitErrorWhenPageLimitExceeded(t *testing.T) {
	sentinel := errors.New("requested page beyond test sentinel")

	items, err := paginateAllPageNumber(func(_ int64, pageNumber int64) ([]string, float64, error) {
		if pageNumber > 10000 {
			return nil, 0, sentinel
		}

		return nil, 1_000_000_000, nil
	})
	if err == nil {
		t.Fatal("Expected pagination limit error, got nil")
	}

	if errors.Is(err, sentinel) {
		t.Fatalf("Expected explicit pagination limit error before sentinel, got: %v", err)
	}

	if !strings.Contains(err.Error(), "pagination") {
		t.Fatalf("Expected pagination-related error, got: %v", err)
	}

	if items != nil {
		t.Fatalf("Expected nil result on pagination limit error, got: %v", items)
	}
}

func TestGetVersionsForAPI_PaginatesAcrossPages(t *testing.T) {
	var requestedPages []int64

	client := &mockAPIVersionAPI{
		listAPIVersionsFunc: func(
			_ context.Context, req kkOps.ListAPIVersionsRequest,
		) (*kkOps.ListAPIVersionsResponse, error) {
			pageNumber := int64(1)
			if req.PageNumber != nil {
				pageNumber = *req.PageNumber
			}
			requestedPages = append(requestedPages, pageNumber)

			switch pageNumber {
			case 1:
				return &kkOps.ListAPIVersionsResponse{
					ListAPIVersionResponse: &kkComps.ListAPIVersionResponse{
						Data: []kkComps.ListAPIVersionResponseAPIVersionSummary{
							{ID: "version-1", Version: "1.0.0"},
						},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 200},
						},
					},
				}, nil
			case 2:
				return &kkOps.ListAPIVersionsResponse{
					ListAPIVersionResponse: &kkComps.ListAPIVersionResponse{
						Data: []kkComps.ListAPIVersionResponseAPIVersionSummary{
							{ID: "version-2", Version: "2.0.0"},
						},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 200},
						},
					},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected page request: %d", pageNumber)
			}
		},
	}

	versions, err := GetVersionsForAPI(t.Context(), client, "api-1")
	require.NoError(t, err)
	require.Len(t, versions, 2)
	assert.Equal(t, []int64{1, 2}, requestedPages)

	secondVersion, ok := versions[1].(kkComps.ListAPIVersionResponseAPIVersionSummary)
	require.True(t, ok)
	assert.Equal(t, "version-2", secondVersion.ID)
}

func TestGetPublicationsForAPI_PaginatesAcrossPages(t *testing.T) {
	var requestedPages []int64

	client := &mockAPIPublicationAPI{
		listAPIPublicationsFunc: func(
			_ context.Context, req kkOps.ListAPIPublicationsRequest,
		) (*kkOps.ListAPIPublicationsResponse, error) {
			pageNumber := int64(1)
			if req.PageNumber != nil {
				pageNumber = *req.PageNumber
			}
			requestedPages = append(requestedPages, pageNumber)

			require.NotNil(t, req.Filter)
			require.NotNil(t, req.Filter.APIID)
			require.NotNil(t, req.Filter.APIID.Eq)
			assert.Equal(t, "api-1", *req.Filter.APIID.Eq)

			switch pageNumber {
			case 1:
				return &kkOps.ListAPIPublicationsResponse{
					ListAPIPublicationResponse: &kkComps.ListAPIPublicationResponse{
						Data: []kkComps.APIPublicationListItem{
							{APIID: "api-1", PortalID: "portal-1"},
						},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 200},
						},
					},
				}, nil
			case 2:
				return &kkOps.ListAPIPublicationsResponse{
					ListAPIPublicationResponse: &kkComps.ListAPIPublicationResponse{
						Data: []kkComps.APIPublicationListItem{
							{APIID: "api-1", PortalID: "portal-2"},
						},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 200},
						},
					},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected page request: %d", pageNumber)
			}
		},
	}

	publications, err := GetPublicationsForAPI(t.Context(), client, "api-1")
	require.NoError(t, err)
	require.Len(t, publications, 2)
	assert.Equal(t, []int64{1, 2}, requestedPages)

	secondPublication, ok := publications[1].(kkComps.APIPublicationListItem)
	require.True(t, ok)
	assert.Equal(t, "portal-2", secondPublication.PortalID)
}

func TestGetImplementationsForAPI_PaginatesAcrossPages(t *testing.T) {
	var requestedPages []int64

	client := &mockAPIImplementationAPI{
		listAPIImplementationsFunc: func(
			_ context.Context, req kkOps.ListAPIImplementationsRequest,
		) (*kkOps.ListAPIImplementationsResponse, error) {
			pageNumber := int64(1)
			if req.PageNumber != nil {
				pageNumber = *req.PageNumber
			}
			requestedPages = append(requestedPages, pageNumber)

			require.NotNil(t, req.Filter)
			require.NotNil(t, req.Filter.APIID)
			require.NotNil(t, req.Filter.APIID.Eq)
			assert.Equal(t, "api-1", *req.Filter.APIID.Eq)

			switch pageNumber {
			case 1:
				return &kkOps.ListAPIImplementationsResponse{
					ListAPIImplementationsResponse: &kkComps.ListAPIImplementationsResponse{
						Data: []kkComps.APIImplementationListItem{
							kkComps.CreateAPIImplementationListItemAPIImplementationListItemGatewayServiceEntity(
								kkComps.APIImplementationListItemGatewayServiceEntity{ID: "impl-1", APIID: "api-1"},
							),
						},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 200},
						},
					},
				}, nil
			case 2:
				return &kkOps.ListAPIImplementationsResponse{
					ListAPIImplementationsResponse: &kkComps.ListAPIImplementationsResponse{
						Data: []kkComps.APIImplementationListItem{
							kkComps.CreateAPIImplementationListItemAPIImplementationListItemGatewayServiceEntity(
								kkComps.APIImplementationListItemGatewayServiceEntity{ID: "impl-2", APIID: "api-1"},
							),
						},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 200},
						},
					},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected page request: %d", pageNumber)
			}
		},
	}

	implementations, err := GetImplementationsForAPI(t.Context(), client, "api-1")
	require.NoError(t, err)
	require.Len(t, implementations, 2)
	assert.Equal(t, []int64{1, 2}, requestedPages)

	secondImplementation, ok := implementations[1].(kkComps.APIImplementationListItem)
	require.True(t, ok)
	require.NotNil(t, secondImplementation.APIImplementationListItemGatewayServiceEntity)
	assert.Equal(t, "impl-2", secondImplementation.APIImplementationListItemGatewayServiceEntity.ID)
}

func TestGetSnippetsForPortal_PaginatesAcrossPages(t *testing.T) {
	var requestedPages []int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		assert.Equal(t, "/v3/portals/portal-1/snippets", r.URL.Path)

		pageNumber := 1
		if raw := r.URL.Query().Get("page[number]"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			require.NoError(t, err)
			pageNumber = parsed
		}
		requestedPages = append(requestedPages, pageNumber)

		response := map[string]any{
			"meta": map[string]any{
				"page": map[string]any{
					"total": 200,
				},
			},
		}

		switch pageNumber {
		case 1:
			response["data"] = []map[string]any{
				{
					"id":         "snippet-1",
					"name":       "snippet-one",
					"title":      "Snippet One",
					"visibility": "public",
					"status":     "published",
					"created_at": "2026-01-01T00:00:00Z",
					"updated_at": "2026-01-01T00:00:00Z",
				},
			}
		case 2:
			response["data"] = []map[string]any{
				{
					"id":         "snippet-2",
					"name":       "snippet-two",
					"title":      "Snippet Two",
					"visibility": "public",
					"status":     "published",
					"created_at": "2026-01-01T00:00:00Z",
					"updated_at": "2026-01-01T00:00:00Z",
				},
			}
		default:
			t.Fatalf("unexpected page request: %d", pageNumber)
		}

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	client := &PortalAPIImpl{
		SDK: kkSDK.New(
			kkSDK.WithServerURL(server.URL),
			kkSDK.WithClient(server.Client()),
		),
	}

	snippets, err := GetSnippetsForPortal(t.Context(), client, "portal-1")
	require.NoError(t, err)
	require.Len(t, snippets, 2)
	assert.Equal(t, []int{1, 2}, requestedPages)
	assert.Equal(t, "snippet-2", snippets[1].ID)
}
