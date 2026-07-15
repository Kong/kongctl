package planner

import (
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestEventGatewayByNameDependenciesProduceSafeExecutionGroups(t *testing.T) {
	backendCluster := resources.EventGatewayBackendClusterResource{
		Ref: "backend",
		CreateBackendClusterRequest: components.CreateBackendClusterRequest{
			Name: "backend-name",
		},
	}
	virtualCluster := resources.EventGatewayVirtualClusterResource{
		Ref: "virtual",
		CreateVirtualClusterRequest: components.CreateVirtualClusterRequest{
			Name: "virtual-name",
			Destination: components.CreateBackendClusterReferenceModifyBackendClusterReferenceByName(
				components.BackendClusterReferenceByName{Name: backendCluster.Name},
			),
		},
	}
	resourceSet := &resources.ResourceSet{
		EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{{
			BaseResource:    resources.BaseResource{Ref: "gateway"},
			BackendClusters: []resources.EventGatewayBackendClusterResource{backendCluster},
			VirtualClusters: []resources.EventGatewayVirtualClusterResource{virtualCluster},
		}},
	}
	planner := &Planner{logger: slog.Default(), resources: resourceSet}
	plan := NewPlan("1.0", "test", PlanModeApply)

	gatewayID := "gateway-create"
	plan.AddChange(PlannedChange{
		ID:           gatewayID,
		ResourceType: ResourceTypeEventGatewayControlPlane,
		ResourceRef:  "gateway",
		Action:       ActionCreate,
		Fields:       map[string]any{FieldName: "gateway-name"},
	})
	planner.planBackendClusterCreate(
		"default", "gateway", "gateway-name", "", backendCluster, []string{gatewayID}, plan,
	)
	listenerID := planner.planListenerCreate(
		"default",
		"gateway",
		"gateway-name",
		"",
		resources.EventGatewayListenerResource{Ref: "listener"},
		[]string{gatewayID},
		plan,
	)
	virtualClusterID := planner.planVirtualClusterCreate(
		"default", "gateway", "gateway-name", "", virtualCluster, []string{gatewayID}, plan,
	)

	var policy resources.EventGatewayListenerPolicyResource
	require.NoError(t, json.Unmarshal([]byte(`{
		"ref":"policy",
		"name":"policy-name",
		"type":"forward_to_virtual_cluster",
		"config":{
			"type":"port_mapping",
			"advertised_host":"example.com",
			"destination":{"name":"virtual-name"}
		}
	}`), &policy))
	planner.planListenerPolicyCreate(
		"default", "", "gateway", "", "listener", "listener-name", policy, []string{listenerID}, plan,
	)

	backendClusterID := plannedChangeID(t, plan, ResourceTypeEventGatewayBackendCluster, "backend")
	policyID := plannedChangeID(t, plan, ResourceTypeEventGatewayListenerPolicy, "policy")

	result, err := NewDependencyResolver().ResolveDependenciesWithGroups(plan.Changes)
	require.NoError(t, err)
	require.Equal(t, [][]string{
		{gatewayID},
		{backendClusterID, listenerID},
		{virtualClusterID},
		{policyID},
	}, result.ExecutionGroups)
	require.ElementsMatch(t, []string{gatewayID, backendClusterID}, result.FullDepsMap[virtualClusterID])
	require.ElementsMatch(t, []string{gatewayID, listenerID, virtualClusterID}, result.FullDepsMap[policyID])

	backendRef := planChange(t, plan, virtualClusterID).References[FieldEventGatewayBackendClusterID]
	require.Equal(t, "backend", backendRef.Ref)
	require.Equal(t, resources.UnknownReferenceID, backendRef.ID)
	virtualRef := planChange(t, plan, policyID).References[FieldEventGatewayVirtualClusterID]
	require.Equal(t, "virtual", virtualRef.Ref)
	require.Equal(t, resources.UnknownReferenceID, virtualRef.ID)
}

func TestEventGatewayByNameDependenciesAreScopedToGateway(t *testing.T) {
	resourceSet := &resources.ResourceSet{
		EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{
			{
				BaseResource: resources.BaseResource{Ref: "gateway-a"},
				VirtualClusters: []resources.EventGatewayVirtualClusterResource{{
					Ref: "virtual-a",
				}},
			},
			{
				BaseResource: resources.BaseResource{Ref: "gateway-b"},
				BackendClusters: []resources.EventGatewayBackendClusterResource{{
					Ref: "backend-b",
					CreateBackendClusterRequest: components.CreateBackendClusterRequest{
						Name: "shared-name",
					},
				}},
				VirtualClusters: []resources.EventGatewayVirtualClusterResource{{
					Ref: "virtual-b",
					CreateVirtualClusterRequest: components.CreateVirtualClusterRequest{
						Name: "shared-virtual-name",
					},
				}},
			},
		},
	}
	planner := &Planner{logger: slog.Default(), resources: resourceSet}
	plan := NewPlan("1.0", "test", PlanModeApply)
	plan.AddChange(PlannedChange{
		ID:           "backend-create",
		ResourceType: ResourceTypeEventGatewayBackendCluster,
		ResourceRef:  "backend-b",
		Action:       ActionCreate,
	})
	plan.AddChange(PlannedChange{
		ID:           "virtual-create",
		ResourceType: ResourceTypeEventGatewayVirtualCluster,
		ResourceRef:  "virtual-b",
		Action:       ActionCreate,
	})

	virtualClusterID := planner.planVirtualClusterCreate(
		"default",
		"gateway-a",
		"gateway-a-name",
		"gateway-a-id",
		resources.EventGatewayVirtualClusterResource{
			Ref: "virtual-a",
			CreateVirtualClusterRequest: components.CreateVirtualClusterRequest{
				Name: "virtual-a",
				Destination: components.CreateBackendClusterReferenceModifyBackendClusterReferenceByName(
					components.BackendClusterReferenceByName{Name: "shared-name"},
				),
			},
		},
		nil,
		plan,
	)

	require.NotContains(
		t,
		planChange(t, plan, virtualClusterID).References,
		FieldEventGatewayBackendClusterID,
	)

	var policy resources.EventGatewayListenerPolicyResource
	require.NoError(t, json.Unmarshal([]byte(`{
		"ref":"policy-a",
		"name":"policy-a",
		"type":"forward_to_virtual_cluster",
		"config":{
			"type":"port_mapping",
			"advertised_host":"example.com",
			"destination":{"name":"shared-virtual-name"}
		}
	}`), &policy))
	planner.planListenerPolicyCreate(
		"default", "gateway-a-id", "gateway-a", "listener-id", "listener", "listener", policy, nil, plan,
	)
	policyID := plannedChangeID(t, plan, ResourceTypeEventGatewayListenerPolicy, "policy-a")
	require.NotContains(t, planChange(t, plan, policyID).References, FieldEventGatewayVirtualClusterID)
}

func TestEventGatewayExplicitRefsRemainDependencies(t *testing.T) {
	backendCluster := resources.EventGatewayBackendClusterResource{
		Ref: "backend",
		CreateBackendClusterRequest: components.CreateBackendClusterRequest{
			Name: "backend-name",
		},
	}
	resourceSet := &resources.ResourceSet{
		EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{{
			BaseResource:    resources.BaseResource{Ref: "gateway"},
			BackendClusters: []resources.EventGatewayBackendClusterResource{backendCluster},
		}},
	}
	planner := &Planner{logger: slog.Default(), resources: resourceSet}
	plan := NewPlan("1.0", "test", PlanModeApply)
	planner.planBackendClusterCreate("default", "gateway", "gateway", "gateway-id", backendCluster, nil, plan)

	virtualClusterID := planner.planVirtualClusterCreate(
		"default",
		"gateway",
		"gateway",
		"gateway-id",
		resources.EventGatewayVirtualClusterResource{
			Ref: "virtual",
			CreateVirtualClusterRequest: components.CreateVirtualClusterRequest{
				Name: "virtual",
				Destination: components.CreateBackendClusterReferenceModifyBackendClusterReferenceByID(
					components.BackendClusterReferenceByID{ID: "__REF__:backend#id"},
				),
			},
		},
		nil,
		plan,
	)

	backendRef := planChange(t, plan, virtualClusterID).References[FieldEventGatewayBackendClusterID]
	require.Equal(t, "__REF__:backend#id", backendRef.Ref)
	require.Equal(t, resources.UnknownReferenceID, backendRef.ID)
	require.Equal(t, "backend-name", backendRef.LookupFields[FieldName])
}

func plannedChangeID(t *testing.T, plan *Plan, resourceType, resourceRef string) string {
	t.Helper()
	for _, change := range plan.Changes {
		if change.ResourceType == resourceType && change.ResourceRef == resourceRef {
			return change.ID
		}
	}
	t.Fatalf("planned change not found: %s %s", resourceType, resourceRef)
	return ""
}

func planChange(t *testing.T, plan *Plan, id string) PlannedChange {
	t.Helper()
	for _, change := range plan.Changes {
		if change.ID == id {
			return change
		}
	}
	t.Fatalf("planned change not found: %s", id)
	return PlannedChange{}
}
