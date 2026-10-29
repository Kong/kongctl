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

func ownedAgent(
	t *testing.T,
	name, namespace string,
) (resources.AIGatewayAgentResource, kkComps.AIGatewayAgent) {
	t.Helper()
	desired := resources.AIGatewayAgentResource{
		BaseResource: resources.BaseResource{Ref: name, Kongctl: &resources.KongctlMeta{Namespace: new(namespace)}},
		AIGateway:    "shared", CreateAIGatewayAgentRequest: kkComps.CreateAIGatewayAgentRequest{
			Name: name, DisplayName: name, Type: "a2a", Enabled: new(true),
			Config: kkComps.CreateAIGatewayAgentRequestConfig{URL: "https://example.com"},
		},
	}

	payload, err := desired.PayloadMap()
	require.NoError(t, err)
	payload[FieldID] = name + "-id"
	payload[FieldManagedBy] = resources.BuildChildManagedBy(
		namespace,
		false,
		"dev",
	)
	payload["created_at"], payload["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var observed kkComps.AIGatewayAgent
	require.NoError(t, json.Unmarshal(data, &observed))
	return desired, observed
}

func agentResourceSet(agents ...resources.AIGatewayAgentResource) *resources.ResourceSet {
	rs := federatedResourceSet()
	rs.AIGatewayAgents = agents
	rs.SyncScope.AddChild(resources.ResourceTypeAIGateway, "shared", resources.ResourceTypeAIGatewayAgent)
	for _, agent := range agents {
		rs.SyncScope.AddChildNamespace(
			resources.ResourceTypeAIGatewayAgent,
			"shared",
			resources.GetNamespace(agent.Kongctl),
		)
	}
	return rs
}

func TestFederatedAgentsScopeAndCollisions(t *testing.T) {
	a, observedA := ownedAgent(t, "agent-a", "team-a")
	b, observedB := ownedAgent(t, "agent-b", "team-b")
	_, unowned := ownedAgent(t, "legacy", "")
	unowned.Labels = map[string]string{"KONGCTL-namespace": "team-a", "KONGCTL-protected": "true"}
	observedA.Labels = map[string]string{"public": "retained", "password": "public-label"}
	api := &testAIGatewayAgentAPI{agents: []kkComps.AIGatewayAgent{observedA, observedB, unowned}}
	client := state.NewClient(agentEmptyUsersConfig(api))
	for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
		rs := agentResourceSet(a, b)
		plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: mode})
		require.NoError(t, err)
		require.Empty(t, plan.Changes)
		empty := agentResourceSet()
		empty.SyncScope.AddChildNamespace(resources.ResourceTypeAIGatewayAgent, "shared", "team-a")
		client = state.NewClient(
			agentEmptyUsersConfig(api),
		)
		plan, err = NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), empty, Options{Mode: mode})
		require.NoError(t, err)
		if mode == PlanModeApply {
			require.Empty(t, plan.Changes)
		} else {
			require.Len(t, plan.Changes, 1)
			require.Equal(t, "agent-a-id", plan.Changes[0].ResourceID)
			require.Equal(t, "team-a", plan.Changes[0].Namespace)
		}
	}
	for _, name := range []string{"agent-b", "legacy"} {
		desired, _ := ownedAgent(t, name, "team-a")
		plan, err := NewPlanner(
			client,
			slog.Default(),
		).GeneratePlan(t.Context(), agentResourceSet(desired), Options{Mode: PlanModeApply})
		require.Error(t, err)
		require.Nil(t, plan)
	}
}

func TestAgentAliasesShareOneCompleteReconciliation(t *testing.T) {
	a, observedA := ownedAgent(t, "agent-a", "team-a")
	b, observedB := ownedAgent(t, "agent-b", "team-b")
	b.AIGateway = "alias-b"
	rs := agentResourceSet(a, b)
	alias := rs.AIGateways[0]
	alias.Ref = "alias-b"
	rs.AIGateways = append(rs.AIGateways, alias)
	api := &testAIGatewayAgentAPI{agents: []kkComps.AIGatewayAgent{observedA, observedB}}
	client := state.NewClient(agentEmptyUsersConfig(api))
	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)

	a.Name = b.Name
	rs.AIGatewayAgents = []resources.AIGatewayAgentResource{a, b}
	_, err = NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.ErrorContains(t, err,
		`duplicate desired AI Gateway agent name "agent-b" on gateway "shared" `+
			`(refs "agent-a" and "agent-b")`)
}

func agentEmptyUsersConfig(api *testAIGatewayAgentAPI) state.ClientConfig {
	cfg := policyEmptyUsersConfig(&testAIGatewayPolicyAPI{})
	cfg.AIGatewayAgentsAPI = api
	return cfg
}
