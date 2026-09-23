package planner

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestMappedChildLifecycleStopsBeforePruning(t *testing.T) {
	for _, failCreate := range []bool{false, true} {
		name := "matched failure"
		if failCreate {
			name = "create failure"
		}
		t.Run(name, func(t *testing.T) {
			failure := errors.New("nested planning failed")
			var events []string
			err := reconcileMappedChildren([]string{"new", "kept", "later"},
				map[string]string{"kept": "kept-id", "stale": "stale-id"},
				mappedChildOperations[string, string]{
					desiredName: func(name string) string { return name },
					create: func(name string) error {
						events = append(events, "create "+name)
						if failCreate {
							return failure
						}
						return nil
					},
					matched: func(name, id string) error {
						events = append(events, "match "+name+" "+id)
						return failure
					},
					remove: func(name, id string) { events = append(events, "delete "+name+" "+id) },
				}, PlanModeSync)
			require.ErrorIs(t, err, failure)
			want := []string{"create new"}
			if !failCreate {
				want = append(want, "match kept kept-id")
			}
			require.Equal(t, want, events)
		})
	}
}

func TestEventMappedLifecycleStaticKeyReplacement(t *testing.T) {
	api := &mappedLifecycleAPI{}
	p := NewPlanner(state.NewClient(state.ClientConfig{EventGatewayStaticKeyAPI: api}), slog.Default())
	desired := resources.EventGatewayStaticKeyResource{
		Ref: "key-ref", EventGatewayStaticKeyCreate: kkComps.EventGatewayStaticKeyCreate{
			Name: "duplicate", Description: new("updated"), Value: "fixture-value",
		},
	}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	err := p.planStaticKeyChangesForExistingGateway(t.Context(), "default", "gateway-id", "gateway-ref", "Gateway",
		[]resources.EventGatewayStaticKeyResource{desired}, plan)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 3)
	require.Equal(t, ActionDelete, plan.Changes[0].Action)
	require.Equal(t, "last", plan.Changes[0].ResourceID)
	require.Equal(t, ActionCreate, plan.Changes[1].Action)
	require.Equal(t, "key-ref", plan.Changes[1].ResourceRef)
	require.Equal(t, []string{plan.Changes[0].ID}, plan.Changes[1].DependsOn)
	require.Equal(t, ActionDelete, plan.Changes[2].Action)
	require.Equal(t, "empty", plan.Changes[2].ResourceID)
}

func TestEventMappedLifecycleListBasedMatching(t *testing.T) {
	t.Run("schema registry moniker", func(t *testing.T) {
		p := NewPlanner(state.NewClient(state.ClientConfig{EventGatewaySchemaRegistryAPI: &mappedLifecycleAPI{}}),
			slog.Default())
		desired := schemaRegistryResourceWithPassword("")
		desired.Ref = "registry-ref"
		desired.SchemaRegistryConfluent.Name = "duplicate"
		plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
		err := p.planSchemaRegistryChangesForExistingGateway(
			t.Context(),
			"default",
			"gateway-id",
			"gateway-ref",
			"Gateway",
			[]resources.EventGatewaySchemaRegistryResource{desired},
			plan,
		)
		require.NoError(t, err)
		require.Len(t, plan.Changes, 1)
		require.Equal(t, ActionUpdate, plan.Changes[0].Action)
		require.Equal(t, "last", plan.Changes[0].ResourceID)
		require.Equal(t, "registry-ref", plan.Changes[0].ResourceRef)
	})
	t.Run("trust bundle moniker", func(t *testing.T) {
		p := NewPlanner(state.NewClient(state.ClientConfig{EventGatewayTLSTrustBundleAPI: &mappedLifecycleAPI{}}),
			slog.Default())
		desired := makeTrustBundleDesired("duplicate", new("updated"), sampleTrustedCA(), nil)
		plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
		err := p.planTrustBundleChangesForExistingGateway(
			t.Context(),
			"default",
			"gateway-id",
			"gateway-ref",
			"Gateway",
			[]resources.EventGatewayTLSTrustBundleResource{desired},
			plan,
		)
		require.NoError(t, err)
		require.Len(t, plan.Changes, 1)
		require.Equal(t, ActionUpdate, plan.Changes[0].Action)
		require.Equal(t, "last", plan.Changes[0].ResourceID)
		require.Equal(t, desired.Ref, plan.Changes[0].ResourceRef)
	})
}

