package planner

import (
	"encoding/json"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestPayloadBinderPreservesExistingEventGatewayBackendReference(t *testing.T) {
	for _, action := range []ActionType{ActionCreate, ActionUpdate} {
		t.Run(string(action), func(t *testing.T) {
			backend := resources.EventGatewayBackendClusterResource{
				Ref: "backend-ref", EventGateway: "event-gateway-ref",
				CreateBackendClusterRequest: kkComps.CreateBackendClusterRequest{Name: "backend-cluster"},
			}
			backend.Authentication.Type = kkComps.BackendClusterAuthenticationSchemeTypeAnonymous
			remoteBackend := kkComps.BackendCluster{ID: "backend-cluster-id", Name: "backend-cluster"}
			remoteBackend.Authentication.Type = kkComps.BackendClusterAuthenticationSensitiveDataAwareSchemeTypeAnonymous
			virtual := virtualClusterResource(nil)
			virtual.Description = new("updated description")
			virtual.Destination.BackendClusterReferenceByID.ID = "__REF__:backend-ref#id"
			gateway := externalEventGatewayResource()
			gateway.VirtualClusters = []resources.EventGatewayVirtualClusterResource{virtual}
			rs := &resources.ResourceSet{
				EventGatewayControlPlanes:   []resources.EventGatewayControlPlaneResource{gateway},
				EventGatewayBackendClusters: []resources.EventGatewayBackendClusterResource{backend},
			}
			remote := &stubExternalEventGatewayVirtualClusterAPI{}
			if action == ActionUpdate {
				remote.clusters = []kkComps.VirtualCluster{virtualClusterState(nil).VirtualCluster}
			}
			client := state.NewClient(state.ClientConfig{
				EGWControlPlaneAPI: &stubExternalEventGatewayControlPlaneAPI{
					gateways: []kkComps.EventGatewayInfo{{ID: "gateway-123", Name: "external-egw"}},
				},
				EventGatewayBackendClusterAPI: &stubExternalEventGatewayBackendClusterAPI{
					clusters: []kkComps.BackendCluster{remoteBackend},
				},
				EventGatewayVirtualClusterAPI: remote,
			})
			plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
			require.NoError(t, err)
			require.Len(t, plan.Changes, 1)
			change := plan.Changes[0]
			require.Equal(t, action, change.Action)
			require.Equal(t, ResourceTypeEventGatewayVirtualCluster, change.ResourceType)
			require.Empty(t, change.PayloadReferences)
			require.Equal(t, "backend-cluster", change.References[FieldEventGatewayBackendClusterID].LookupFields[FieldName])
			// An unchanged backend has no CREATE and no ID cached on its declaration.
			require.Empty(t, rs.EventGatewayBackendClusters[0].GetKonnectID())
			data, err := json.Marshal(plan)
			require.NoError(t, err)
			var saved Plan
			require.NoError(t, json.Unmarshal(data, &saved))
			require.Equal(t, change.References, saved.Changes[0].References)
		})
	}
}

func TestPayloadBinderPreservesNestedProducePolicySchemaRegistryReference(t *testing.T) {
	policy := producePolicyResourceFromJSON(t, `{
  "ref": "policy", "name": "policy", "type": "schema_validation",
  "config": {
    "type": "confluent_schema_registry", "key_validation_action": "mark",
    "value_validation_action": "mark", "schema_registry": {"id": "__REF__:registry#id"}
  }
}`)
	gateway := externalEventGatewayResource()
	gateway.SetKonnectID("gateway-123")
	gateway.VirtualClusters = []resources.EventGatewayVirtualClusterResource{{
		Ref: "virtual", ProducePolicies: []resources.EventGatewayProducePolicyResource{policy},
	}}
	rs := &resources.ResourceSet{
		EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{gateway},
		EventGatewaySchemaRegistries: []resources.EventGatewaySchemaRegistryResource{{
			Ref: "registry", EventGateway: gateway.Ref,
			SchemaRegistryCreate: kkComps.CreateSchemaRegistryCreateConfluent(
				kkComps.SchemaRegistryConfluent{Name: "remote-registry"},
			),
		}},
	}
	client := state.NewClient(state.ClientConfig{
		EventGatewaySchemaRegistryAPI: &schemaRegistryAPIForResolver{
			gatewayID:  "gateway-123",
			registries: []kkComps.SchemaRegistry{{ID: "registry-id", Name: "remote-registry"}},
		},
	})
	p := NewPlanner(client, slog.Default())
	p.resources = rs
	p.resolver = NewReferenceResolver(client, rs)
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
	p.planProducePolicyUpdate("default", "gateway-123", gateway.Ref, "virtual-id", "virtual", "policy-id",
		policy, map[string]any{
			FieldConfig: map[string]any{"schema_registry": map[string]any{FieldID: "__REF__:registry#id"}},
		}, nil, plan)
	require.NoError(t, p.bindPayloadReferences(plan, rs))
	require.Empty(t, plan.Changes[0].PayloadReferences)
	require.Equal(t, "remote-registry", plan.Changes[0].References["config.schema_registry.id"].LookupFields[FieldName])
	resolved, err := p.resolver.ResolveReferences(t.Context(), plan.Changes)
	require.NoError(t, err)
	require.Empty(t, resolved.Errors)
	require.Equal(t, "registry-id", resolved.ChangeReferences[plan.Changes[0].ID]["config.schema_registry.id"].ID)
}

func TestPayloadBinderCreatesDependencyOnNestedEventGatewayTarget(t *testing.T) {
	rs := &resources.ResourceSet{
		EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{{
			BaseResource:    resources.BaseResource{Ref: "gateway"},
			VirtualClusters: []resources.EventGatewayVirtualClusterResource{{Ref: "virtual"}},
		}},
		AIGatewayPolicies: []resources.AIGatewayPolicyResource{{BaseResource: resources.BaseResource{Ref: "policy"}}},
	}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
	plan.AddChange(PlannedChange{
		ID: "create-virtual", ResourceType: ResourceTypeEventGatewayVirtualCluster,
		ResourceRef: "virtual", Action: ActionCreate,
	})
	plan.AddChange(PlannedChange{
		ID: "create-policy", ResourceType: ResourceTypeAIGatewayPolicy,
		ResourceRef: "policy", Action: ActionCreate,
		Fields: map[string]any{FieldConfig: map[string]any{"target": "__REF__:virtual#id"}},
	})
	require.NoError(t, (&Planner{}).bindPayloadReferences(plan, rs))
	require.Equal(t, []string{"create-virtual"}, plan.Changes[1].DependsOn)
	require.Equal(t, []PayloadReference{{
		Path: "/config/target", ResourceType: ResourceTypeEventGatewayVirtualCluster, Ref: "virtual", Selector: FieldID,
	}}, plan.Changes[1].PayloadReferences)
}
