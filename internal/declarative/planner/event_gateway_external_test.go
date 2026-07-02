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

type stubExternalEventGatewayControlPlaneAPI struct {
	gateways []kkComps.EventGatewayInfo
}

func (s *stubExternalEventGatewayControlPlaneAPI) ListEGWControlPlanes(
	_ context.Context,
	_ kkOps.ListEventGatewaysRequest,
	_ ...kkOps.Option,
) (*kkOps.ListEventGatewaysResponse, error) {
	return &kkOps.ListEventGatewaysResponse{
		StatusCode: 200,
		ListEventGatewaysResponse: &kkComps.ListEventGatewaysResponse{
			Meta: *eventGatewayExternalCursorMeta(),
			Data: s.gateways,
		},
	}, nil
}

func (s *stubExternalEventGatewayControlPlaneAPI) FetchEGWControlPlane(
	_ context.Context,
	gatewayID string,
	_ ...kkOps.Option,
) (*kkOps.GetEventGatewayResponse, error) {
	for _, gateway := range s.gateways {
		if gateway.ID == gatewayID {
			return &kkOps.GetEventGatewayResponse{StatusCode: 200, EventGatewayInfo: &gateway}, nil
		}
	}
	return &kkOps.GetEventGatewayResponse{StatusCode: 404}, nil
}

func (s *stubExternalEventGatewayControlPlaneAPI) CreateEGWControlPlane(
	_ context.Context,
	_ kkComps.CreateGatewayRequest,
	_ ...kkOps.Option,
) (*kkOps.CreateEventGatewayResponse, error) {
	return nil, errors.New("unexpected create event gateway")
}

func (s *stubExternalEventGatewayControlPlaneAPI) UpdateEGWControlPlane(
	_ context.Context,
	_ string,
	_ kkComps.UpdateGatewayRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateEventGatewayResponse, error) {
	return nil, errors.New("unexpected update event gateway")
}

func (s *stubExternalEventGatewayControlPlaneAPI) DeleteEGWControlPlane(
	_ context.Context,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DeleteEventGatewayResponse, error) {
	return nil, errors.New("unexpected delete event gateway")
}

type stubExternalEventGatewayBackendClusterAPI struct {
	clusters []kkComps.BackendCluster
}

func (s *stubExternalEventGatewayBackendClusterAPI) ListEventGatewayBackendClusters(
	_ context.Context,
	_ kkOps.ListEventGatewayBackendClustersRequest,
	_ ...kkOps.Option,
) (*kkOps.ListEventGatewayBackendClustersResponse, error) {
	return &kkOps.ListEventGatewayBackendClustersResponse{
		StatusCode: 200,
		ListBackendClustersResponse: &kkComps.ListBackendClustersResponse{
			Meta: eventGatewayExternalCursorMeta(),
			Data: s.clusters,
		},
	}, nil
}

func (s *stubExternalEventGatewayBackendClusterAPI) FetchEventGatewayBackendCluster(
	_ context.Context,
	_ string,
	clusterID string,
	_ ...kkOps.Option,
) (*kkOps.GetEventGatewayBackendClusterResponse, error) {
	for _, cluster := range s.clusters {
		if cluster.ID == clusterID {
			return &kkOps.GetEventGatewayBackendClusterResponse{StatusCode: 200, BackendCluster: &cluster}, nil
		}
	}
	return &kkOps.GetEventGatewayBackendClusterResponse{StatusCode: 404}, nil
}

func (s *stubExternalEventGatewayBackendClusterAPI) CreateEventGatewayBackendCluster(
	_ context.Context,
	_ string,
	_ kkComps.CreateBackendClusterRequest,
	_ ...kkOps.Option,
) (*kkOps.CreateEventGatewayBackendClusterResponse, error) {
	return nil, errors.New("unexpected create backend cluster")
}

func (s *stubExternalEventGatewayBackendClusterAPI) UpdateEventGatewayBackendCluster(
	_ context.Context,
	_ string,
	_ string,
	_ kkComps.UpdateBackendClusterRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateEventGatewayBackendClusterResponse, error) {
	return nil, errors.New("unexpected update backend cluster")
}

func (s *stubExternalEventGatewayBackendClusterAPI) DeleteEventGatewayBackendCluster(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DeleteEventGatewayBackendClusterResponse, error) {
	return nil, errors.New("unexpected delete backend cluster")
}

type stubExternalEventGatewayVirtualClusterAPI struct {
	clusters []kkComps.VirtualCluster
}

