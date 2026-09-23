package planner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRootOwnerLifecycleChildAndProtectionOrdering(t *testing.T) {
	for _, kind := range []string{ResourceTypeControlPlane, ResourceTypePortal} {
		for _, decision := range []string{"no-op", "blocked update", "remove protection"} {
			for _, childFails := range []bool{false, true} {
				name := kind + "/" + decision + "/child succeeds"
				if childFails {
					name = kind + "/" + decision + "/child fails"
				}
				t.Run(name, func(t *testing.T) {
					run, reads, childKind := rootOwnerFixture(t, kind, decision, childFails)
					plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
					err := run(plan)
					require.Equal(t, []string{"kept-id"}, *reads)
					var refs []string
					for _, change := range plan.Changes {
						refs = append(refs, change.ResourceRef)
					}
					var want []string
					if decision == "remove protection" {
						want = append(want, "kept-ref")
						require.Equal(t, ActionUpdate, plan.Changes[0].Action)
						require.Equal(t, ProtectionChange{Old: true, New: false}, plan.Changes[0].Protection)
					}
					if childFails {
						require.ErrorContains(t, err, "child observation failed")
						require.NotContains(t, err.Error(), "protected resources")
						require.Equal(t, want, refs, "child failure must stop later parents and sync pruning")
						return
					}
					require.ErrorContains(t, err, kind+" \"stale\" is protected and cannot be deleted")
					if decision == "blocked update" {
						require.ErrorContains(t, err, kind+" \"kept\" is protected and cannot be updated")
						require.Less(t, strings.Index(err.Error(), "\"kept\""), strings.Index(err.Error(), "\"stale\""))
					} else {
						require.NotContains(t, err.Error(), "\"kept\"")
					}
					want = append(want, "kept-child", "later-ref", "later-child")
					require.Equal(t, want, refs, "children must be interleaved with their parents")
					keptChild := plan.Changes[len(plan.Changes)-3]
					require.Equal(t, childKind, keptChild.ResourceType)
					require.Equal(t, "kept-id", keptChild.Parent.ID)
					parent := plan.Changes[len(plan.Changes)-2]
					child := plan.Changes[len(plan.Changes)-1]
					require.Equal(t, kind, parent.ResourceType)
					require.Equal(t, ActionCreate, parent.Action)
					require.Equal(t, childKind, child.ResourceType)
					require.Equal(t, ActionCreate, child.Action)
					require.Equal(t, []string{parent.ID}, child.DependsOn)
				})
			}
		}
	}
}

func rootOwnerFixture(t *testing.T, kind, decision string, childFails bool) (func(*Plan) error, *[]string, string) {
	t.Helper()
	var reads []string
	readChild := func(id string) error {
		reads = append(reads, id)
		if childFails {
			return errors.New("child observation failed")
		}
		return nil
	}
	meta := &resources.KongctlMeta{Namespace: new("default"), Protected: new(decision != "remove protection")}
	var description *string
	if decision == "blocked update" {
		description = new("changed")
	}
	observedLabels := map[string]string{labels.NamespaceKey: "default", labels.ProtectedKey: "true"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if kind == ResourceTypeControlPlane {
		api := helpers.NewMockControlPlaneAPI(t)
		current := []kkComps.ControlPlane{
			{ID: "kept-id", Name: "kept", Labels: observedLabels},
			{ID: "stale-id", Name: "stale", Labels: observedLabels},
		}
		api.EXPECT().ListControlPlanes(mock.Anything, mock.Anything).
			Return(newListControlPlaneResponse(current, float64(len(current))), nil).Once()
		certs := &mockDataPlaneCertificateAPI{list: func(_ context.Context, id string) (
			*kkOps.ListDpClientCertificatesResponse, error,
		) {
			if err := readChild(id); err != nil {
				return nil, err
			}
			return dataPlaneCertificateListResponse(nil), nil
		}}
		p := NewPlanner(state.NewClient(state.ClientConfig{ControlPlaneAPI: api, DataPlaneCertificateAPI: certs}), logger)
		p.resources = &resources.ResourceSet{ControlPlanes: []resources.ControlPlaneResource{
			{
				CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{Name: "kept", Description: description},
				BaseResource:              resources.BaseResource{Ref: "kept-ref", Kongctl: meta},
				DataPlaneCertificates: []resources.ControlPlaneDataPlaneCertificateResource{
					{Ref: "kept-child", Cert: "kept-certificate"},
				},
			},
			{
				CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{Name: "later"},
				BaseResource:              resources.BaseResource{Ref: "later-ref"},
				DataPlaneCertificates: []resources.ControlPlaneDataPlaneCertificateResource{
					{Ref: "later-child", Cert: "later-certificate"},
				},
			},
		}}
		scope := p.resources.EnsureSyncScope()
		scope.AddRoot(resources.ResourceTypeControlPlane)
		for _, ref := range []string{"kept-ref", "later-ref"} {
			scope.AddChild(resources.ResourceTypeControlPlane, ref, resources.ResourceTypeControlPlaneDataPlaneCertificate)
		}
		return func(plan *Plan) error {
			return NewControlPlanePlanner(NewBasePlanner(p)).PlanChanges(t.Context(), NewConfig("default"), plan)
		}, &reads, ResourceTypeControlPlaneDataPlaneCertificate
	}
	api := new(MockPortalAPI)
	api.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{Data: []kkComps.ListPortalsResponsePortal{
			newListPortal("kept-id", "kept", observedLabels), newListPortal("stale-id", "stale", observedLabels),
		}},
	}, nil).Once()
	t.Cleanup(func() { api.AssertExpectations(t) })
	p := NewPlanner(state.NewClient(state.ClientConfig{
		PortalAPI: api, PortalSnippetAPI: &rootOwnerSnippetAPI{read: readChild},
	}), logger)
	p.resources = &resources.ResourceSet{Portals: []resources.PortalResource{
		{
			BaseResource: resources.BaseResource{Ref: "kept-ref", Kongctl: meta},
			CreatePortal: kkComps.CreatePortal{Name: "kept", Description: description},
		},
		{BaseResource: resources.BaseResource{Ref: "later-ref"}, CreatePortal: kkComps.CreatePortal{Name: "later"}},
	}}
	p.desiredPortals = p.resources.Portals
	scope := p.resources.EnsureSyncScope()
	scope.AddRoot(resources.ResourceTypePortal)
	for _, ref := range []string{"kept-ref", "later-ref"} {
		scope.AddChild(resources.ResourceTypePortal, ref, resources.ResourceTypePortalSnippet)
	}
	p.desiredPortalSnippets = []resources.PortalSnippetResource{
		{Ref: "kept-child", Portal: "kept-ref", Name: "kept-child", Content: "content"},
		{Ref: "later-child", Portal: "later-ref", Name: "later-child", Content: "content"},
	}
	return func(plan *Plan) error {
		return NewPortalPlanner(NewBasePlanner(p)).PlanChanges(t.Context(), NewConfig("default"), plan)
	}, &reads, ResourceTypePortalSnippet
}

type rootOwnerSnippetAPI struct {
	mockPortalSnippetAPI
	read func(string) error
}

func (a *rootOwnerSnippetAPI) ListPortalSnippets(
	ctx context.Context, req kkOps.ListPortalSnippetsRequest, opts ...kkOps.Option,
) (*kkOps.ListPortalSnippetsResponse, error) {
	if err := a.read(req.PortalID); err != nil {
		return nil, err
	}
	return a.mockPortalSnippetAPI.ListPortalSnippets(ctx, req, opts...)
}
