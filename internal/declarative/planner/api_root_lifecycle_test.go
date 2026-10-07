package planner

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAPIRootLifecycleChildAndProtectionOrdering(t *testing.T) {
	for _, decision := range []string{"no-op", "blocked update", "remove protection"} {
		for _, fails := range []bool{false, true} {
			name := decision + "/child succeeds"
			if fails {
				name = decision + "/child fails"
			}
			t.Run(name, func(t *testing.T) {
				observed := map[string]string{labels.NamespaceKey: "default", labels.ProtectedKey: "true"}
				versions := &apiRootVersionAPI{fail: fails}
				p := apiRootLifecyclePlanner(t, []kkComps.APIResponseSchema{
					{ID: "kept-id", Name: "kept", Labels: observed},
					{ID: "stale-id", Name: "stale", Labels: observed},
				}, versions)
				var description *string
				if decision == "blocked update" {
					description = new("changed")
				}
				p.resources = &resources.ResourceSet{APIs: []resources.APIResource{
					{
						BaseResource: resources.BaseResource{
							Ref:     "kept-ref",
							Kongctl: &resources.KongctlMeta{Protected: new(decision != "remove protection")},
						},
						CreateAPIRequest: kkComps.CreateAPIRequest{Name: "kept", Description: description},
						Versions: []resources.APIVersionResource{
							{
								Ref:                     "kept-child",
								CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{Version: new("v1")},
							},
						},
					},
					{
						BaseResource: resources.BaseResource{
							Ref: "later-ref",
						}, CreateAPIRequest: kkComps.CreateAPIRequest{Name: "later"},
						Versions: []resources.APIVersionResource{
							{
								Ref:                     "later-child",
								CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{Version: new("v1")},
							},
						},
					},
				}}
				scope := p.resources.EnsureSyncScope()
				scope.AddRoot(resources.ResourceTypeAPI)
				for _, ref := range []string{"kept-ref", "later-ref"} {
					scope.AddChild(resources.ResourceTypeAPI, ref, resources.ResourceTypeAPIVersion)
				}
				plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
				err := p.planAPIChanges(t.Context(), NewConfig("default"), p.resources.APIs, plan)
				require.Equal(t, []string{"kept-id"}, versions.reads)
				var refs []string
				for _, c := range plan.Changes {
					refs = append(refs, c.ResourceRef)
				}
				var want []string
				if decision == "remove protection" {
					want = append(want, "kept-ref")
					require.Equal(t, ProtectionChange{Old: true, New: false}, plan.Changes[0].Protection)
				}
				if fails {
					require.ErrorContains(t, err, "child observation failed")
					require.NotContains(t, err.Error(), "protected resources")
					require.Equal(t, want, refs)
					return
				}
				require.ErrorContains(t, err, "api \"stale\" is protected and cannot be deleted")
				if decision == "blocked update" {
					require.ErrorContains(t, err, "api \"kept\" is protected and cannot be updated")
				}
				want = append(want, "kept-child", "later-ref", "later-child")
				require.Equal(t, want, refs)
				child := plan.Changes[len(plan.Changes)-3]
				require.Equal(t, "kept-id", child.Parent.ID)
				parent := plan.Changes[len(plan.Changes)-2]
				require.Equal(t, []string{parent.ID}, plan.Changes[len(plan.Changes)-1].DependsOn)
			})
		}
	}
}

func TestAPIRootLifecyclePreservesEarlierPayloadIdentity(t *testing.T) {
	p := apiRootLifecyclePlanner(t, []kkComps.APIResponseSchema{
		{ID: "last-id", Name: "kept", Labels: map[string]string{labels.NamespaceKey: "default"}},
	}, nil)
	p.matchedIdentities = make(map[string]matchedResourceIdentity)
	desired := resources.APIResource{
		BaseResource:     resources.BaseResource{Ref: "kept-ref"},
		CreateAPIRequest: kkComps.CreateAPIRequest{Name: "kept", Description: new("__REF__:kept-ref#id")},
	}
	desired.SetKonnectID("earlier-id")
	p.resources = &resources.ResourceSet{APIs: []resources.APIResource{desired}}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
	require.NoError(t, p.planAPIChanges(t.Context(), NewConfig("default"), p.resources.APIs, plan))
	require.Len(t, plan.Changes, 1)
	require.Equal(t, "last-id", plan.Changes[0].ResourceID)
	p.resolveMatchedPayloadIdentities(plan, p.resources)
	require.NoError(t, p.bindPayloadReferences(plan, p.resources))
	require.Equal(t, "earlier-id", plan.Changes[0].Fields[FieldDescription])
	require.Equal(t, "earlier-id", p.resources.APIs[0].GetKonnectID())
}

func TestAPIRootLifecycleSyncUsesNameIndexAndExcludesExternalNames(t *testing.T) {
	p := apiRootLifecyclePlanner(t, []kkComps.APIResponseSchema{
		{ID: "first-id", Name: "shared", Labels: map[string]string{labels.NamespaceKey: "default"}},
		{ID: "last-id", Name: "shared", Labels: map[string]string{labels.NamespaceKey: "default"}},
	}, nil)
	external := resources.APIResource{
		BaseResource:     resources.BaseResource{Ref: "external-ref"},
		CreateAPIRequest: kkComps.CreateAPIRequest{Name: "shared"},
		External:         &resources.ExternalBlock{ID: "external-id"},
	}
	external.SetKonnectID("external-id")
	p.resources = &resources.ResourceSet{APIs: []resources.APIResource{external}}
	p.resources.EnsureSyncScope().AddRoot(resources.ResourceTypeAPI)
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	require.NoError(t, p.planAPIChanges(t.Context(), NewConfig("default"), p.resources.APIs, plan))
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ActionDelete, plan.Changes[0].Action)
	require.Equal(t, "last-id", plan.Changes[0].ResourceID)
}

func apiRootLifecyclePlanner(t *testing.T, current []kkComps.APIResponseSchema, versions *apiRootVersionAPI) *Planner {
	t.Helper()
	api := new(MockAPIAPI)
	api.On("ListApis", mock.Anything, mock.Anything).Return(&kkOps.ListApisResponse{
		ListAPIResponse: &kkComps.ListAPIResponse{
			Data: current, Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: float64(len(current))}},
		},
	}, nil).Once()
	t.Cleanup(func() { api.AssertExpectations(t) })
	config := state.ClientConfig{APIAPI: api}
	if versions != nil {
		config.APIVersionAPI = versions
	}
	return NewPlanner(state.NewClient(config), slog.Default())
}

type apiRootVersionAPI struct {
	stubAPIVersionAPI
	fail  bool
	reads []string
}

func (a *apiRootVersionAPI) ListAPIVersions(ctx context.Context, req kkOps.ListAPIVersionsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAPIVersionsResponse, error) {
	a.reads = append(a.reads, req.APIID)
	if a.fail {
		return nil, errors.New("child observation failed")
	}
	return a.stubAPIVersionAPI.ListAPIVersions(ctx, req, opts...)
}
