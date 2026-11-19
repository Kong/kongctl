package resources

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func testAIGatewayAgentWithConfig(config kkComps.CreateAIGatewayAgentRequestConfig) AIGatewayAgentResource {
	return AIGatewayAgentResource{
		BaseResource: BaseResource{Ref: "booking-agent"},
		AIGateway:    "support-gateway",
		CreateAIGatewayAgentRequest: kkComps.CreateAIGatewayAgentRequest{
			Name:        "booking-agent",
			DisplayName: "Booking Agent",
			Type:        kkComps.CreateAIGatewayAgentRequestTypeA2a,
			Config:      config,
		},
	}
}

func TestAIGatewayAgentValidateURLOnly(t *testing.T) {
	agent := testAIGatewayAgentWithConfig(kkComps.CreateAIGatewayAgentRequestConfig{
		URL: new("https://booking-agent.example.com"),
	})
	require.NoError(t, agent.Validate())
}

func TestAIGatewayAgentValidateTargetsOnly(t *testing.T) {
	agent := testAIGatewayAgentWithConfig(kkComps.CreateAIGatewayAgentRequestConfig{
		Targets: []kkComps.AIGatewayUpstreamTarget{
			{Address: "booking-agent-1.internal.kongair.com:443"},
		},
	})
	require.NoError(t, agent.Validate())
}

func TestAIGatewayAgentValidateRejectsNeitherURLNorTargets(t *testing.T) {
	agent := testAIGatewayAgentWithConfig(kkComps.CreateAIGatewayAgentRequestConfig{})
	err := agent.Validate()
	require.ErrorContains(t, err, "booking-agent")
	require.ErrorContains(t, err, "exactly one of config.url or config.targets")
}

func TestAIGatewayAgentValidateRejectsBothURLAndTargets(t *testing.T) {
	agent := testAIGatewayAgentWithConfig(kkComps.CreateAIGatewayAgentRequestConfig{
		URL: new("https://booking-agent.example.com"),
		Targets: []kkComps.AIGatewayUpstreamTarget{
			{Address: "booking-agent-1.internal.kongair.com:443"},
		},
	})
	err := agent.Validate()
	require.ErrorContains(t, err, "booking-agent")
	require.ErrorContains(t, err, "exactly one of config.url or config.targets")
}

func TestAIGatewayAgentValidateRejectsBalancerWithoutTargets(t *testing.T) {
	agent := testAIGatewayAgentWithConfig(kkComps.CreateAIGatewayAgentRequestConfig{
		URL: new("https://booking-agent.example.com"),
		Balancer: func() *kkComps.AIGatewayUpstreamBalancerConfig {
			cfg := kkComps.CreateAIGatewayUpstreamBalancerConfigRoundRobin(
				kkComps.AIGatewayUpstreamBalancerRoundRobinConfig{},
			)
			return &cfg
		}(),
	})
	require.ErrorContains(t, agent.Validate(), "config.balancer is only valid when config.targets is set")
}

func TestAIGatewayAgentValidateRejectsProtocolWithoutTargets(t *testing.T) {
	agent := testAIGatewayAgentWithConfig(kkComps.CreateAIGatewayAgentRequestConfig{
		URL:      new("https://booking-agent.example.com"),
		Protocol: kkComps.CreateAIGatewayAgentRequestProtocolHTTPS.ToPointer(),
	})
	require.ErrorContains(t, agent.Validate(), "config.protocol is only valid when config.targets is set")
}

func TestAIGatewayAgentValidateRejectsPathWithoutTargets(t *testing.T) {
	agent := testAIGatewayAgentWithConfig(kkComps.CreateAIGatewayAgentRequestConfig{
		URL:  new("https://booking-agent.example.com"),
		Path: new("/a2a"),
	})
	require.ErrorContains(t, agent.Validate(), "config.path is only valid when config.targets is set")
}

func TestAIGatewayAgentValidateAllowsBalancerProtocolPathWithTargets(t *testing.T) {
	agent := testAIGatewayAgentWithConfig(kkComps.CreateAIGatewayAgentRequestConfig{
		Targets: []kkComps.AIGatewayUpstreamTarget{
			{Address: "booking-agent-1.internal.kongair.com:443"},
		},
		Balancer: func() *kkComps.AIGatewayUpstreamBalancerConfig {
			cfg := kkComps.CreateAIGatewayUpstreamBalancerConfigRoundRobin(
				kkComps.AIGatewayUpstreamBalancerRoundRobinConfig{},
			)
			return &cfg
		}(),
		Protocol: kkComps.CreateAIGatewayAgentRequestProtocolHTTPS.ToPointer(),
		Path:     new("/a2a"),
	})
	require.NoError(t, agent.Validate())
}

func TestAIGatewayAgentBalancerAlgorithmsRoundTrip(t *testing.T) {
	balancers := map[string]kkComps.AIGatewayUpstreamBalancerConfig{
		"consistent-hashing": kkComps.CreateAIGatewayUpstreamBalancerConfigConsistentHashing(
			kkComps.AIGatewayUpstreamBalancerConsistentHashingConfig{HashOnHeader: "x-session-id"},
		),
		"least-connections": kkComps.CreateAIGatewayUpstreamBalancerConfigLeastConnections(
			kkComps.AIGatewayUpstreamBalancerLeastConnectionsConfig{},
		),
		"lowest-latency": kkComps.CreateAIGatewayUpstreamBalancerConfigLowestLatency(
			kkComps.AIGatewayUpstreamBalancerLowestLatencyConfig{},
		),
		"round-robin": kkComps.CreateAIGatewayUpstreamBalancerConfigRoundRobin(
			kkComps.AIGatewayUpstreamBalancerRoundRobinConfig{},
		),
	}

	for algorithm, balancer := range balancers {
		t.Run(algorithm, func(t *testing.T) {
			balancer := balancer
			agent := testAIGatewayAgentWithConfig(kkComps.CreateAIGatewayAgentRequestConfig{
				Targets: []kkComps.AIGatewayUpstreamTarget{
					{Address: "booking-agent-1.internal.kongair.com:443"},
				},
				Balancer: &balancer,
			})
			require.NoError(t, agent.Validate())

			payload, err := agent.PayloadMap()
			require.NoError(t, err)
			config := payload[aiGatewayAgentFieldConfig].(map[string]any)
			roundTripped := config["balancer"].(map[string]any)
			require.Equal(t, algorithm, roundTripped["algorithm"])
		})
	}
}