func TestEventMappedLifecycleListenerChildFailure(t *testing.T) {
	api := &mappedLifecycleAPI{listeners: []kkComps.EventGatewayListener{
		{ID: "kept-id", Name: "kept"}, {ID: "stale-id", Name: "stale"},
	}}
	p := NewPlanner(state.NewClient(state.ClientConfig{EventGatewayListenerAPI: api}), slog.Default())
	p.resources = &resources.ResourceSet{}
	p.resources.EnsureSyncScope().AddChild(resources.ResourceTypeEventGatewayListener, "kept-ref",
		resources.ResourceTypeEventGatewayListenerPolicy)
	desired := []resources.EventGatewayListenerResource{
		{Ref: "kept-ref", CreateEventGatewayListenerRequest: kkComps.CreateEventGatewayListenerRequest{Name: "kept"}},
		{Ref: "later-ref", CreateEventGatewayListenerRequest: kkComps.CreateEventGatewayListenerRequest{Name: "later"}},
	}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	err := p.planListenerChangesForExistingGateway(t.Context(), "default", "gateway-id", "gateway-ref", "Gateway",
		desired, plan)
	require.ErrorContains(t, err, "failed to list listener policies")
	require.Empty(t, plan.Changes, "a no-op parent must still plan children and propagate their error before pruning")
}

func (a *mappedLifecycleAPI) FetchEventGatewayListener(
	_ context.Context, _ string, id string, _ ...kkOps.Option,
) (*kkOps.GetEventGatewayListenerResponse, error) {
	for _, listener := range a.listeners {
		if listener.ID == id {
			return &kkOps.GetEventGatewayListenerResponse{EventGatewayListener: &listener}, nil
		}
	}
	return nil, errors.New("unexpected detail read")
}

func TestEventMappedLifecyclePrunesEligibleIndexOnly(t *testing.T) {
	for _, family := range []struct {
		kind      string
		run       func(context.Context, *Planner, *Plan) error
		omitEmpty bool
	}{
		{
			kind: ResourceTypeEventGatewayBackendCluster, omitEmpty: false,
			run: func(ctx context.Context, p *Planner, plan *Plan) error {
				return p.planBackendClusterChangesForExistingGateway(
					ctx, "default", "gateway-id", "gateway-ref",
					"Gateway", nil, plan,
				)
			},
		},
		{
			kind: ResourceTypeEventGatewayListener, omitEmpty: false,
			run: func(ctx context.Context, p *Planner, plan *Plan) error {
				return p.planListenerChangesForExistingGateway(
					ctx, "default", "gateway-id", "gateway-ref",
					"Gateway", nil, plan,
				)
			},
		},
		{
			kind: ResourceTypeEventGatewaySchemaRegistry, omitEmpty: false,
			run: func(ctx context.Context, p *Planner, plan *Plan) error {
				return p.planSchemaRegistryChangesForExistingGateway(
					ctx, "default", "gateway-id", "gateway-ref",
					"Gateway", nil, plan,
				)
			},
		},
		{
			kind: ResourceTypeEventGatewayTLSTrustBundle, omitEmpty: false,
			run: func(ctx context.Context, p *Planner, plan *Plan) error {
				return p.planTrustBundleChangesForExistingGateway(
					ctx, "default", "gateway-id", "gateway-ref",
					"Gateway", nil, plan,
				)
			},
		},
		{
			kind: ResourceTypeEventGatewayStaticKey, omitEmpty: false,
			run: func(ctx context.Context, p *Planner, plan *Plan) error {
				return p.planStaticKeyChangesForExistingGateway(
					ctx, "default", "gateway-id", "gateway-ref",
					"Gateway", nil, plan,
				)
			},
		},
		{
			kind: ResourceTypeEventGatewayClusterPolicy, omitEmpty: false,
			run: func(ctx context.Context, p *Planner, plan *Plan) error {
				return p.planClusterPolicyChangesForExistingVirtualCluster(
					ctx, "default", "gateway-id", "gateway-ref",
					"parent-id", "parent-ref", "Parent", nil, plan,
				)
			},
		},
		{
			kind: ResourceTypeEventGatewayListenerPolicy, omitEmpty: false,
			run: func(ctx context.Context, p *Planner, plan *Plan) error {
				return p.planListenerPolicyChangesForExistingListener(
					ctx, "default", "gateway-id", "gateway-ref",
					"parent-id", "parent-ref", "Parent", nil, plan,
				)
			},
		},
		{
			kind: ResourceTypeEventGatewayProducePolicy, omitEmpty: true,
			run: func(ctx context.Context, p *Planner, plan *Plan) error {
				return p.planProducePolicyChangesForExistingVirtualCluster(
					ctx, "default", "gateway-id", "gateway-ref",
					"parent-id", "parent-ref", "Parent", nil, plan,
				)
			},
		},
		{
			kind: ResourceTypeEventGatewayConsumePolicy, omitEmpty: false,
			run: func(ctx context.Context, p *Planner, plan *Plan) error {
				return p.planConsumePolicyChangesForExistingVirtualCluster(
					ctx, "default", "gateway-id", "gateway-ref",
					"parent-id", "parent-ref", "Parent", nil, plan,
				)
			},
		},
	} {
		t.Run(family.kind, func(t *testing.T) {
			for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
				t.Run(string(mode), func(t *testing.T) {
					api := &mappedLifecycleAPI{}
					p := NewPlanner(state.NewClient(state.ClientConfig{
						EventGatewayBackendClusterAPI: api,
						EventGatewayListenerAPI:       api,
						EventGatewaySchemaRegistryAPI: api,
						EventGatewayTLSTrustBundleAPI: api,
						EventGatewayStaticKeyAPI:      api,
						EventGatewayClusterPolicyAPI:  api,
						EventGatewayListenerPolicyAPI: api,
						EventGatewayProducePolicyAPI:  api,
						EventGatewayConsumePolicyAPI:  api,
					}), slog.Default())
					p.resources = &resources.ResourceSet{}
					plan := NewPlan(CurrentPlanVersion, "test", mode)
					require.NoError(t, family.run(t.Context(), p, plan))
					var deleted []string
					for _, change := range plan.Changes {
						require.Equal(t, ActionDelete, change.Action)
						require.Equal(t, family.kind, change.ResourceType)
						deleted = append(deleted, change.ResourceID)
					}
					if mode == PlanModeApply {
						require.Empty(t, deleted)
					} else {
						want := []string{"last", "empty"}
						if family.omitEmpty {
							want = []string{"last"}
						}
						require.ElementsMatch(t, want, deleted, "preserve duplicate and nil/empty-name indexing")
					}
				})
			}
		})
	}
}

