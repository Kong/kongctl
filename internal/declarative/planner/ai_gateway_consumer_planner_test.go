package planner

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConsumerPlannerCreatesChildForExistingGateway(t *testing.T) {
	consumer := testAIGatewayConsumerResource(t, nil)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayConsumersAPI: &testAIGatewayConsumerAPI{},
	})
	rs := testAIGatewayConsumerResourceSet(consumer)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayConsumer, change.ResourceType)
	require.Equal(t, "support-user", change.ResourceRef)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
	require.Equal(t, "support-gateway", change.Parent.Ref)
	require.Equal(t, "Support User", change.Fields[FieldDisplayName])
	require.Equal(t, "api-key", change.Fields[FieldType])
}

func TestAIGatewayConsumerPlannerUpdatesExistingConsumer(t *testing.T) {
	consumer := testAIGatewayConsumerResource(t, nil)
	consumer.DisplayName = "Support User Updated"
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayConsumersAPI: &testAIGatewayConsumerAPI{
			consumers: []kkComps.AIGatewayConsumer{
				testAIGatewayConsumer("consumer-id", "support-user", nil),
			},
		},
	})
	rs := testAIGatewayConsumerResourceSet(consumer)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionUpdate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayConsumer, change.ResourceType)
	require.Equal(t, "consumer-id", change.ResourceID)
	require.Equal(t, "Support User Updated", change.Fields[FieldDisplayName])
	require.Contains(t, change.ChangedFields, FieldDisplayName)
}

func TestAIGatewayConsumerPlannerSyncDeletesScopedConsumers(t *testing.T) {
	scope := resources.NewSyncScope()
	scope.AddRoot(resources.ResourceTypeAIGateway)
	scope.AddChild(resources.ResourceTypeAIGateway, "support-gateway", resources.ResourceTypeAIGatewayConsumer)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayConsumersAPI: &testAIGatewayConsumerAPI{
			consumers: []kkComps.AIGatewayConsumer{
				testAIGatewayConsumer("consumer-id", "support-user", nil),
			},
		},
	})
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{testAIGatewayResource()},
		SyncScope:  scope,
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionDelete, change.Action)
	require.Equal(t, ResourceTypeAIGatewayConsumer, change.ResourceType)
	require.Equal(t, "consumer-id", change.ResourceID)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
}

func TestAIGatewayConsumerPlannerDependsOnPolicyCreate(t *testing.T) {
	policy := testAIGatewayPolicyResource(t)
	consumer := testAIGatewayConsumerResource(t, []string{tags.RefPlaceholderPrefix + "mask-sensitive-data#id"})
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{},
	})
	rs := &resources.ResourceSet{
		AIGateways:          []resources.AIGatewayResource{testAIGatewayResource()},
		AIGatewayPolicies:   []resources.AIGatewayPolicyResource{policy},
		AIGatewayConsumers:  []resources.AIGatewayConsumerResource{consumer},
		AIGatewayProviders:  nil,
		AIGatewayModels:     nil,
		AIGatewayMCPServers: nil,
		AIGatewayVaults:     nil,
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)

	gatewayCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGateway, "support-gateway")
	policyCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayPolicy, "mask-sensitive-data")
	consumerCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayConsumer, "support-user")

	require.Contains(t, policyCreate.DependsOn, gatewayCreate.ID)
	require.Contains(t, consumerCreate.DependsOn, policyCreate.ID)
	require.Equal(t, resources.UnknownReferenceID, consumerCreate.References[FieldPolicies+".0"].ID)
	require.Equal(
		t,
		tags.RefPlaceholderPrefix+"mask-sensitive-data#name",
		consumerCreate.References[FieldPolicies+".0"].Ref,
	)
}

