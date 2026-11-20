package loader

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func aiGateway24ModelManifest(balancer string) string {
	return `ai_gateways:
- ref: gateway
  name: gateway
  display_name: Gateway
  models:
  - ref: model
    name: model
    display_name: Model
    type: model
    formats: [{type: openai}]
    capabilities: [generate]
    config:
      route: {}
      model: {}
      balancer: ` + balancer + `
    targets:
    - name: target-a
      provider: existing-provider
      config: {type: openai}
    - name: target-b
      provider: existing-provider
      config: {type: openai}
`
}

func TestAIGateway24SessionAffinityLocalStorageLoads(t *testing.T) {
	manifest := aiGateway24ModelManifest(
		`{algorithm: round-robin, session_affinity: {ttl: 3600, storage: {type: local}}}`,
	)
	rs, err := New().LoadFile(writeLoaderTestFile(t, manifest))
	require.NoError(t, err)
	require.NoError(t, rs.AIGatewayModels[0].Validate())

	payload, err := rs.AIGatewayModels[0].MutablePayloadMap()
	require.NoError(t, err)
	balancer := payload["config"].(map[string]any)["balancer"].(map[string]any)
	affinity := balancer["session_affinity"].(map[string]any)
	require.InEpsilon(t, 3600, affinity["ttl"], 0)
	require.Equal(t, "local", affinity["storage"].(map[string]any)["type"])
}

func TestAIGateway24SessionAffinityRedisStorageLoads(t *testing.T) {
	manifest := aiGateway24ModelManifest(
		`{algorithm: least-connections, session_affinity: {storage: {type: redis, host: redis.test, port: 6379}}}`,
	)
	rs, err := New().LoadFile(writeLoaderTestFile(t, manifest))
	require.NoError(t, err)
	require.NoError(t, rs.AIGatewayModels[0].Validate())

	payload, err := rs.AIGatewayModels[0].MutablePayloadMap()
	require.NoError(t, err)
	balancer := payload["config"].(map[string]any)["balancer"].(map[string]any)
	storage := balancer["session_affinity"].(map[string]any)["storage"].(map[string]any)
	require.Equal(t, "redis", storage["type"])
	require.Equal(t, "redis.test", storage["host"])
}

func TestAIGateway24SessionAffinityRejectedOnConsistentHashing(t *testing.T) {
	manifest := aiGateway24ModelManifest(
		`{algorithm: consistent-hashing, session_affinity: {storage: {type: local}}}`,
	)
	_, err := New().LoadFile(writeLoaderTestFile(t, manifest))
	require.ErrorContains(t, err, "session_affinity")
	require.ErrorContains(t, err, "consistent-hashing")
}

func TestAIGateway24ModelDatastoreSessionAffinityPurpose(t *testing.T) {
	manifest := strings.Replace(
		aiGateway24ModelManifest(`{algorithm: round-robin, session_affinity: {storage: {type: redis}}}`),
		"formats: [{type: openai}]",
		"datastores: [{name: shared-redis, purpose: session-affinity}]\n    formats: [{type: openai}]",
		1,
	)
	rs, err := New().LoadFile(writeLoaderTestFile(t, manifest))
	require.NoError(t, err)
	require.NoError(t, rs.AIGatewayModels[0].Validate())

	encoded, err := json.Marshal(rs.AIGatewayModels[0])
	require.NoError(t, err)
	var round map[string]any
	require.NoError(t, json.Unmarshal(encoded, &round))
	stores := round["datastores"].([]any)
	store := stores[0].(map[string]any)
	require.Equal(t, "shared-redis", store["name"])
	require.Equal(t, "session-affinity", store["purpose"])
}

func TestAIGateway24SessionAffinityRedisPasswordRequiresSecret(t *testing.T) {
	manifest := aiGateway24ModelManifest(
		`{algorithm: round-robin, session_affinity: {storage: {type: redis, host: redis.test, ` +
			`password: plaintext-sentinel}}}`,
	)
	_, err := New().LoadFile(writeLoaderTestFile(t, manifest))
	require.ErrorContains(t, err, "requires !secret")
	require.NotContains(t, err.Error(), "plaintext-sentinel")
}

func TestAIGateway24SessionAffinityRedisPasswordAcceptsSecret(t *testing.T) {
	t.Setenv("AIGW24_SESSION_AFFINITY_REDIS_PASSWORD", "redis-password-sentinel")
	manifest := aiGateway24ModelManifest(
		`{algorithm: round-robin, session_affinity: {storage: {type: redis, host: redis.test, ` +
			`password: !secret {source: !env AIGW24_SESSION_AFFINITY_REDIS_PASSWORD}}}}`,
	)
	rs, err := New().LoadFile(writeLoaderTestFile(t, manifest))
	require.NoError(t, err)
	require.NoError(t, rs.AIGatewayModels[0].Validate())

	saved, err := json.Marshal(rs)
	require.NoError(t, err)
	require.NotContains(t, string(saved), "redis-password-sentinel")
}
