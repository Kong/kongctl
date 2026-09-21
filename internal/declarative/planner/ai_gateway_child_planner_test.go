package planner

import (
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayChildTraversalParentRoutingAndDependencies(t *testing.T) {
	for _, parent := range []string{"new", "existing", "external"} {
		t.Run(parent, func(t *testing.T) {
			gateway := testAIGatewayResource()
			namespace := "default"
			gatewayAPI := &testAIGatewayAPI{}
			switch parent {
			case "existing":
				gatewayAPI.gateways = []kkComps.AIGateway{testAIGateway()}
			case "external":
				gateway.External = &resources.ExternalBlock{ID: "gateway-id"}
				gateway.SetKonnectID("gateway-id")
				namespace = resources.NamespaceExternal
			}
			policy := testAIGatewayPolicyResource(t)
			policyRefs := []string{tags.RefPlaceholderPrefix + policy.Ref + "#id"}
			consumer := testAIGatewayConsumerResource(t, policyRefs)
			group := testAIGatewayConsumerGroupResource(t, policyRefs)
			group.Consumers = []string{tags.RefPlaceholderPrefix + consumer.Ref + "#name"}
			providerAPI := &nameIdentityProviderAPI{}
			rs := &resources.ResourceSet{
				AIGateways: []resources.AIGatewayResource{gateway},
				AIGatewayProviders: []resources.AIGatewayProviderResource{{
					BaseResource: resources.BaseResource{Ref: "provider-ref"},
					AIGateway:    gateway.Ref, Name: "support-openai", Type: "openai", DisplayName: "OpenAI",
				}},
				AIGatewayPolicies:       []resources.AIGatewayPolicyResource{policy},
				AIGatewayConsumers:      []resources.AIGatewayConsumerResource{consumer},
				AIGatewayConsumerGroups: []resources.AIGatewayConsumerGroupResource{group},
				AIGatewayModels:         []resources.AIGatewayModelResource{testAIGatewayModelResource(t)},
			}
			p := NewPlanner(state.NewClient(state.ClientConfig{
				AIGatewayAPI: gatewayAPI, AIGatewayProvidersAPI: providerAPI,
				AIGatewayPoliciesAPI: &testAIGatewayPolicyAPI{}, AIGatewayConsumersAPI: &testAIGatewayConsumerAPI{},
				AIGatewayConsumerGroupsAPI: &testAIGatewayConsumerGroupAPI{}, AIGatewayModelAPI: &testAIGatewayModelAPI{},
			}), slog.Default())
			p.resources = rs
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)

			require.NoError(t, p.planAIGatewayChanges(t.Context(), &Config{Namespace: namespace}, rs.AIGateways, plan))
			require.Empty(t, plan.Warnings)
			children := plan.Changes
			gatewayChangeID := ""
			if parent == "new" {
				require.Len(t, children, 6)
				require.Equal(t, ResourceTypeAIGateway, children[0].ResourceType)
				gatewayChangeID = children[0].ID
				children = children[1:]
				require.Empty(t, providerAPI.lists)
			} else {
				require.Equal(t, []string{"gateway-id"}, providerAPI.lists)
			}
			wantKinds := []string{
				ResourceTypeAIGatewayProvider, ResourceTypeAIGatewayPolicy,
				ResourceTypeAIGatewayConsumer, ResourceTypeAIGatewayConsumerGroup, ResourceTypeAIGatewayModel,
			}
			require.Len(t, children, len(wantKinds))
			for i, change := range children {
				require.Equal(t, wantKinds[i], change.ResourceType)
				require.Equal(t, ActionCreate, change.Action)
				require.Equal(t, namespace, change.Namespace)
				if parent == "new" {
					require.Nil(t, change.Parent)
					require.Equal(t, gateway.Ref, change.References[FieldAIGatewayID].Ref)
					require.Contains(t, change.DependsOn, gatewayChangeID)
				} else {
					require.Equal(t, &ParentInfo{Ref: gateway.Ref, ID: "gateway-id"}, change.Parent)
				}
			}
			require.Contains(t, children[2].DependsOn, children[1].ID)
			require.Contains(t, children[3].DependsOn, children[1].ID)
			require.Contains(t, children[3].DependsOn, children[2].ID)
			require.Contains(t, children[4].DependsOn, children[0].ID)
		})
	}
}

