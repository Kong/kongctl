package planner

import (
	"context"
	"errors"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAPIChildTraversalOrderAndContext(t *testing.T) {
	kinds := []string{
		ResourceTypeAPIVersion, ResourceTypeAPIPublication,
		ResourceTypeAPIImplementation, ResourceTypeAPIDocument,
	}
	contexts := map[string]string{
		ResourceTypeAPIVersion:        "failed to plan API version changes: ",
		ResourceTypeAPIPublication:    "failed to plan API publication changes: ",
		ResourceTypeAPIImplementation: "failed to plan API implementation changes: ",
		ResourceTypeAPIDocument:       "failed to plan API document changes: ",
	}
	for _, path := range []string{"new", "existing", "external", "extracted"} {
		failures := []string{""}
		if path != "new" {
			failures = append(failures, kinds...)
		}
		for _, fail := range failures {
			t.Run(path+"/fail="+fail, func(t *testing.T) {
				p, api, parent := apiInventoryFixture()
				api.fail = fail
				plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
				var err error
				switch path {
				case "new":
					p.planAPIChildResourcesCreate("tenant", parent, "parent-create", plan)
				case "existing":
					err = p.planAPIChildResourceChanges(t.Context(), NewConfig("tenant"),
						state.API{APIResponseSchema: kkComps.APIResponseSchema{ID: "api-id"}}, parent, plan)
				default:
					parent.External = &resources.ExternalBlock{ID: "api-id"}
					parent.SetKonnectID("api-id")
					if path == "extracted" {
						p.resources.APIVersions = parent.Versions
						p.resources.APIPublications = parent.Publications
						p.resources.APIImplementations = parent.Implementations
						p.resources.APIDocuments = parent.Documents
						parent.Versions, parent.Publications, parent.Implementations, parent.Documents = nil, nil, nil, nil
					}
					p.resources.APIs = []resources.APIResource{parent}
					err = NewAPIPlanner(NewBasePlanner(p)).PlanChanges(t.Context(), NewConfig(resources.NamespaceExternal), plan)
				}
				wantReads := kinds
				wantKinds := append(append([]string{}, kinds...), ResourceTypeAPIDocument)
				if fail != "" {
					require.ErrorIs(t, err, api.failure)
					for i, kind := range kinds {
						if kind == fail {
							wantReads = kinds[:i+1]
							wantKinds = kinds[:i]
							break
						}
					}
					if path == "extracted" {
						require.NotContains(t, err.Error(), "failed to plan API")
					} else {
						require.Contains(t, err.Error(), contexts[fail])
					}
				} else {
					require.NoError(t, err)
				}
				if path == "new" {
					require.Empty(t, api.kinds)
				} else {
					require.Equal(t, wantReads, api.kinds)
				}
				gotKinds := make([]string, 0, len(plan.Changes))
				for _, change := range plan.Changes {
					gotKinds = append(gotKinds, change.ResourceType)
					require.Equal(t, "tenant", change.Namespace)
					require.Equal(t, "api", change.Parent.Ref)
					if path == "new" {
						require.Empty(t, change.Parent.ID)
						require.Contains(t, change.DependsOn, "parent-create")
					} else {
						require.Equal(t, "api-id", change.Parent.ID)
					}
				}
				require.Equal(t, wantKinds, gotKinds)
				if fail == "" {
					child := plan.Changes[len(plan.Changes)-1]
					require.Equal(t, "document-child", child.ResourceRef)
					require.Equal(t, "document", child.References[FieldParentDocumentID].Ref)
					require.Contains(t, child.DependsOn, plan.Changes[len(plan.Changes)-2].ID)
					lookup := child.References[FieldParentDocumentID].LookupFields
					if path == "new" {
						require.Empty(t, lookup)
					} else {
						require.Equal(t, "parent", lookup[FieldSlugPath])
					}
				}
			})
		}
	}
}

func TestAPIChildTraversalHonorsNestedSyncScope(t *testing.T) {
	p, api, parent := apiInventoryFixture()
	scope := p.resources.EnsureSyncScope()
	scope.AddChild(resources.ResourceTypeAPI, parent.GetRef(), resources.ResourceTypeAPIPublication)
	scope.AddChild(resources.ResourceTypeAPI, parent.GetRef(), resources.ResourceTypeAPIDocument)
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	require.NoError(t, p.planAPIChildResourceChanges(t.Context(), NewConfig("tenant"),
		state.API{APIResponseSchema: kkComps.APIResponseSchema{ID: "api-id"}}, parent, plan))
	require.Equal(t, []string{ResourceTypeAPIPublication, ResourceTypeAPIDocument}, api.kinds)
	refs := make([]string, 0, len(plan.Changes))
	for _, change := range plan.Changes {
		refs = append(refs, change.ResourceRef)
	}
	require.Equal(t, []string{"publication", "document", "document-child"}, refs)
}

func TestAPIChildNewParentMergesDeclarationsAndSkipsPlannedRefs(t *testing.T) {
	p, _, parent := apiInventoryFixture()
	// Declarations for another API must never enter this parent's create pass.
	p.resources.APIVersions = append(append([]resources.APIVersionResource{}, parent.Versions...),
		resources.APIVersionResource{
			Ref: "extra", API: "api",
			CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{Version: new("v2")},
		},
		resources.APIVersionResource{
			Ref: "other", API: "other-api",
			CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{Version: new("v3")},
		})
	p.resources.APIPublications = parent.Publications
	p.resources.APIImplementations = parent.Implementations
	p.resources.APIDocuments = parent.Documents
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
	plan.AddChange(PlannedChange{
		ID: "preplanned", Action: ActionCreate, ResourceType: ResourceTypeAPIVersion, ResourceRef: "version",
	})
	p.planAPIChildResourcesCreate("tenant", parent, "parent-create", plan)
	refs := make([]string, 0, len(plan.Changes))
	for _, change := range plan.Changes {
		refs = append(refs, change.ResourceRef)
	}
	require.Equal(t, []string{"version", "extra", "publication", "implementation", "document", "document-child"}, refs)
}

func apiInventoryFixture() (*Planner, *apiInventoryObservation, resources.APIResource) {
	api := &apiInventoryObservation{failure: errors.New("observation failed")}
	p := NewPlanner(state.NewClient(state.ClientConfig{
		APIVersionAPI: api, APIPublicationAPI: api, APIImplementationAPI: api, APIDocumentAPI: api,
	}), discardPlannerLogger())
	parent := resources.APIResource{
		BaseResource: resources.BaseResource{Ref: "api", Kongctl: &resources.KongctlMeta{Namespace: new("tenant")}},
		Versions: []resources.APIVersionResource{{
			Ref: "version", API: "api",
			CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{Version: new("v1")},
		}},
		Publications: []resources.APIPublicationResource{{Ref: "publication", API: "api", PortalID: "portal-id"}},
		Implementations: []resources.APIImplementationResource{{
			Ref: "implementation", API: "api",
			APIImplementation: kkComps.CreateAPIImplementationControlPlaneReference(kkComps.ControlPlaneReference{
				ControlPlane: &kkComps.APIImplementationControlPlaneInput{ID: "cp-id"},
			}),
		}},
		Documents: []resources.APIDocumentResource{
			{
				Ref: "document", API: "api",
				CreateAPIDocumentRequest: kkComps.CreateAPIDocumentRequest{Slug: new("parent"), Content: "body"},
			},
			{
				Ref: "document-child", API: "api", ParentDocumentRef: "document",
				CreateAPIDocumentRequest: kkComps.CreateAPIDocumentRequest{Slug: new("child"), Content: "body"},
			},
		},
	}
	p.resources = &resources.ResourceSet{APIs: []resources.APIResource{parent}}
	return p, api, parent
}

type apiInventoryObservation struct {
	apiParentCapture
	kinds   []string
	fail    string
	failure error
}

func (a *apiInventoryObservation) observe(kind string) error {
	a.kinds = append(a.kinds, kind)
	if kind == a.fail {
		return a.failure
	}
	return nil
}

func (a *apiInventoryObservation) ListAPIVersions(
	ctx context.Context,
	req kkOps.ListAPIVersionsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAPIVersionsResponse, error) {
	if err := a.observe(ResourceTypeAPIVersion); err != nil {
		return nil, err
	}
	return a.stubAPIVersionAPI.ListAPIVersions(ctx, req, opts...)
}

func (a *apiInventoryObservation) ListAPIPublications(
	ctx context.Context,
	req kkOps.ListAPIPublicationsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAPIPublicationsResponse, error) {
	if err := a.observe(ResourceTypeAPIPublication); err != nil {
		return nil, err
	}
	return a.stubAPIPublicationAPI.ListAPIPublications(ctx, req, opts...)
}

func (a *apiInventoryObservation) ListAPIImplementations(
	ctx context.Context,
	req kkOps.ListAPIImplementationsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAPIImplementationsResponse, error) {
	if err := a.observe(ResourceTypeAPIImplementation); err != nil {
		return nil, err
	}
	return a.stubAPIImplementationAPI.ListAPIImplementations(ctx, req, opts...)
}

func (a *apiInventoryObservation) ListAPIDocuments(
	ctx context.Context, id string,
	filter *kkComps.APIDocumentFilterParameters,
	opts ...kkOps.Option,
) (*kkOps.ListAPIDocumentsResponse, error) {
	if err := a.observe(ResourceTypeAPIDocument); err != nil {
		return nil, err
	}
	return a.stubAPIDocumentAPI.ListAPIDocuments(ctx, id, filter, opts...)
}
