package aigateway

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayAgentDetailViewIncludesAuthStrategies(t *testing.T) {
	t.Parallel()

	agent := kkComps.AIGatewayAgent{
		ID:          "agent-id",
		Name:        "booking-agent",
		DisplayName: "Booking Agent",
		Type:        kkComps.TypeA2a,
		Access: &kkComps.AIGatewayAgentAccess{
			AuthStrategies: []string{"support-key-auth"},
		},
	}

	detail := aiGatewayAgentDetailView(agent)
	require.Contains(t, detail, `access: {"auth_strategies":["support-key-auth"]}`)
	require.NotContains(t, detail, "acls: n/a")
}
