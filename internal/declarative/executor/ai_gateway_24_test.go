package executor

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

const aiGateway24SessionAffinityModelJSON = `{
	"ref": "affinity-model", "ai_gateway": "gateway", "type": "model",
	"name": "affinity-model", "display_name": "Affinity model", "enabled": true,
	"formats": [{"type": "openai"}], "capabilities": ["generate"],
	"config": {
		"route": {}, "model": {},
		"balancer": {
			"algorithm": "round-robin",
			"session_affinity": {"ttl": 3600, "storage": {"type": "redis", "host": "redis.test"}}
		}
	},
	"targets": [
		{"name": "target-a", "provider": "provider", "config": {"type": "openai"}},
		{"name": "target-b", "provider": "provider", "config": {"type": "openai"}}
	]
}`

// The model adapter maps planner fields straight into the generated SDK request type
// and verifies nothing silently drops during the round trip (mapAIGatewaySDKRequest).
// session_affinity flows through automatically via that embedded SDK type; this pins
// that no field is lost for either storage variant.
func TestAIGatewayModelMapFieldsRoundTripsSessionAffinity(t *testing.T) {
	var model resources.AIGatewayModelResource
	require.NoError(t, json.Unmarshal([]byte(aiGateway24SessionAffinityModelJSON), &model))
	require.NoError(t, model.Validate())

	fields, err := model.MutablePayloadMap()
	require.NoError(t, err)

	adapter := NewAIGatewayModelAdapter(nil)
	var create kkComps.CreateAIGatewayModelRequest
	require.NoError(t, adapter.MapCreateFields(t.Context(), nil, fields, &create))
	require.NotNil(t, create.AIGatewayModelModel)
	balancer := create.AIGatewayModelModel.Config.Balancer
	require.NotNil(t, balancer)
	require.NotNil(t, balancer.AIGatewayModelBalancerRoundRobinConfig)
	affinity := balancer.AIGatewayModelBalancerRoundRobinConfig.SessionAffinity
	require.NotNil(t, affinity)
	require.EqualValues(t, 3600, *affinity.TTL)
	require.NotNil(t, affinity.Storage)
	require.NotNil(t, affinity.Storage.AIGatewaySessionAffinityStorageRedis)
	require.Equal(t, "redis.test", *affinity.Storage.AIGatewaySessionAffinityStorageRedis.Host)

	var update kkComps.UpdateAIGatewayModelRequest
	require.NoError(t, adapter.MapUpdateFields(t.Context(), nil, fields, &update, nil))
	require.NotNil(t, update.AIGatewayModelModel)
	require.NotNil(t, update.AIGatewayModelModel.Config.Balancer.AIGatewayModelBalancerRoundRobinConfig.SessionAffinity)
}
