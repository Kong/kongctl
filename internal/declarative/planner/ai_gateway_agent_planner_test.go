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
				testAIGatewayAgent(nil),
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

func TestAIGatewayAgentPlannerIgnoresAPIDefaults(t *testing.T) {
	agent := testAIGatewayAgentResourceWithConfig(t, `{
		"url": "https://booking-agent.example.com",
		"route": {"paths": ["/agents/support"]},
		"logging": {"max_payload_size": 524288}
	}`)
	var current kkComps.AIGatewayAgent
	require.NoError(t, json.Unmarshal([]byte(`{
		"id": "agent-id",
		"name": "booking-agent",
		"type": "a2a",
		"display_name": "Booking Agent",
		"enabled": true,
		"config": {
			"url": "https://booking-agent.example.com",
			"route": {
				"paths": ["/agents/support"],
				"https_redirect_status_code": 426,
				"preserve_host": false,
				"protocols": ["http", "https"],
				"regex_priority": 0,
				"request_buffering": true,
				"response_buffering": true,
				"strip_path": true
			},
			"max_request_body_size": 8388608,
			"logging": {
				"payloads": false,
				"statistics": true,
				"max_payload_size": 524288
			}
		}
	}`), &current))

	needsUpdate, fields, changed, err := (&Planner{}).shouldUpdateAIGatewayAgent(
		state.AIGatewayAgent{AIGatewayAgent: current},
		agent,
	)

	require.NoError(t, err)
	require.False(t, needsUpdate)
	require.Nil(t, fields)
	require.Nil(t, changed)
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
				testAIGatewayAgent(nil),
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
	require.Equal(t, tags.RefPlaceholderPrefix+"mask-sensitive-data#name", agentCreate.References[FieldPolicies+".0"].Ref)
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
						testAIGatewayAgent([]string{currentPolicyRef}),
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

func TestAIGatewayAgentPlannerPolicyRefsNoopWhenResolvedAndReordered(t *testing.T) {
	policyOne := testAIGatewayPolicyResource(t)
	policyOne.Ref = "poc-a2a-key-auth"
	policyOne.Name = "poc-a2a-key-auth"
	policyOne.DisplayName = "POC A2A Key Auth"

	policyTwo := testAIGatewayPolicyResource(t)
	policyTwo.Ref = "poc-a2a-rate-limit"
	policyTwo.Name = "poc-a2a-rate-limit"
	policyTwo.DisplayName = "POC A2A Rate Limit"

	agent := testAIGatewayAgentResource(t, []string{
		tags.RefPlaceholderPrefix + "poc-a2a-key-auth#name",
		tags.RefPlaceholderPrefix + "poc-a2a-rate-limit#name",
	})
	current := testAIGatewayAgent([]string{
		"poc-a2a-rate-limit",
		"poc-a2a-key-auth",
	})
	rs := &resources.ResourceSet{
		AIGatewayPolicies: []resources.AIGatewayPolicyResource{policyOne, policyTwo},
	}

	needsUpdate, fields, changed, err := (&Planner{resources: rs}).shouldUpdateAIGatewayAgent(
		state.AIGatewayAgent{AIGatewayAgent: current},
		agent,
	)

	require.NoError(t, err)
	require.Falsef(t, needsUpdate, "changed fields: %#v", changed)
	require.Nil(t, fields)
	require.Nil(t, changed)
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
	return testAIGatewayAgentResourceWithConfigAndPolicies(
		t,
		`{"url": "https://booking-agent.example.com"}`,
		policies,
	)
}

func testAIGatewayAgentResourceWithConfig(
	t *testing.T,
	config string,
) resources.AIGatewayAgentResource {
	t.Helper()
	return testAIGatewayAgentResourceWithConfigAndPolicies(t, config, nil)
}

func testAIGatewayAgentResourceWithConfigAndPolicies(
	t *testing.T,
	config string,
	policies []string,
) resources.AIGatewayAgentResource {
	t.Helper()
	payload := map[string]any{
		"ref":          "booking-agent",
		"ai_gateway":   "support-gateway",
		"name":         "booking-agent",
		"type":         "a2a",
		"display_name": "Booking Agent",
		"config":       json.RawMessage(config),
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

func testAIGatewayAgent(policies []string) kkComps.AIGatewayAgent {
	enabled := true
	return kkComps.AIGatewayAgent{
		ID:          "agent-id",
		Name:        "booking-agent",
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
