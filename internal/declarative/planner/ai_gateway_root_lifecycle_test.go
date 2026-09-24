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

func TestAIGatewayRootLifecycleRuntimeIsolation(t *testing.T) {
	for _, direction := range []string{"upgrade", "downgrade"} {
		for _, protect := range []bool{false, true} {
			name := direction + "/ordinary update"
			if protect {
				name = direction + "/protection transition"
			}
			t.Run(name, func(t *testing.T) {
				p, api := aiRootLifecycleFixture(direction, protect)
				plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
				err := p.planAIGatewayChanges(t.Context(), NewConfig("default"), p.resources.AIGateways, plan)
				require.ErrorContains(t, err, "ai_gateway \"blocked\" is protected and cannot be updated")
				require.Equal(t, []string{"first-id", "blocked-id", "external-id"}, api.reads)
				require.Len(t, plan.Changes, 6)
				var refs []string
				for _, change := range plan.Changes {
					refs = append(refs, change.ResourceRef)
				}
				require.Equal(t,
					[]string{"first-ref", "first-store", "blocked-store", "external-store", "new-ref", "new-store"}, refs)
				update, child := plan.Changes[0], plan.Changes[1]
				require.Equal(t, ActionUpdate, update.Action)
				require.Equal(t, ProtectionChange{Old: false, New: protect}, update.Protection)
				require.Equal(t, "first-id", child.Parent.ID)
				if direction == "downgrade" {
					require.Equal(t, []string{child.ID}, update.DependsOn)
					require.Empty(t, child.DependsOn)
				} else {
					require.Empty(t, update.DependsOn)
					require.Equal(t, []string{update.ID}, child.DependsOn)
				}
				require.Equal(t, "blocked-id", plan.Changes[2].Parent.ID)
				require.Empty(t, plan.Changes[2].DependsOn, "blocked updates must not reuse a prior gateway's update ID")
				require.Equal(t, "external-id", plan.Changes[3].Parent.ID)
				require.Empty(t, plan.Changes[3].DependsOn)
				require.Equal(t, ActionCreate, plan.Changes[4].Action)
				require.Equal(t, []string{plan.Changes[4].ID}, plan.Changes[5].DependsOn)
			})
		}
	}
}

func TestAIGatewayRootLifecycleChildFailureStopsBeforePruning(t *testing.T) {
	p, api := aiRootLifecycleFixture("upgrade", false)
	api.failID = "blocked-id"
	api.gateways = append(api.gateways, kkComps.AIGateway{
		ID: "stale-id", Name: "stale", Labels: map[string]string{labels.NamespaceKey: "default"},
	})
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	err := p.planAIGatewayChanges(t.Context(), NewConfig("default"), p.resources.AIGateways, plan)
	require.ErrorContains(t, err, "child observation failed")
	require.NotContains(t, err.Error(), "protected resources", "child errors take precedence over protection errors")
	require.Equal(t, []string{"first-id", "blocked-id"}, api.reads)
	require.Len(t, plan.Changes, 2, "no later parents or sync deletes after a child failure")
	require.Equal(t, "first-ref", plan.Changes[0].ResourceRef)
	require.Equal(t, "first-store", plan.Changes[1].ResourceRef)
	require.Equal(t, []string{plan.Changes[0].ID}, plan.Changes[1].DependsOn)
}

func TestAIGatewayRootLifecyclePrunesObservedIdentities(t *testing.T) {
	current := []kkComps.AIGateway{
		{ID: "old-id", Name: "same"},
		{ID: "kept-id", Name: "same"},
		{ID: "unnamed-id", DisplayName: "unnamed"},
		{DisplayName: "display-only"},
	}
	for i := range current {
		current[i].Labels = map[string]string{labels.NamespaceKey: "default"}
	}
	desired := resources.AIGatewayResource{
		BaseResource:           resources.BaseResource{Ref: "local-ref"},
		CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{Name: "same"},
	}
	desired.SetKonnectID("old-id")
	p := NewPlanner(state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{gateways: current},
	}), slog.Default())
	p.resources = &resources.ResourceSet{AIGateways: []resources.AIGatewayResource{desired}}
	p.resources.EnsureSyncScope().AddRoot(resources.ResourceTypeAIGateway)
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	require.NoError(t, p.planAIGatewayChanges(t.Context(), NewConfig("default"), p.resources.AIGateways, plan))
	var ids, refs []string
	for _, change := range plan.Changes {
		require.Equal(t, ActionDelete, change.Action)
		ids = append(ids, change.ResourceID)
		refs = append(refs, change.ResourceRef)
	}
	require.Equal(t, []string{"old-id", "unnamed-id", ""}, ids,
		"match the last name observation, ignoring cached IDs; prune unmatched observations in source order")
	require.Equal(t, []string{"same", "unnamed", "display-only"}, refs)
}

func aiRootLifecycleFixture(direction string, protect bool) (*Planner, *aiRootLifecycleAPI) {
	oldVersion, newVersion := "2.0", "2.1"
	if direction == "downgrade" {
		oldVersion, newVersion = newVersion, oldVersion
	}
	api := &aiRootLifecycleAPI{testAIGatewayAPI: testAIGatewayAPI{gateways: []kkComps.AIGateway{
		{
			ID: "first-id", Name: "first", DisplayName: "first", MinRuntimeVersion: new(oldVersion),
			Labels: map[string]string{labels.NamespaceKey: "default"},
		},
		{
			ID: "blocked-id", Name: "blocked", DisplayName: "blocked", MinRuntimeVersion: new(oldVersion),
			Labels: map[string]string{labels.NamespaceKey: "default", labels.ProtectedKey: "true"},
		},
	}}}
	rs := &resources.ResourceSet{}
	for _, name := range []string{"first", "blocked", "external", "new"} {
		gateway := resources.AIGatewayResource{
			BaseResource: resources.BaseResource{Ref: name + "-ref", Kongctl: &resources.KongctlMeta{
				Protected: new(name == "blocked" || name == "first" && protect),
			}},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name: name, DisplayName: name, MinRuntimeVersion: new(newVersion),
			},
		}
		if name == "external" {
			gateway.External = &resources.ExternalBlock{ID: "external-id"}
			gateway.SetKonnectID("external-id")
		}
		rs.AIGateways = append(rs.AIGateways, gateway)
		rs.AIGatewayConfigStores = append(rs.AIGatewayConfigStores, resources.AIGatewayConfigStoreResource{
			BaseResource: resources.BaseResource{Ref: name + "-store"}, AIGateway: gateway.Ref, Name: name + "-store",
		})
		rs.EnsureSyncScope().AddChild(
			resources.ResourceTypeAIGateway, gateway.Ref, resources.ResourceTypeAIGatewayConfigStore,
		)
	}
	p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayAPI: api, AIGatewayConfigStoresAPI: api}), slog.Default())
	p.resources = rs
	return p, api
}

type aiRootLifecycleAPI struct {
	testAIGatewayAPI
	testAIGatewayConfigStoreAPI
	reads  []string
	failID string
}

func (a *aiRootLifecycleAPI) ListAiGatewayConfigStores(
	ctx context.Context, req kkOps.ListAiGatewayConfigStoresRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewayConfigStoresResponse, error) {
	a.reads = append(a.reads, req.GatewayID)
	if req.GatewayID == a.failID {
		return nil, errors.New("child observation failed")
	}
	return a.testAIGatewayConfigStoreAPI.ListAiGatewayConfigStores(ctx, req, opts...)
}
