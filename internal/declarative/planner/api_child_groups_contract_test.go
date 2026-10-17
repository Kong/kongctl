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

func TestExtractedAPIParentContracts(t *testing.T) {
	create := func(ns string) PlannedChange {
		return PlannedChange{
			ID:           "parent-create",
			ResourceType: ResourceTypeAPI,
			ResourceRef:  "api",
			Action:       ActionCreate,
			Namespace:    ns,
		}
	}
	update := func(id string) PlannedChange {
		return PlannedChange{
			ID:           "parent-update-" + id,
			ResourceType: ResourceTypeAPI,
			ResourceRef:  "api",
			ResourceID:   id,
			Action:       ActionUpdate,
			Namespace:    "ignored-change-ns",
		}
	}
	cases := []struct {
		name                     string
		changes                  []PlannedChange
		resolved                 string
		namespace                *string
		external, skip, fail     bool
		readID, childID, childNS string
		creates                  bool
	}{
		{name: "unresolved parent"},
		{
			name:      "new parent default",
			changes:   []PlannedChange{create("")},
			namespace: new("ignored-resource-ns"),
			childNS:   DefaultNamespace,
			creates:   true,
		},
		{name: "new parent namespace", changes: []PlannedChange{create("new-ns")}, childNS: "new-ns", creates: true},
		{
			name:      "resolved external",
			resolved:  "resolved-id",
			external:  true,
			namespace: new("existing-ns"),
			readID:    "resolved-id",
			childID:   "resolved-id",
			childNS:   "existing-ns",
			creates:   true,
		},
		{
			name:      "resolved empty namespace",
			resolved:  "resolved-id",
			namespace: new(""),
			readID:    "resolved-id",
			childID:   "resolved-id",
			childNS:   "",
			creates:   true,
		},
		{
			name:     "first noncreate wins",
			changes:  []PlannedChange{update("first-id"), update("later-id")},
			resolved: "fallback-id",
			readID:   "first-id",
			childID:  "first-id",
			childNS:  DefaultNamespace,
			creates:  true,
		},
		{
			name:     "empty noncreate stops scan",
			changes:  []PlannedChange{update(""), create("unused-ns")},
			resolved: "fallback-id",
			readID:   "fallback-id",
			childID:  "fallback-id",
			childNS:  DefaultNamespace,
			creates:  true,
		},
		{
			name:     "create then resolved fallback",
			changes:  []PlannedChange{create("new-ns")},
			resolved: "resolved-id",
			readID:   "resolved-id",
			childNS:  "new-ns",
			creates:  true,
		},
		{
			name:     "create then noncreate",
			changes:  []PlannedChange{create("new-ns"), update("later-id")},
			resolved: "fallback-id",
			readID:   "later-id",
			childNS:  "new-ns",
			creates:  true,
		},
		{
			name:    "already planned child",
			changes: []PlannedChange{create("")},
			skip:    true,
			childNS: DefaultNamespace,
			creates: true,
		},
		{
			name:     "error after creates",
			changes:  []PlannedChange{create("")},
			resolved: "resolved-id",
			readID:   "resolved-id",
			childNS:  DefaultNamespace,
			creates:  true,
			fail:     true,
		},
	}
	for _, kind := range []string{
		ResourceTypeAPIVersion, ResourceTypeAPIPublication, ResourceTypeAPIImplementation, ResourceTypeAPIDocument,
	} {
		t.Run(kind, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					api := &apiParentCapture{}
					if tc.fail {
						api.err = errors.New("observation failed")
					}
					p := NewPlanner(
						state.NewClient(
							state.ClientConfig{
								APIVersionAPI:        api,
								APIPublicationAPI:    api,
								APIImplementationAPI: api,
								APIDocumentAPI:       api,
							},
						),
						discardPlannerLogger(),
					)
					parent := resources.APIResource{BaseResource: resources.BaseResource{Ref: "api"}}
					if tc.namespace != nil {
						parent.Kongctl = &resources.KongctlMeta{Namespace: tc.namespace}
					}
					parent.SetKonnectID(tc.resolved)
					if tc.external {
						parent.External = &resources.ExternalBlock{ID: tc.resolved}
					}
					p.resources = &resources.ResourceSet{APIs: []resources.APIResource{parent}}
					plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
					for _, change := range tc.changes {
						plan.AddChange(change)
					}
					if tc.skip {
						plan.AddChange(
							PlannedChange{
								ID:           "already-planned",
								ResourceType: kind,
								ResourceRef:  "z-child",
								Action:       ActionUpdate,
							},
						)
					}
					before := len(plan.Changes)
					err := planAPIParentFixture(t.Context(), p, plan, kind)
					if tc.fail {
						require.ErrorIs(t, err, api.err)
					} else {
						require.NoError(t, err)
					}
					if tc.readID == "" {
						require.Empty(t, api.reads)
					} else {
						require.Equal(t, []string{tc.readID}, api.reads)
					}
					changes := plan.Changes[before:]
					if !tc.creates {
						require.Empty(t, changes)
						return
					}
					refs := []string{"z-child", "a-child"}
					if tc.skip {
						refs = refs[1:]
					}
					require.Len(t, changes, len(refs))
					for i, c := range changes {
						require.Equal(t, ActionCreate, c.Action)
						require.Equal(t, refs[i], c.ResourceRef)
						require.Equal(t, kind, c.ResourceType)
						require.Equal(t, tc.childNS, c.Namespace)
						require.Equal(t, &ParentInfo{Ref: "api", ID: tc.childID}, c.Parent)
						if tc.childID == "" {
							require.Contains(t, c.DependsOn, "parent-create")
						}
					}
					if kind == ResourceTypeAPIDocument {
						child := changes[len(changes)-1]
						require.Equal(t, "z-child", child.References[FieldParentDocumentID].Ref)
						require.Equal(t, "parent", child.References[FieldParentDocumentID].LookupFields[FieldSlugPath])
						require.Equal(t, "parent", child.References[FieldParentDocumentID].LookupFields[FieldSlug])
						parentChange := "already-planned"
						if !tc.skip {
							parentChange = changes[0].ID
						}
						require.Contains(t, child.DependsOn, parentChange)
					}
				})
			}
		})
	}
}

