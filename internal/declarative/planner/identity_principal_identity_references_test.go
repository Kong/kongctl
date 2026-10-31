package planner

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestIdentityReferencesOrderDeletesAndRequireProvenRetargeting(t *testing.T) {
	for _, action := range []string{"delete", "retarget", "metadata-only", "retained", "new-binding"} {
		t.Run(action, func(t *testing.T) {
			const directory = "731559d3-39bd-48c7-b33c-eb192847eb20"
			const principal = "731559d3-39bd-48c7-b33c-eb192847eb21"
			const id = "731559d3-39bd-48c7-b33c-eb192847eb22"
			p := &Planner{resourceCache: newPlanningResourceCache()}
			p.resourceCache.identityPrincipalIdentities[directory+"/"+principal] = []helpers.PrincipalIdentity{{
				ID: id, Type: "auth_server_client", AuthServerID: new(directory), ClientID: new("old-client"),
			}}
			target := PlannedChange{
				ID: "client-delete", ResourceType: ResourceTypeIdentityAuthServerClient,
				ResourceRef: "client", ResourceID: "old-client", Action: ActionDelete, Parent: &ParentInfo{ID: directory},
			}
			child := PlannedChange{
				ID: "identity-change", ResourceType: ResourceTypeIdentityPrincipalIdentity,
				ResourceRef: "worker", ResourceID: id, Parent: &ParentInfo{ID: principal}, Action: ActionUpdate,
				References: map[string]ReferenceInfo{FieldDirectoryID: {ID: directory}},
				Fields: map[string]any{
					FieldType:         "auth_server_client",
					FieldAuthServerID: directory,
					FieldClientID:     "old-client",
				},
			}
			switch action {
			case "delete":
				child.Action = ActionDelete
			case "retarget":
				child.Fields[FieldClientID] = "new-client"
			case "new-binding":
				child.Action = ActionCreate
				child.ResourceID = ""
				p.resourceCache.identityPrincipalIdentities[directory+"/"+principal] = nil
			}
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
			plan.Changes = []PlannedChange{target}
			if action != "retained" {
				plan.Changes = append(plan.Changes, child)
			}
			err := p.resolvePrincipalIdentityTargetDeletes(plan)
			if action == "delete" || action == "retarget" {
				require.NoError(t, err)
				require.Contains(t, plan.Changes[0].DependsOn, child.ID)
				require.Equal(
					t,
					[]IdentityReferenceScope{{DirectoryID: directory, PrincipalID: principal}},
					plan.Changes[0].IdentityReferenceScopes,
				)
			} else {
				require.Error(t, err)
			}
			require.Len(t, plan.Warnings, 1)
		})
	}
}

func TestIdentityAssociationTargetsIncludeGatewayAndClientParents(t *testing.T) {
	client := resources.IdentityPrincipalIdentityResource{
		Type:         "auth_server_client",
		AuthServerID: new("server"),
		ClientID:     new("client"),
	}
	require.True(t, client.ReferencesTarget(resources.ResourceTypeIdentityAuthServer, "server", ""))
	require.True(t, client.ReferencesTarget(resources.ResourceTypeIdentityAuthServerClient, "client", "server"))
	require.False(t, client.ReferencesTarget(resources.ResourceTypeIdentityAuthServerClient, "client", "other-server"))
	consumer := resources.IdentityPrincipalIdentityResource{
		Type:           "control_plane_consumer",
		ControlPlaneID: new("gateway"),
		ConsumerID:     new("consumer"),
	}
	require.True(t, consumer.ReferencesTarget(resources.ResourceTypeControlPlane, "gateway", ""))
	require.True(t, consumer.ReferencesTarget(resources.ResourceTypeAIGateway, "gateway", ""))
	require.True(t, consumer.ReferencesTarget(resources.ResourceTypeAIGatewayConsumer, "consumer", "gateway"))
	require.False(t, consumer.ReferencesTarget(resources.ResourceTypeAIGatewayConsumer, "consumer", "other-gateway"))
}
