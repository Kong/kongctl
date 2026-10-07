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
	"github.com/stretchr/testify/require"
)

func TestEventRootLifecycleChildAndProtectionOrdering(t *testing.T) {
	for _, decision := range []string{"no-op", "update", "blocked update", "remove protection"} {
		for _, fails := range []bool{false, true} {
			name := decision + "/child succeeds"
			if fails {
				name = decision + "/child fails"
			}
			t.Run(name, func(t *testing.T) {
				observed := map[string]string{labels.NamespaceKey: "default"}
				if decision != "update" {
					observed[labels.ProtectedKey] = "true"
				}
				backends := &eventRootBackendAPI{fail: fails}
				p := eventRootLifecyclePlanner([]kkComps.EventGatewayInfo{
					{ID: "first-id", Name: "kept", Labels: observed},
					{ID: "kept-id", Name: "kept", Labels: observed},
					{
						ID:     "stale-id",
						Name:   "stale",
						Labels: map[string]string{labels.NamespaceKey: "default", labels.ProtectedKey: "true"},
					},
				}, backends)
				kept := eventRootDesired("kept-ref", "kept")
				kept.Kongctl = &resources.KongctlMeta{
					Protected: new(decision != "remove protection" && decision != "update"),
				}
				if decision == "update" || decision == "blocked update" {
					kept.Description = new("changed")
				}
				kept.BackendClusters = []resources.EventGatewayBackendClusterResource{{
					Ref: "kept-child", CreateBackendClusterRequest: kkComps.CreateBackendClusterRequest{Name: "kept-child"},
				}}
				later := eventRootDesired("later-ref", "later")
				later.BackendClusters = []resources.EventGatewayBackendClusterResource{{
					Ref: "later-child", CreateBackendClusterRequest: kkComps.CreateBackendClusterRequest{Name: "later-child"},
				}}
				p.resources = &resources.ResourceSet{
					EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{kept, later},
				}
				scope := p.resources.EnsureSyncScope()
				scope.AddRoot(resources.ResourceTypeEventGatewayControlPlane)
				for _, ref := range []string{"kept-ref", "later-ref"} {
					scope.AddChild(
						resources.ResourceTypeEventGatewayControlPlane,
						ref,
						resources.ResourceTypeEventGatewayBackendCluster,
					)
				}
				plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
				err := p.planEGWControlPlaneChanges(
					t.Context(),
					NewConfig("default"),
					p.resources.EventGatewayControlPlanes,
					plan,
				)
				require.Equal(t, []string{"kept-id"}, backends.reads)
				var refs, want []string
				for _, c := range plan.Changes {
					refs = append(refs, c.ResourceRef)
				}
				if decision == "update" || decision == "remove protection" {
					want = append(want, "kept-ref")
					require.Equal(t, ActionUpdate, plan.Changes[0].Action)
					require.Equal(t, "kept-id", plan.Changes[0].ResourceID)
					if decision == "remove protection" {
						require.Equal(t, ProtectionChange{Old: true, New: false}, plan.Changes[0].Protection)
					} else {
						require.Equal(t, "changed", plan.Changes[0].Fields[FieldDescription])
					}
				}
				if fails {
					require.ErrorContains(t, err, "child observation failed")
					require.NotContains(t, err.Error(), "protected resources")
					require.Equal(t, want, refs)
					return
				}
				require.ErrorContains(
					t,
					err,
					"event_gateway \"stale\" is protected and cannot be deleted",
				)
				if decision == "blocked update" {
					require.ErrorContains(
						t,
						err,
						"event_gateway \"kept\" is protected and cannot be updated",
					)
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

func TestEventRootLifecycleDeleteMode(t *testing.T) {
	p := eventRootLifecyclePlanner([]kkComps.EventGatewayInfo{
		{
			ID:     "protected-id",
			Name:   "protected",
			Labels: map[string]string{labels.NamespaceKey: "default", labels.ProtectedKey: "true"},
		},
		{ID: "first-id", Name: "removable", Labels: map[string]string{labels.NamespaceKey: "default"}},
		{ID: "last-id", Name: "removable", Labels: map[string]string{labels.NamespaceKey: "default"}},
	}, nil)
	external := eventRootDesired("external-ref", "external")
	external.External = &resources.ExternalBlock{ID: "external-id"}
	removable := eventRootDesired("removable-ref", "removable")
	removable.SetKonnectID("first-id")
	p.resources = &resources.ResourceSet{EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{
		eventRootDesired("protected-ref", "protected"), external, eventRootDesired("missing-ref", "missing"), removable,
	}}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeDelete)
	err := p.planEGWControlPlaneChanges(t.Context(), NewConfig("default"), p.resources.EventGatewayControlPlanes, plan)
	require.ErrorContains(t, err, "event_gateway \"protected\" is protected and cannot be deleted")
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ActionDelete, plan.Changes[0].Action)
	require.Equal(t, ResourceTypeEventGatewayControlPlane, plan.Changes[0].ResourceType)
	require.Equal(t, "last-id", plan.Changes[0].ResourceID)
	require.Len(t, plan.Warnings, 2)
	require.Equal(
		t,
		"event_gateway_control_plane \"external-ref\" is external, skipping delete",
		plan.Warnings[0].Message,
	)
	require.Equal(
		t,
		"event_gateway_control_plane \"missing\" not found in Konnect, skipping delete",
		plan.Warnings[1].Message,
	)
}

func TestEventRootLifecycleSyncRetainsExternalNames(t *testing.T) {
	backends := &eventRootBackendAPI{}
	p := eventRootLifecyclePlanner([]kkComps.EventGatewayInfo{
		{
			ID:     "managed-id",
			Name:   "shared",
			Labels: map[string]string{labels.NamespaceKey: "default", labels.ProtectedKey: "true"},
		},
		{ID: "first-id", Name: "stale", Labels: map[string]string{labels.NamespaceKey: "default"}},
		{ID: "last-id", Name: "stale", Labels: map[string]string{labels.NamespaceKey: "default"}},
		{ID: "empty-id", Name: "", Labels: map[string]string{labels.NamespaceKey: "default"}},
	}, backends)
	external := eventRootDesired("external-ref", "shared")
	external.External = &resources.ExternalBlock{ID: "external-id"}
	external.SetKonnectID("external-id")
	external.BackendClusters = []resources.EventGatewayBackendClusterResource{{
		Ref: "external-child", CreateBackendClusterRequest: kkComps.CreateBackendClusterRequest{Name: "external-child"},
	}}
	p.resources = &resources.ResourceSet{
		EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{external},
	}
	scope := p.resources.EnsureSyncScope()
	scope.AddRoot(resources.ResourceTypeEventGatewayControlPlane)
	scope.AddChild(
		resources.ResourceTypeEventGatewayControlPlane,
		external.Ref,
		resources.ResourceTypeEventGatewayBackendCluster,
	)
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	require.NoError(
		t,
		p.planEGWControlPlaneChanges(t.Context(), NewConfig("default"), p.resources.EventGatewayControlPlanes, plan),
	)
	require.Equal(t, []string{"external-id"}, backends.reads)
	require.Len(t, plan.Changes, 3)
	require.Equal(t, ActionCreate, plan.Changes[0].Action)
	require.Equal(t, "external-id", plan.Changes[0].Parent.ID)
	require.Empty(t, plan.Changes[0].DependsOn)
	var deleted []string
	for _, c := range plan.Changes[1:] {
		require.Equal(t, ActionDelete, c.Action)
		deleted = append(deleted, c.ResourceID)
	}
	require.ElementsMatch(t, []string{"last-id", "empty-id"}, deleted)
}

func TestEventRootLifecyclePreservesPayloadIdentityBoundary(t *testing.T) {
	for _, bound := range []bool{false, true} {
		name := "unbound"
		if bound {
			name = "already bound"
		}
		t.Run(name, func(t *testing.T) {
			p := eventRootLifecyclePlanner([]kkComps.EventGatewayInfo{
				{ID: "observed-id", Name: "kept", Labels: map[string]string{labels.NamespaceKey: "default"}},
			}, nil)
			p.matchedIdentities = make(map[string]matchedResourceIdentity)
			desired := eventRootDesired("kept-ref", "kept")
			desired.Description = new("__REF__:kept-ref#id")
			if bound {
				desired.SetKonnectID("earlier-id")
			}
			p.resources = &resources.ResourceSet{
				EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{desired},
			}
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
			require.NoError(
				t,
				p.planEGWControlPlaneChanges(
					t.Context(),
					NewConfig("default"),
					p.resources.EventGatewayControlPlanes,
					plan,
				),
			)
			require.Len(t, plan.Changes, 1)
			require.Equal(t, "observed-id", plan.Changes[0].ResourceID)
			p.resolveMatchedPayloadIdentities(plan, p.resources)
			err := p.bindPayloadReferences(plan, p.resources)
			if bound {
				require.NoError(t, err)
				require.Equal(t, "earlier-id", plan.Changes[0].Fields[FieldDescription])
			} else {
				require.ErrorContains(t, err, "has no resolved value or creation operation")
			}
		})
	}
}

func eventRootDesired(ref, name string) resources.EventGatewayControlPlaneResource {
	return resources.EventGatewayControlPlaneResource{
		BaseResource:         resources.BaseResource{Ref: ref},
		CreateGatewayRequest: kkComps.CreateGatewayRequest{Name: name},
	}
}

func eventRootLifecyclePlanner(current []kkComps.EventGatewayInfo, backends *eventRootBackendAPI) *Planner {
	config := state.ClientConfig{EGWControlPlaneAPI: &stubExternalEventGatewayControlPlaneAPI{gateways: current}}
	if backends != nil {
		config.EventGatewayBackendClusterAPI = backends
	}
	return NewPlanner(state.NewClient(config), slog.Default())
}

type eventRootBackendAPI struct {
	stubExternalEventGatewayBackendClusterAPI
	fail  bool
	reads []string
}

func (a *eventRootBackendAPI) ListEventGatewayBackendClusters(ctx context.Context,
	req kkOps.ListEventGatewayBackendClustersRequest, opts ...kkOps.Option,
) (*kkOps.ListEventGatewayBackendClustersResponse, error) {
	a.reads = append(a.reads, req.GatewayID)
	if a.fail {
		return nil, errors.New("child observation failed")
	}
	return a.stubExternalEventGatewayBackendClusterAPI.ListEventGatewayBackendClusters(ctx, req, opts...)
}