type mappedLifecycleAPI struct {
	helpers.EventGatewayBackendClusterAPI
	helpers.EventGatewayListenerAPI
	helpers.EventGatewaySchemaRegistryAPI
	helpers.EventGatewayTLSTrustBundleAPI
	helpers.EventGatewayStaticKeyAPI
	helpers.EventGatewayClusterPolicyAPI
	helpers.EventGatewayListenerPolicyAPI
	helpers.EventGatewayProducePolicyAPI
	helpers.EventGatewayConsumePolicyAPI
	listeners []kkComps.EventGatewayListener
}

func (a *mappedLifecycleAPI) ListEventGatewayBackendClusters(
	_ context.Context,
	_ kkOps.ListEventGatewayBackendClustersRequest,
	_ ...kkOps.Option,
) (*kkOps.ListEventGatewayBackendClustersResponse, error) {
	data := []kkComps.BackendCluster{
		{ID: "first", Name: "duplicate"},
		{ID: "last", Name: "duplicate"},
		{ID: "empty", Name: ""},
	}
	return &kkOps.ListEventGatewayBackendClustersResponse{
		ListBackendClustersResponse: &kkComps.ListBackendClustersResponse{
			Data: data,
			Meta: eventGatewayExternalCursorMeta(),
		},
	}, nil
}

func (a *mappedLifecycleAPI) ListEventGatewayListeners(
	_ context.Context,
	_ kkOps.ListEventGatewayListenersRequest,
	_ ...kkOps.Option,
) (*kkOps.ListEventGatewayListenersResponse, error) {
	data := []kkComps.EventGatewayListener{
		{ID: "first", Name: "duplicate"},
		{ID: "last", Name: "duplicate"},
		{ID: "empty", Name: ""},
	}
	if a.listeners != nil {
		data = a.listeners
	}
	return &kkOps.ListEventGatewayListenersResponse{
		ListEventGatewayListenersResponse: &kkComps.ListEventGatewayListenersResponse{
			Data: data,
			Meta: eventGatewayExternalCursorMeta(),
		},
	}, nil
}