func (s *stubExternalEventGatewayVirtualClusterAPI) ListEventGatewayVirtualClusters(
	_ context.Context,
	_ kkOps.ListEventGatewayVirtualClustersRequest,
	_ ...kkOps.Option,
) (*kkOps.ListEventGatewayVirtualClustersResponse, error) {
	return &kkOps.ListEventGatewayVirtualClustersResponse{
		StatusCode: 200,
		ListVirtualClustersResponse: &kkComps.ListVirtualClustersResponse{
			Meta: eventGatewayExternalCursorMeta(),
			Data: s.clusters,
		},
	}, nil
}

func (s *stubExternalEventGatewayVirtualClusterAPI) FetchEventGatewayVirtualCluster(
	_ context.Context,
	_ string,
	clusterID string,
	_ ...kkOps.Option,
) (*kkOps.GetEventGatewayVirtualClusterResponse, error) {
	for _, cluster := range s.clusters {
		if cluster.ID == clusterID {
			return &kkOps.GetEventGatewayVirtualClusterResponse{StatusCode: 200, VirtualCluster: &cluster}, nil
		}
	}
	return &kkOps.GetEventGatewayVirtualClusterResponse{StatusCode: 404}, nil
}

func (s *stubExternalEventGatewayVirtualClusterAPI) CreateEventGatewayVirtualCluster(
	_ context.Context,
	_ string,
	_ kkComps.CreateVirtualClusterRequest,
	_ ...kkOps.Option,
) (*kkOps.CreateEventGatewayVirtualClusterResponse, error) {
	return nil, errors.New("unexpected create virtual cluster")
}

func (s *stubExternalEventGatewayVirtualClusterAPI) UpdateEventGatewayVirtualCluster(
	_ context.Context,
	_ string,
	_ string,
	_ kkComps.UpdateVirtualClusterRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateEventGatewayVirtualClusterResponse, error) {
	return nil, errors.New("unexpected update virtual cluster")
}

func (s *stubExternalEventGatewayVirtualClusterAPI) DeleteEventGatewayVirtualCluster(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DeleteEventGatewayVirtualClusterResponse, error) {
	return nil, errors.New("unexpected delete virtual cluster")
}

type stubExternalEventGatewayClusterPolicyAPI struct {
	policies []kkComps.EventGatewayPolicy
}

func (s *stubExternalEventGatewayClusterPolicyAPI) ListEventGatewayVirtualClusterClusterLevelPolicies(
	_ context.Context,
	_ kkOps.ListEventGatewayVirtualClusterClusterLevelPoliciesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListEventGatewayVirtualClusterClusterLevelPoliciesResponse, error) {
	return &kkOps.ListEventGatewayVirtualClusterClusterLevelPoliciesResponse{
		StatusCode:                  200,
		ListClusterPoliciesResponse: s.policies,
	}, nil
}

func (s *stubExternalEventGatewayClusterPolicyAPI) GetEventGatewayVirtualClusterClusterLevelPolicy(
	_ context.Context,
	_ kkOps.GetEventGatewayVirtualClusterClusterLevelPolicyRequest,
	_ ...kkOps.Option,
) (*kkOps.GetEventGatewayVirtualClusterClusterLevelPolicyResponse, error) {
	return nil, errors.New("unexpected get cluster policy")
}

func (s *stubExternalEventGatewayClusterPolicyAPI) CreateEventGatewayVirtualClusterClusterLevelPolicy(
	_ context.Context,
	_ kkOps.CreateEventGatewayVirtualClusterClusterLevelPolicyRequest,
	_ ...kkOps.Option,
) (*kkOps.CreateEventGatewayVirtualClusterClusterLevelPolicyResponse, error) {
	return nil, errors.New("unexpected create cluster policy")
}

func (s *stubExternalEventGatewayClusterPolicyAPI) UpdateEventGatewayVirtualClusterClusterLevelPolicy(
	_ context.Context,
	_ kkOps.UpdateEventGatewayVirtualClusterClusterLevelPolicyRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateEventGatewayVirtualClusterClusterLevelPolicyResponse, error) {
	return nil, errors.New("unexpected update cluster policy")
}

func (s *stubExternalEventGatewayClusterPolicyAPI) DeleteEventGatewayVirtualClusterClusterLevelPolicy(
	_ context.Context,
	_ kkOps.DeleteEventGatewayVirtualClusterClusterLevelPolicyRequest,
	_ ...kkOps.Option,
) (*kkOps.DeleteEventGatewayVirtualClusterClusterLevelPolicyResponse, error) {
	return nil, errors.New("unexpected delete cluster policy")
}

