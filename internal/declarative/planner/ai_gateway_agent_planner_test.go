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

func TestAIGatewayAgentPlannerCreatesChildForExistingGateway(t *testing.T) {
	agent := testAIGatewayAgentResource(t, nil)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayAgentsAPI: &testAIGatewayAgentAPI{},
	})
	rs := testAIGatewayAgentResourceSet(agent)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayAgent, change.ResourceType)
	require.Equal(t, "booking-agent", change.ResourceRef)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
	require.Equal(t, "support-gateway", change.Parent.Ref)
	require.Equal(t, "Booking Agent", change.Fields[FieldDisplayName])
	require.Equal(t, "a2a", change.Fields[FieldType])
	require.Equal(t, "https://booking-agent.example.com", change.Fields[FieldConfig].(map[string]any)["url"])
}

func TestAIGatewayAgentPlannerUpdatesExistingAgent(t *testing.T) {
	agent := testAIGatewayAgentResource(t, nil)
	agent.DisplayName = "Booking Agent Updated"
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayAgentsAPI: &testAIGatewayAgentAPI{
			agents: []kkComps.AIGatewayAgent{
				testAIGatewayAgent("agent-id", "booking-agent", nil),
			},
		},
	})
	rs := testAIGatewayAgentResourceSet(agent)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionUpdate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayAgent, change.ResourceType)
	require.Equal(t, "agent-id", change.ResourceID)
	require.Equal(t, "Booking Agent Updated", change.Fields[FieldDisplayName])
	require.Contains(t, change.ChangedFields, FieldDisplayName)
}

func TestAIGatewayAgentPlannerSyncDeletesScopedAgents(t *testing.T) {
	scope := resources.NewSyncScope()
	scope.AddRoot(resources.ResourceTypeAIGateway)
	scope.AddChild(resources.ResourceTypeAIGateway, "support-gateway", resources.ResourceTypeAIGatewayAgent)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayAgentsAPI: &testAIGatewayAgentAPI{
			agents: []kkComps.AIGatewayAgent{
				testAIGatewayAgent("agent-id", "booking-agent", nil),
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
	require.Equal(t, ResourceTypeAIGatewayAgent, change.ResourceType)
	require.Equal(t, "agent-id", change.ResourceID)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
}

func TestAIGatewayAgentPlannerDependsOnPolicyCreate(t *testing.T) {
	policy := testAIGatewayPolicyResource(t)
	agent := testAIGatewayAgentResource(t, []string{tags.RefPlaceholderPrefix + "mask-sensitive-data#id"})
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{},
	})
	rs := &resources.ResourceSet{
		AIGateways:          []resources.AIGatewayResource{testAIGatewayResource()},
		AIGatewayPolicies:   []resources.AIGatewayPolicyResource{policy},
		AIGatewayAgents:     []resources.AIGatewayAgentResource{agent},
		AIGatewayProviders:  nil,
		AIGatewayModels:     nil,
		AIGatewayMCPServers: nil,
		AIGatewayVaults:     nil,
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)

	gatewayCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGateway, "support-gateway")
	policyCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayPolicy, "mask-sensitive-data")
	agentCreate := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayAgent, "booking-agent")

	require.Contains(t, policyCreate.DependsOn, gatewayCreate.ID)
	require.Contains(t, agentCreate.DependsOn, policyCreate.ID)
	require.Equal(t, resources.UnknownReferenceID, agentCreate.References[FieldPolicies+".0"].ID)
	require.Equal(t, tags.RefPlaceholderPrefix+"mask-sensitive-data#id", agentCreate.References[FieldPolicies+".0"].Ref)
}