func TestAIGatewayConsumerPlannerPolicyRefNoopForExistingConsumer(t *testing.T) {
	for _, currentPolicyRef := range []string{"policy-id", "mask-sensitive-data"} {
		t.Run(currentPolicyRef, func(t *testing.T) {
			policy := testAIGatewayPolicyResource(t)
			consumer := testAIGatewayConsumerResource(t, []string{tags.RefPlaceholderPrefix + "mask-sensitive-data#id"})
			client := state.NewClient(state.ClientConfig{
				AIGatewayAPI: &testAIGatewayAPI{
					gateways: []kkComps.AIGateway{testAIGateway()},
				},
				AIGatewayPoliciesAPI: &testAIGatewayPolicyAPI{
					policies: []kkComps.AIGatewayPolicy{testAIGatewayPolicy()},
				},
				AIGatewayConsumersAPI: &testAIGatewayConsumerAPI{
					consumers: []kkComps.AIGatewayConsumer{
						testAIGatewayConsumer("consumer-id", "support-user", []string{currentPolicyRef}),
					},
				},
			})
			rs := &resources.ResourceSet{
				AIGateways:         []resources.AIGatewayResource{testAIGatewayResource()},
				AIGatewayPolicies:  []resources.AIGatewayPolicyResource{policy},
				AIGatewayConsumers: []resources.AIGatewayConsumerResource{consumer},
			}

			plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
			require.NoError(t, err)
			require.Empty(t, plan.Changes)
		})
	}
}

func testAIGatewayConsumerResourceSet(
	consumer resources.AIGatewayConsumerResource,
) *resources.ResourceSet {
	return &resources.ResourceSet{
		AIGateways:         []resources.AIGatewayResource{testAIGatewayResource()},
		AIGatewayConsumers: []resources.AIGatewayConsumerResource{consumer},
	}
}

func testAIGatewayConsumerResource(
	t *testing.T,
	policies []string,
) resources.AIGatewayConsumerResource {
	t.Helper()
	payload := map[string]any{
		"ref":          "support-user",
		"ai_gateway":   "support-gateway",
		"name":         "support-user",
		"type":         "api-key",
		"display_name": "Support User",
	}
	if policies != nil {
		payload[FieldPolicies] = policies
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var consumer resources.AIGatewayConsumerResource
	require.NoError(t, json.Unmarshal(data, &consumer))
	return consumer
}

func testAIGatewayConsumer(id string, name string, policies []string) kkComps.AIGatewayConsumer {
	return kkComps.AIGatewayConsumer{
		ID:          id,
		Name:        name,
		Type:        kkComps.AIGatewayConsumerTypeAPIKey,
		DisplayName: "Support User",
		Policies:    policies,
	}
}

type testAIGatewayConsumerAPI struct {
	consumers []kkComps.AIGatewayConsumer
}

func (t *testAIGatewayConsumerAPI) ListAiGatewayConsumers(
	context.Context,
	kkOps.ListAiGatewayConsumersRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayConsumersResponse, error) {
	return &kkOps.ListAiGatewayConsumersResponse{
		ListAIGatewayConsumersResponse: &kkComps.ListAIGatewayConsumersResponse{
			Data: t.consumers,
		},
	}, nil
}

func (t *testAIGatewayConsumerAPI) CreateAiGatewayConsumer(
	context.Context,
	string,
	kkComps.CreateAIGatewayConsumerRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayConsumerResponse, error) {
	return nil, nil
}

func (t *testAIGatewayConsumerAPI) GetAiGatewayConsumer(
	_ context.Context,
	_ string,
	consumerID string,
	_ ...kkOps.Option,
) (*kkOps.GetAiGatewayConsumerResponse, error) {
	for _, consumer := range t.consumers {
		if consumer.ID == consumerID || consumer.Name == consumerID {
			return &kkOps.GetAiGatewayConsumerResponse{AIGatewayConsumer: &consumer}, nil
		}
	}
	return &kkOps.GetAiGatewayConsumerResponse{}, nil
}

func (t *testAIGatewayConsumerAPI) UpdateAiGatewayConsumer(
	context.Context,
	kkOps.UpdateAiGatewayConsumerRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayConsumerResponse, error) {
	return nil, nil
}

func (t *testAIGatewayConsumerAPI) DeleteAiGatewayConsumer(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayConsumerResponse, error) {
	return nil, nil
}