var (
	_ helpers.EGWControlPlaneAPI            = (*stubExternalEventGatewayControlPlaneAPI)(nil)
	_ helpers.EventGatewayBackendClusterAPI = (*stubExternalEventGatewayBackendClusterAPI)(nil)
	_ helpers.EventGatewayVirtualClusterAPI = (*stubExternalEventGatewayVirtualClusterAPI)(nil)
	_ helpers.EventGatewayClusterPolicyAPI  = (*stubExternalEventGatewayClusterPolicyAPI)(nil)
)

func TestPlanner_ExternalEventGateway_PlansExternalVirtualClusterChildren(t *testing.T) {
	ctx := context.Background()
	gatewayID := "gateway-123"
	virtualClusterID := "vc-123"

	client := state.NewClient(state.ClientConfig{
		EGWControlPlaneAPI: &stubExternalEventGatewayControlPlaneAPI{
			gateways: []kkComps.EventGatewayInfo{
				{ID: gatewayID, Name: "external-egw"},
			},
		},
		EventGatewayBackendClusterAPI: &stubExternalEventGatewayBackendClusterAPI{},
		EventGatewayVirtualClusterAPI: &stubExternalEventGatewayVirtualClusterAPI{
			clusters: []kkComps.VirtualCluster{
				{ID: virtualClusterID, Name: "external-vc"},
			},
		},
		EventGatewayClusterPolicyAPI: &stubExternalEventGatewayClusterPolicyAPI{},
	})

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(ctx, externalEventGatewayResourceSet(), Options{
		Mode: PlanModeApply,
	})
	require.NoError(t, err)

	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypeEventGatewayClusterPolicy, change.ResourceType)
	require.Equal(t, "cluster-policy-ref", change.ResourceRef)
	require.Equal(t, resources.NamespaceExternal, change.Namespace)
	require.NotNil(t, change.Parent)
	require.Equal(t, "virtual-cluster-ref", change.Parent.Ref)
	require.Equal(t, virtualClusterID, change.Parent.ID)
	require.Equal(t, gatewayID, change.References[FieldEventGatewayID].ID)
	require.Equal(t, virtualClusterID, change.References[FieldEventGatewayVirtualClusterID].ID)
}

func TestPlanner_ExternalEventGateway_SyncDoesNotDeleteExistingBackendClusters(t *testing.T) {
	t.Parallel()

	planner := &Planner{
		client: state.NewClient(state.ClientConfig{
			EventGatewayBackendClusterAPI: &stubExternalEventGatewayBackendClusterAPI{
				clusters: []kkComps.BackendCluster{{ID: "backend-123", Name: "backend"}},
			},
		}),
		logger: slog.Default(),
		resources: &resources.ResourceSet{
			EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{
				externalEventGatewayResource(),
			},
		},
	}

	plan := NewPlan("1.0", "test", PlanModeSync)
	err := planner.planEventGatewayBackendClusterChanges(
		context.Background(),
		nil,
		resources.NamespaceExternal,
		"external-egw",
		"gateway-123",
		"event-gateway-ref",
		"",
		nil,
		plan,
	)
	require.NoError(t, err)
	requireNoDeleteChange(t, plan, ResourceTypeEventGatewayBackendCluster)
}

func TestPlanner_ExternalEventGateway_SyncDoesNotDeleteExistingVirtualClusters(t *testing.T) {
	t.Parallel()

	planner := &Planner{
		client: state.NewClient(state.ClientConfig{
			EventGatewayVirtualClusterAPI: &stubExternalEventGatewayVirtualClusterAPI{
				clusters: []kkComps.VirtualCluster{{ID: "vc-123", Name: "virtual-cluster"}},
			},
		}),
		logger: slog.Default(),
		resources: &resources.ResourceSet{
			EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{
				externalEventGatewayResource(),
			},
		},
	}

	plan := NewPlan("1.0", "test", PlanModeSync)
	err := planner.planEventGatewayVirtualClusterChanges(
		context.Background(),
		nil,
		resources.NamespaceExternal,
		"external-egw",
		"gateway-123",
		"event-gateway-ref",
		"",
		nil,
		plan,
	)
	require.NoError(t, err)
	requireNoDeleteChange(t, plan, ResourceTypeEventGatewayVirtualCluster)
}

