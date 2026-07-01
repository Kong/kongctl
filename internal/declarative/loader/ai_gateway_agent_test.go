package loader

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

const aiGatewayAgentYAML = `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    policies:
      - ref: mask-sensitive-data
        type: ai-sanitizer
        name: mask-sensitive-data
        display_name: Mask Sensitive Data
        config:
          anonymize:
            - email
    agents:
      - ref: booking-agent
        name: booking-agent
        type: a2a
        display_name: Booking Agent
        config:
          url: https://booking-agent.example.com
        policies:
          - !ref mask-sensitive-data
`

func TestLoaderExtractsNestedAIGatewayAgents(t *testing.T) {
	path := writeLoaderTestFile(t, aiGatewayAgentYAML)

	rs, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGateways, 1)
	require.Empty(t, rs.AIGateways[0].Agents)
	require.Len(t, rs.AIGatewayAgents, 1)
	require.Equal(t, "support-gateway", rs.AIGatewayAgents[0].AIGateway)
	require.Equal(t, "booking-agent", rs.AIGatewayAgents[0].Name)
	require.Equal(t, "a2a", string(rs.AIGatewayAgents[0].Type))
	require.Equal(t, "https://booking-agent.example.com", rs.AIGatewayAgents[0].Config.URL)
	require.Equal(
		t,
		[]string{tags.RefPlaceholderPrefix + "mask-sensitive-data#id"},
		rs.AIGatewayAgents[0].Policies,
	)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"support-gateway",
		resources.ResourceTypeAIGatewayAgent,
	))
}

func TestLoaderValidatesAIGatewayAgentParentAndDuplicates(t *testing.T) {
	rootOnly := `
ai_gateway_agents:
  - ref: booking-agent
    ai_gateway: missing-gateway
    name: booking-agent
    type: a2a
    display_name: Booking Agent
    config:
      url: https://booking-agent.example.com
`
	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, rootOnly), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "references unknown ai_gateway")

	duplicates := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    agents:
      - ref: booking-agent
        name: booking-agent
        type: a2a
        display_name: Booking Agent
        config:
          url: https://booking-agent.example.com
      - ref: booking-agent-2
        name: booking-agent
        type: a2a
        display_name: Booking Agent 2
        config:
          url: https://booking-agent-2.example.com
`
	_, err = New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, duplicates), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate ai_gateway_agent name")
}

func TestLoaderRejectsRootLevelEmptyAIGatewayAgents(t *testing.T) {
	input := `ai_gateway_agents: []`

	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ai_gateway_agents cannot be empty")
}

func TestLoaderAcceptsAIGatewayAgentDeferredExternalParentRef(t *testing.T) {
	input := `
ai_gateways:
  - ref: external-shared-gateway
    _external:
      selector:
        matchFields:
          display_name: Shared Gateway
ai_gateway_agents:
  - ref: booking-agent
    ai_gateway: !ref external-shared-gateway#id
    name: booking-agent
    type: a2a
    display_name: Booking Agent
    config:
      url: https://booking-agent.example.com
`

	rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayAgents, 1)
	require.Equal(t, tags.RefPlaceholderPrefix+"external-shared-gateway#id", rs.AIGatewayAgents[0].AIGateway)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"external-shared-gateway",
		resources.ResourceTypeAIGatewayAgent,
	))
}