func (a *mappedLifecycleAPI) ListEventGatewaySchemaRegistries(
	_ context.Context,
	_ kkOps.ListEventGatewaySchemaRegistriesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListEventGatewaySchemaRegistriesResponse, error) {
	data := []kkComps.SchemaRegistry{
		{ID: "first", Name: "duplicate", Type: "confluent"},
		{ID: "last", Name: "duplicate", Type: "confluent"},
		{ID: "empty", Name: "", Type: "confluent"},
	}
	return &kkOps.ListEventGatewaySchemaRegistriesResponse{
		ListSchemaRegistriesResponse: &kkComps.ListSchemaRegistriesResponse{
			Data: data,
			Meta: eventGatewayExternalCursorMeta(),
		},
	}, nil
}

func (a *mappedLifecycleAPI) ListEventGatewayTLSTrustBundles(
	_ context.Context,
	_ kkOps.ListEventGatewayTLSTrustBundlesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListEventGatewayTLSTrustBundlesResponse, error) {
	data := []kkComps.TLSTrustBundle{
		{ID: "first", Name: "duplicate"},
		{ID: "last", Name: "duplicate"},
		{ID: "empty", Name: ""},
	}
	return &kkOps.ListEventGatewayTLSTrustBundlesResponse{
		ListTLSTrustBundlesResponse: &kkComps.ListTLSTrustBundlesResponse{
			Data: data,
			Meta: eventGatewayExternalCursorMeta(),
		},
	}, nil
}

func (a *mappedLifecycleAPI) ListEventGatewayStaticKeys(
	_ context.Context,
	_ kkOps.ListEventGatewayStaticKeysRequest,
	_ ...kkOps.Option,
) (*kkOps.ListEventGatewayStaticKeysResponse, error) {
	data := []kkComps.EventGatewayStaticKey{
		{ID: "first", Name: "duplicate"},
		{ID: "last", Name: "duplicate"},
		{ID: "empty", Name: ""},
	}
	return &kkOps.ListEventGatewayStaticKeysResponse{
		ListEventGatewayStaticKeysResponse: &kkComps.ListEventGatewayStaticKeysResponse{
			Data: data,
			Meta: eventGatewayExternalCursorMeta(),
		},
	}, nil
}

func (a *mappedLifecycleAPI) ListEventGatewayVirtualClusterClusterLevelPolicies(
	_ context.Context,
	_ kkOps.ListEventGatewayVirtualClusterClusterLevelPoliciesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListEventGatewayVirtualClusterClusterLevelPoliciesResponse, error) {
	data := []kkComps.EventGatewayPolicy{
		{ID: "first", Name: new("duplicate")},
		{ID: "last", Name: new("duplicate")},
		{ID: "empty", Name: new("")},
		{ID: "nil"},
	}
	return &kkOps.ListEventGatewayVirtualClusterClusterLevelPoliciesResponse{ListClusterPoliciesResponse: data}, nil
}

func (a *mappedLifecycleAPI) ListEventGatewayListenerPolicies(
	_ context.Context,
	_ kkOps.ListEventGatewayListenerPoliciesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListEventGatewayListenerPoliciesResponse, error) {
	data := []kkComps.EventGatewayListenerPolicy{
		{ID: "first", Name: new("duplicate")},
		{ID: "last", Name: new("duplicate")},
		{ID: "empty", Name: new("")},
		{ID: "nil"},
	}
	return &kkOps.ListEventGatewayListenerPoliciesResponse{ListEventGatewayListenerPoliciesResponse: data}, nil
}

func (a *mappedLifecycleAPI) ListEventGatewayVirtualClusterProducePolicies(
	_ context.Context,
	_ kkOps.ListEventGatewayVirtualClusterProducePoliciesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListEventGatewayVirtualClusterProducePoliciesResponse, error) {
	data := []kkComps.EventGatewayPolicy{
		{ID: "first", Name: new("duplicate")},
		{ID: "last", Name: new("duplicate")},
		{ID: "empty", Name: new("")},
		{ID: "nil"},
	}
	return &kkOps.ListEventGatewayVirtualClusterProducePoliciesResponse{ListProducePoliciesResponse: data}, nil
}

func (a *mappedLifecycleAPI) ListEventGatewayVirtualClusterConsumePolicies(
	_ context.Context,
	_ kkOps.ListEventGatewayVirtualClusterConsumePoliciesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListEventGatewayVirtualClusterConsumePoliciesResponse, error) {
	data := []kkComps.EventGatewayPolicy{
		{ID: "first", Name: new("duplicate")},
		{ID: "last", Name: new("duplicate")},
		{ID: "empty", Name: new("")},
		{ID: "nil"},
	}
	return &kkOps.ListEventGatewayVirtualClusterConsumePoliciesResponse{ListConsumePoliciesResponse: data}, nil
}
