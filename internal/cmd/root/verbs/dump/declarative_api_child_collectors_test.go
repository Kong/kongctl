package dump

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	declresources "github.com/kong/kongctl/internal/declarative/resources"
	declstate "github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestAPIChildCollectorCoverage(t *testing.T) {
	validate := func(c []apiChildCollector) error { return validateChildCollectors("API child dump", c) }
	require.NoError(t, validate(apiChildCollectors))
	for _, tt := range []struct {
		name   string
		change func([]apiChildCollector) []apiChildCollector
		want   string
	}{
		{
			name:   "missing children",
			change: func(c []apiChildCollector) []apiChildCollector { return c[1:3] },
			want: fmt.Sprintf("missing resource types [%s %s]",
				declresources.ResourceTypeAPIImplementation, declresources.ResourceTypeAPIVersion),
		},
		{
			name:   "duplicate",
			change: func(c []apiChildCollector) []apiChildCollector { return append(c, c[0]) },
			want:   "more than once",
		},
		{
			name: "foreign owner",
			change: func(c []apiChildCollector) []apiChildCollector {
				c[0].kind = declresources.ResourceTypePortalPage
				return c
			},
			want: "is not a managed child",
		},
		{
			name: "document recursion is not another kind",
			change: func(c []apiChildCollector) []apiChildCollector {
				c[1].nested = []declresources.ResourceType{declresources.ResourceTypeAPIDocument}
				return c
			},
			want: "is not a managed child",
		},
		{
			name:   "missing callback",
			change: func(c []apiChildCollector) []apiChildCollector { c[0].collect = nil; return c },
			want:   "requires a collector and warning",
		},
		{
			name:   "missing warning",
			change: func(c []apiChildCollector) []apiChildCollector { c[0].warning = " "; return c },
			want:   "requires a collector and warning",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorContains(t, validate(tt.change(slices.Clone(apiChildCollectors))), tt.want)
		})
	}
}

type apiChildDumpHTTPClient func(*http.Request) (*http.Response, error)