func TestAIGatewayAgentPlannerPolicyRefNoopForExistingAgent(t *testing.T) {
	for _, currentPolicyRef := range []string{"policy-id", "mask-sensitive-data"} {
		t.Run(currentPolicyRef, func(t *testing.T) {
			policy := testAIGatewayPolicyResource(t)
			agent := testAIGatewayAgentResource(t, []string{tags.RefPlaceholderPrefix + "mask-sensitive-data#id"})
			client := state.NewClient(state.ClientConfig{
				AIGatewayAPI: &testAIGatewayAPI{
					gateways: []kkComps.AIGateway{testAIGateway()},
				},
				AIGatewayPoliciesAPI: &testAIGatewayPolicyAPI{
					policies: []kkComps.AIGatewayPolicy{testAIGatewayPolicy()},
				},
				AIGatewayAgentsAPI: &testAIGatewayAgentAPI{
					agents: []kkComps.AIGatewayAgent{
						testAIGatewayAgent("agent-id", "booking-agent", []string{currentPolicyRef}),
					},
				},
			})
			rs := &resources.ResourceSet{
				AIGateways:        []resources.AIGatewayResource{testAIGatewayResource()},
				AIGatewayPolicies: []resources.AIGatewayPolicyResource{policy},
				AIGatewayAgents:   []resources.AIGatewayAgentResource{agent},
			}

			plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
			require.NoError(t, err)
			require.Empty(t, plan.Changes)
		})
	}
}

func testAIGatewayAgentResourceSet(
	agent resources.AIGatewayAgentResource,
) *resources.ResourceSet {
	return &resources.ResourceSet{
		AIGateways:      []resources.AIGatewayResource{testAIGatewayResource()},
		AIGatewayAgents: []resources.AIGatewayAgentResource{agent},
	}
}

func testAIGatewayAgentResource(
	t *testing.T,
	policies []string,
) resources.AIGatewayAgentResource {
	t.Helper()
	payload := map[string]any{
		"ref":          "booking-agent",
		"ai_gateway":   "support-gateway",
		"name":         "booking-agent",
		"type":         "a2a",
		"display_name": "Booking Agent",
		"config": map[string]any{
			"url": "https://booking-agent.example.com",
		},
	}
	if policies != nil {
		payload[FieldPolicies] = policies
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var agent resources.AIGatewayAgentResource
	require.NoError(t, json.Unmarshal(data, &agent))
	return agent
}

func testAIGatewayAgent(id string, name string, policies []string) kkComps.AIGatewayAgent {
	enabled := true
	return kkComps.AIGatewayAgent{
		ID:          id,
		Name:        name,
		Type:        kkComps.AIGatewayAgentTypeA2a,
		DisplayName: "Booking Agent",
		Enabled:     &enabled,
		Config: kkComps.AIGatewayAgentConfig{
			URL: "https://booking-agent.example.com",
		},
		Policies: policies,
	}
}

type testAIGatewayAgentAPI struct {
	agents []kkComps.AIGatewayAgent
}

func (t *testAIGatewayAgentAPI) ListAiGatewayAgents(
	context.Context,
	kkOps.ListAiGatewayAgentsRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayAgentsResponse, error) {
	return &kkOps.ListAiGatewayAgentsResponse{
		ListAIGatewayAgentsResponse: &kkComps.ListAIGatewayAgentsResponse{
			Data: t.agents,
		},
	}, nil
}

func (t *testAIGatewayAgentAPI) CreateAiGatewayAgent(
	context.Context,
	string,
	kkComps.CreateAIGatewayAgentRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayAgentResponse, error) {
	return nil, nil
}

func (t *testAIGatewayAgentAPI) GetAiGatewayAgent(
	_ context.Context,
	_ string,
	agentID string,
	_ ...kkOps.Option,
) (*kkOps.GetAiGatewayAgentResponse, error) {
	for _, agent := range t.agents {
		if agent.ID == agentID || agent.Name == agentID {
			return &kkOps.GetAiGatewayAgentResponse{AIGatewayAgent: &agent}, nil
		}
	}
	return &kkOps.GetAiGatewayAgentResponse{}, nil
}

func (t *testAIGatewayAgentAPI) UpdateAiGatewayAgent(
	context.Context,
	kkOps.UpdateAiGatewayAgentRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayAgentResponse, error) {
	return nil, nil
}

func (t *testAIGatewayAgentAPI) DeleteAiGatewayAgent(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayAgentResponse, error) {
	return nil, nil
}