func TestPlanner_ExternalEventGatewayVirtualCluster_SyncDoesNotDeleteExistingClusterPolicies(t *testing.T) {
	t.Parallel()

	policyName := "external-policy"
	planner := &Planner{
		client: state.NewClient(state.ClientConfig{
			EventGatewayClusterPolicyAPI: &stubExternalEventGatewayClusterPolicyAPI{
				policies: []kkComps.EventGatewayPolicy{
					{ID: "policy-123", Name: &policyName, Type: "acls"},
				},
			},
		}),
		logger: slog.Default(),
		resources: &resources.ResourceSet{
			EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{
				externalEventGatewayResourceWithVirtualCluster(),
			},
			EventGatewayVirtualClusters: []resources.EventGatewayVirtualClusterResource{
				externalEventGatewayVirtualClusterResource(),
			},
		},
	}

	plan := NewPlan("1.0", "test", PlanModeSync)
	err := planner.planEventGatewayClusterPolicyChanges(
		context.Background(),
		nil,
		resources.NamespaceExternal,
		"gateway-123",
		"event-gateway-ref",
		"external-vc",
		"vc-123",
		"virtual-cluster-ref",
		"",
		nil,
		plan,
	)
	require.NoError(t, err)
	requireNoDeleteChange(t, plan, ResourceTypeEventGatewayClusterPolicy)
}

func externalEventGatewayResourceSet() *resources.ResourceSet {
	name := "cluster-policy"
	enabled := true
	return &resources.ResourceSet{
		EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{
			{
				BaseResource:         resources.BaseResource{Ref: "event-gateway-ref"},
				CreateGatewayRequest: kkComps.CreateGatewayRequest{Name: "external-egw"},
				External: &resources.ExternalBlock{
					Selector: &resources.ExternalSelector{MatchFields: map[string]string{"name": "external-egw"}},
				},
				VirtualClusters: []resources.EventGatewayVirtualClusterResource{
					{
						Ref:                         "virtual-cluster-ref",
						CreateVirtualClusterRequest: kkComps.CreateVirtualClusterRequest{Name: "external-vc"},
						External: &resources.ExternalBlock{
							Selector: &resources.ExternalSelector{MatchFields: map[string]string{"name": "external-vc"}},
						},
						ClusterPolicies: []resources.EventGatewayClusterPolicyResource{
							{
								Ref: "cluster-policy-ref",
								EventGatewayClusterPolicyModify: kkComps.CreateEventGatewayClusterPolicyModifyAcls(
									kkComps.EventGatewayACLsPolicy{
										Name:    &name,
										Enabled: &enabled,
										Config:  kkComps.EventGatewayACLPolicyConfig{},
									},
								),
							},
						},
					},
				},
			},
		},
	}
}

func externalEventGatewayResource() resources.EventGatewayControlPlaneResource {
	return resources.EventGatewayControlPlaneResource{
		BaseResource:         resources.BaseResource{Ref: "event-gateway-ref"},
		CreateGatewayRequest: kkComps.CreateGatewayRequest{Name: "external-egw"},
		External:             &resources.ExternalBlock{ID: "gateway-123"},
	}
}

func externalEventGatewayResourceWithVirtualCluster() resources.EventGatewayControlPlaneResource {
	gateway := externalEventGatewayResource()
	gateway.VirtualClusters = []resources.EventGatewayVirtualClusterResource{
		externalEventGatewayVirtualClusterResource(),
	}
	return gateway
}

func externalEventGatewayVirtualClusterResource() resources.EventGatewayVirtualClusterResource {
	return resources.EventGatewayVirtualClusterResource{
		Ref:                         "virtual-cluster-ref",
		CreateVirtualClusterRequest: kkComps.CreateVirtualClusterRequest{Name: "external-vc"},
		External:                    &resources.ExternalBlock{ID: "vc-123"},
	}
}

func requireNoDeleteChange(t *testing.T, plan *Plan, resourceType string) {
	t.Helper()

	for _, change := range plan.Changes {
		if change.ResourceType == resourceType && change.Action == ActionDelete {
			t.Fatalf("unexpected %s delete planned for external event gateway: %+v", resourceType, change)
		}
	}
}

func eventGatewayExternalCursorMeta() *kkComps.CursorMeta {
	return &kkComps.CursorMeta{
		Page: kkComps.CursorMetaPage{},
	}
}