func planAPIParentFixture(ctx context.Context, p *Planner, plan *Plan, kind string) error {
	cfg := NewConfig("default")
	refs := []string{"z-child", "a-child"}
	switch kind {
	case ResourceTypeAPIVersion:
		var items []resources.APIVersionResource
		for _, ref := range refs {
			items = append(
				items,
				resources.APIVersionResource{
					Ref:                     ref,
					API:                     "api",
					CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{Version: new(ref)},
				},
			)
		}
		return p.planAPIVersionsChanges(ctx, cfg, items, plan)
	case ResourceTypeAPIPublication:
		var items []resources.APIPublicationResource
		for _, ref := range refs {
			items = append(items, resources.APIPublicationResource{Ref: ref, API: "api", PortalID: ref})
		}
		return p.planAPIPublicationsChanges(ctx, cfg, items, plan)
	case ResourceTypeAPIImplementation:
		var items []resources.APIImplementationResource
		for _, ref := range refs {
			items = append(
				items,
				resources.APIImplementationResource{
					Ref: ref,
					API: "api",
					APIImplementation: kkComps.CreateAPIImplementationControlPlaneReference(
						kkComps.ControlPlaneReference{
							ControlPlane: &kkComps.APIImplementationControlPlaneInput{ID: ref},
						},
					),
				},
			)
		}
		return p.planAPIImplementationsChanges(ctx, cfg, items, plan)
	default:
		return p.planAPIDocumentsChanges(ctx, cfg, []resources.APIDocumentResource{
			{
				Ref:                      refs[0],
				API:                      "api",
				CreateAPIDocumentRequest: kkComps.CreateAPIDocumentRequest{Slug: new("parent"), Content: "body"},
			},
			{
				Ref:                      refs[1],
				API:                      "api",
				ParentDocumentRef:        refs[0],
				CreateAPIDocumentRequest: kkComps.CreateAPIDocumentRequest{Slug: new("child"), Content: "body"},
			},
		}, plan)
	}
}

type apiParentCapture struct {
	stubAPIVersionAPI
	stubAPIPublicationAPI
	stubAPIImplementationAPI
	stubAPIDocumentAPI
	reads []string
	err   error
}

func (a *apiParentCapture) ListAPIVersions(
	ctx context.Context,
	req kkOps.ListAPIVersionsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAPIVersionsResponse, error) {
	a.reads = append(a.reads, req.APIID)
	if a.err != nil {
		return nil, a.err
	}
	return a.stubAPIVersionAPI.ListAPIVersions(ctx, req, opts...)
}

func (a *apiParentCapture) ListAPIPublications(
	ctx context.Context,
	req kkOps.ListAPIPublicationsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAPIPublicationsResponse, error) {
	a.reads = append(a.reads, *req.Filter.APIID.Eq)
	if a.err != nil {
		return nil, a.err
	}
	return a.stubAPIPublicationAPI.ListAPIPublications(ctx, req, opts...)
}

func (a *apiParentCapture) ListAPIImplementations(
	ctx context.Context,
	req kkOps.ListAPIImplementationsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAPIImplementationsResponse, error) {
	a.reads = append(a.reads, *req.Filter.APIID.Eq)
	if a.err != nil {
		return nil, a.err
	}
	return a.stubAPIImplementationAPI.ListAPIImplementations(ctx, req, opts...)
}

func (a *apiParentCapture) ListAPIDocuments(
	ctx context.Context,
	id string,
	filter *kkComps.APIDocumentFilterParameters,
	opts ...kkOps.Option,
) (*kkOps.ListAPIDocumentsResponse, error) {
	a.reads = append(a.reads, id)
	if a.err != nil {
		return nil, a.err
	}
	return a.stubAPIDocumentAPI.ListAPIDocuments(ctx, id, filter, opts...)
}
