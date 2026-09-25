package planner

import (
	"log/slog"
	"testing"

	"github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestPlannerEventGatewayChildPruning(t *testing.T) {
	for _, mode := range []PlanMode{PlanModeSync, PlanModeApply} {
		t.Run(string(mode), func(t *testing.T) {
			gateway := externalEventGatewayResource()
			rs := &resources.ResourceSet{
				EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{gateway},
			}
			rs.EnsureSyncScope().AddRoot(resources.ResourceTypeEventGatewayControlPlane)
			for _, child := range []resources.ResourceType{
				resources.ResourceTypeEventGatewayBackendCluster, resources.ResourceTypeEventGatewayVirtualCluster,
			} {
				rs.EnsureSyncScope().AddChild(resources.ResourceTypeEventGatewayControlPlane, gateway.Ref, child)
			}
			client := state.NewClient(state.ClientConfig{
				EGWControlPlaneAPI: &stubExternalEventGatewayControlPlaneAPI{
					gateways: []components.EventGatewayInfo{{ID: "gateway-id", Name: "external-egw"}},
				},
				EventGatewayBackendClusterAPI: &stubExternalEventGatewayBackendClusterAPI{
					clusters: []components.BackendCluster{{ID: "backend-id", Name: "backend"}},
				},
				EventGatewayVirtualClusterAPI: &stubExternalEventGatewayVirtualClusterAPI{
					clusters: []components.VirtualCluster{
						{ID: "virtual-one", Name: "one", Destination: components.BackendClusterReference{ID: "backend-id"}},
						{ID: "virtual-two", Name: "two", Destination: components.BackendClusterReference{ID: "backend-id"}},
					},
				},
			})
			plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: mode})
			require.NoError(t, err)
			if mode == PlanModeApply {
				require.Empty(t, plan.Changes)
				return
			}
			require.Len(t, plan.Changes, 3)
			var backend PlannedChange
			var virtualIDs []string
			for _, change := range plan.Changes {
				require.Equal(t, ActionDelete, change.Action)
				if change.ResourceType == ResourceTypeEventGatewayBackendCluster {
					backend = change
				} else {
					require.Equal(t, "backend-id", change.References[FieldEventGatewayBackendClusterID].ID)
					virtualIDs = append(virtualIDs, change.ID)
				}
			}
			require.ElementsMatch(t, virtualIDs, backend.DependsOn)
			require.Len(t, plan.ExecutionGroups, 2)
			require.ElementsMatch(t, virtualIDs, plan.ExecutionGroups[0])
			require.Equal(t, []string{backend.ID}, plan.ExecutionGroups[1])
		})
	}
}

func TestEventGatewayDeleteDependenciesRespectScope(t *testing.T) {
	for _, test := range []struct {
		name, namespace, gatewayID, backendID string
		action                                ActionType
	}{
		{"other namespace", "other", "gateway", "backend", ActionDelete},
		{"other gateway", "namespace", "other", "backend", ActionDelete},
		{"other backend", "namespace", "gateway", "other", ActionDelete},
		{"update", "namespace", "gateway", "backend", ActionUpdate},
		{"missing destination", "namespace", "gateway", "", ActionDelete},
	} {
		t.Run(test.name, func(t *testing.T) {
			changes := []PlannedChange{
				{
					ID: "backend-delete", ResourceType: ResourceTypeEventGatewayBackendCluster, Action: ActionDelete,
					ResourceID: "backend", Namespace: "namespace", Parent: &ParentInfo{ID: "gateway"},
				},
				{
					ID: "virtual-change", ResourceType: ResourceTypeEventGatewayVirtualCluster, Action: test.action,
					Namespace: test.namespace, Parent: &ParentInfo{ID: test.gatewayID},
					References: map[string]ReferenceInfo{FieldEventGatewayBackendClusterID: {ID: test.backendID}},
				},
			}
			adjustEventGatewayBackendClusterDeleteDependencies(changes)
			require.Empty(t, changes[0].DependsOn)
		})
	}
}