func (f apiChildDumpHTTPClient) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestPopulateAPIChildrenExportContract(t *testing.T) {
	for _, tt := range []struct {
		name           string
		failure        string
		empty          bool
		missingContent bool
		warning        string
	}{
		{name: "success"},
		{name: "empty collections", empty: true},
		{name: "versions failure", failure: "versions", warning: "failed to load API versions"},
		{name: "documents failure", failure: "documents", warning: "failed to load API documents"},
		{name: "publications failure", failure: "publications", warning: "failed to load API publications"},
		{name: "implementations failure", failure: "implementations", warning: "failed to load API implementations"},
		{name: "version detail failure", failure: "versions/version-id", warning: "failed to fetch API version"},
		{name: "document detail failure", failure: "documents/parent", warning: "failed to fetch API document"},
		{name: "missing detail content", missingContent: true, warning: "API version missing spec content"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			payloads := apiChildDumpPayloads()
			if tt.missingContent {
				payloads["versions/version-id"] = kkComps.APIVersionResponse{ID: "version-id"}
				payloads["documents/parent"] = kkComps.APIDocumentResponse{ID: "parent"}
			}
			var calls []string
			httpClient := apiChildDumpHTTPClient(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, req.Method)
				var path string
				switch req.URL.Path {
				case "/v3/api-publications", "/v3/api-implementations":
					require.Equal(t, "api-id", req.URL.Query().Get("filter[api_id][eq]"))
					path = strings.TrimPrefix(req.URL.Path, "/v3/api-")
				default:
					require.True(t, strings.HasPrefix(req.URL.Path, "/v3/apis/api-id/"), req.URL.Path)
					path = strings.TrimPrefix(req.URL.Path, "/v3/apis/api-id/")
				}
				calls = append(calls, path)
				payload, ok := payloads[path]
				require.True(t, ok, "unexpected request: %s", path)
				status := http.StatusOK
				if tt.empty {
					payload = map[string]any{"data": []any{}}
				}
				if path == tt.failure {
					status, payload = http.StatusForbidden, map[string]any{"message": "denied"}
				}
				body, err := json.Marshal(payload)
				require.NoError(t, err)
				return &http.Response{
					StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(bytes.NewReader(body)), Request: req,
				}, nil
			})
			sdk := &helpers.KonnectSDK{SDK: kkSDK.New(
				kkSDK.WithServerURL("https://example.test"), kkSDK.WithClient(httpClient),
			)}
			client := declstate.NewClient(declstate.ClientConfig{
				APIVersionAPI: sdk.GetAPIVersionAPI(), APIDocumentAPI: sdk.GetAPIDocumentAPI(),
				APIPublicationAPI: sdk.GetAPIPublicationAPI(), APIImplementationAPI: sdk.GetAPIImplementationAPI(),
			})
			apis := []declresources.APIResource{
				{BaseResource: declresources.BaseResource{Ref: " "}},
				{
					BaseResource: declresources.BaseResource{
						Ref: " api-id ",
					}, CreateAPIRequest: kkComps.CreateAPIRequest{Name: "API name"},
					Versions:        []declresources.APIVersionResource{{Ref: "existing"}},
					Documents:       []declresources.APIDocumentResource{{Ref: "existing"}},
					Publications:    []declresources.APIPublicationResource{{Ref: "existing"}},
					Implementations: []declresources.APIImplementationResource{{Ref: "existing"}},
				},
			}
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			populateAPIChildren(t.Context(), logger, nil, apis)
			require.Empty(t, calls)
			require.Equal(t, "existing", apis[1].Versions[0].Ref)
			populateAPIChildren(t.Context(), logger, client, apis)
			wantCalls := []string{"versions"}
			if !tt.empty && tt.failure != "versions" {
				wantCalls = append(wantCalls, "versions/version-id")
			}
			wantCalls = append(wantCalls, "documents")
			if !tt.empty && tt.failure != "documents" {
				wantCalls = append(
					wantCalls,
					"documents/child-z",
					"documents/parent",
					"documents/grandchild",
					"documents/child-a",
				)
			}
			wantCalls = append(wantCalls, "publications", "implementations")
			require.Equal(t, wantCalls, calls)
			if tt.warning == "" {
				require.Empty(t, logs.String())
			} else {
				require.Contains(t, logs.String(), tt.warning)
				require.Contains(t, logs.String(), "api-id")
				require.Contains(t, logs.String(), "API name")
			}
			if tt.missingContent {
				require.Contains(t, logs.String(), "API document missing content")
			}
			api := apis[1]
			require.Len(t, api.Versions, 1)
			if tt.empty || strings.HasPrefix(tt.failure, "versions") || tt.missingContent {
				require.Equal(t, "existing", api.Versions[0].Ref)
			} else {
				require.Equal(t, "version-id", api.Versions[0].Ref)
				require.Equal(t, "openapi: 3.0.0", *api.Versions[0].Spec.Content)
				require.Empty(t, api.Versions[0].API)
			}
			if tt.empty || tt.failure == "documents" {
				require.Equal(t, []declresources.APIDocumentResource{{Ref: "existing"}}, api.Documents)
			} else {
				children := api.Documents
				if tt.failure != "documents/parent" && !tt.missingContent {
					require.Len(t, api.Documents, 1)
					require.Equal(t, "parent", api.Documents[0].Ref)
					require.Empty(t, api.Documents[0].API)
					children = api.Documents[0].Children
				}
				require.Len(t, children, 2)
				require.Equal(t, "child-z", children[0].Ref)
				require.Equal(t, "child-a", children[1].Ref)
				require.Len(t, children[0].Children, 1)
				require.Equal(t, "grandchild", children[0].Children[0].Ref)
				require.Equal(t, "# child-z", children[0].Content)
				require.Empty(t, children[0].API)
			}
			require.Len(t, api.Publications, 1)
			if tt.empty || tt.failure == "publications" {
				require.Equal(t, "existing", api.Publications[0].Ref)
			} else {
				require.Equal(t, buildChildRef("api-publication", "api-id", "portal-id"), api.Publications[0].Ref)
				require.Equal(t, "portal-id", api.Publications[0].PortalID)
				require.Equal(t, []string{"auth-id"}, api.Publications[0].AuthStrategyIds)
				require.True(t, *api.Publications[0].AutoApproveRegistrations)
				require.Equal(t, kkComps.APIPublicationVisibilityPublic, *api.Publications[0].Visibility)
				require.Empty(t, api.Publications[0].API)
			}
			if tt.empty || tt.failure == "implementations" {
				require.Equal(t, []declresources.APIImplementationResource{{Ref: "existing"}}, api.Implementations)
			} else {
				require.Len(t, api.Implementations, 2)
				require.Equal(t, "service-impl", api.Implementations[0].Ref)
				require.Equal(t, "service-id", api.Implementations[0].ServiceReference.Service.ID)
				require.Equal(t, "cp-id", api.Implementations[0].ServiceReference.Service.ControlPlaneID)
				require.Equal(t, "cp-impl", api.Implementations[1].Ref)
				require.Equal(t, "cp-id", api.Implementations[1].ControlPlaneReference.ControlPlane.ID)
				require.Empty(t, api.Implementations[0].API)
				require.Empty(t, api.Implementations[1].API)
			}
		})
	}
}

func apiChildDumpPayloads() map[string]any {
	list := func(data any) any {
		return map[string]any{"data": data, "meta": map[string]any{"page": map[string]any{"size": 100}}}
	}
	documents := []kkComps.APIDocumentSummaryWithChildren{
		{ID: "child-z", ParentDocumentID: new("parent")},
		{ID: "parent"},
		{ID: "grandchild", ParentDocumentID: new("child-z")},
		{ID: "child-a", ParentDocumentID: new("parent")},
	}
	payloads := map[string]any{
		"versions": list([]kkComps.ListAPIVersionResponseAPIVersionSummary{{ID: "version-id", Version: "v1"}}),
		"versions/version-id": kkComps.APIVersionResponse{
			ID: "version-id", Version: "v1", Spec: &kkComps.APIVersionResponseSpec{Content: new("openapi: 3.0.0")},
		},
		"documents": list(documents),
		"publications": list([]kkComps.APIPublicationListItem{{
			APIID: "api-id", PortalID: "portal-id", Visibility: kkComps.APIPublicationVisibilityPublic,
			AuthStrategyIds: []string{"auth-id"}, AutoApproveRegistrations: true,
		}}),
		"implementations": list([]any{
			kkComps.APIImplementationListItemGatewayServiceEntity{
				ID: "service-impl", APIID: "api-id",
				Service: &kkComps.APIImplementationService{ID: "service-id", ControlPlaneID: "cp-id"},
			},
			kkComps.APIImplementationListItemControlPlaneEntity{
				ID: "cp-impl", APIID: "api-id", ControlPlane: kkComps.APIImplementationControlPlane{ID: "cp-id"},
			},
		}),
	}
	for _, doc := range documents {
		payloads["documents/"+doc.ID] = kkComps.APIDocumentResponse{
			ID: doc.ID, Content: "# " + doc.ID, Title: doc.ID, Slug: doc.ID, ParentDocumentID: doc.ParentDocumentID,
		}
	}
	return payloads
}