func TestAIGatewayChildTraversalEmptyScope(t *testing.T) {
	for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
		t.Run(string(mode), func(t *testing.T) {
			gateway := testAIGatewayResource()
			gateway.External = &resources.ExternalBlock{ID: "gateway-id"}
			gateway.SetKonnectID("gateway-id")
			other := gateway
			other.Ref = "other-gateway"
			other.SetKonnectID("other-id")
			scope := resources.NewSyncScope()
			scope.AddChild(resources.ResourceTypeAIGateway, gateway.Ref, resources.ResourceTypeAIGatewayConsumerGroup)
			api := &groupNameIdentityAPI{testAIGatewayConsumerGroupAPI: &testAIGatewayConsumerGroupAPI{}}
			p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayConsumerGroupsAPI: api}), slog.Default())
			p.resources = &resources.ResourceSet{
				AIGateways: []resources.AIGatewayResource{gateway, other}, SyncScope: scope,
			}
			plan := NewPlan(CurrentPlanVersion, "test", mode)

			err := p.planAIGatewayChanges(t.Context(), &Config{Namespace: resources.NamespaceExternal},
				p.resources.AIGateways, plan)

			require.NoError(t, err)
			require.Empty(t, plan.Changes)
			require.Empty(t, plan.Warnings)
			if mode == PlanModeSync {
				require.Equal(t, []string{"gateway-id"}, api.lists)
			} else {
				require.Empty(t, api.lists)
			}
		})
	}
}

func TestAIGatewayChildTraversalSkipsUnresolvedExternalParent(t *testing.T) {
	gateway := testAIGatewayResource()
	gateway.External = &resources.ExternalBlock{ID: "unresolved-id"}
	p := NewPlanner(state.NewClient(state.ClientConfig{}), slog.Default())
	p.resources = &resources.ResourceSet{
		AIGateways:      []resources.AIGatewayResource{gateway},
		AIGatewayModels: []resources.AIGatewayModelResource{testAIGatewayModelResource(t)},
	}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)

	err := p.planAIGatewayChanges(t.Context(), &Config{Namespace: resources.NamespaceExternal},
		p.resources.AIGateways, plan)

	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	require.Len(t, plan.Warnings, 1)
	require.Contains(t, plan.Warnings[0].Message, `external ai_gateway "support-gateway" has no resolved ID`)
}

func TestAIGatewayChildTraversalStopsAfterObservationError(t *testing.T) {
	gateway := testAIGatewayResource()
	gateway.External = &resources.ExternalBlock{ID: "gateway-id"}
	gateway.SetKonnectID("gateway-id")
	groupAPI := &groupNameIdentityAPI{testAIGatewayConsumerGroupAPI: &testAIGatewayConsumerGroupAPI{}}
	p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayConsumerGroupsAPI: groupAPI}), slog.Default())
	p.resources = &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{gateway},
		AIGatewayProviders: []resources.AIGatewayProviderResource{{
			BaseResource: resources.BaseResource{Ref: "provider-ref"}, AIGateway: gateway.Ref, Name: "support-openai",
		}},
		AIGatewayConsumerGroups: []resources.AIGatewayConsumerGroupResource{testAIGatewayConsumerGroupResource(t, nil)},
	}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)

	err := p.planAIGatewayChanges(t.Context(), &Config{Namespace: resources.NamespaceExternal},
		p.resources.AIGateways, plan)

	require.ErrorContains(t, err, "failed to list AI Gateway Model Providers for gateway gateway-id")
	require.Empty(t, groupAPI.lists)
	require.Empty(t, plan.Changes)
}
