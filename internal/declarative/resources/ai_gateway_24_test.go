package resources

import (
	"encoding/json"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func aiGateway24BalancerModelJSON(balancer string) string {
	return strings.Replace(
		aiGatewayModelJSON,
		`"config": {
    "route": {},
    "model": {}
  },`,
		`"config": {
    "route": {},
    "model": {},
    "balancer": `+balancer+`
  },`,
		1,
	)
}

func TestAIGatewayModelSessionAffinityRejectedOnConsistentHashing(t *testing.T) {
	payload := aiGateway24BalancerModelJSON(
		`{"algorithm":"consistent-hashing","session_affinity":{"storage":{"type":"local"}}}`,
	)
	var model AIGatewayModelResource
	err := json.Unmarshal([]byte(payload), &model)
	require.ErrorContains(t, err, "session_affinity")
	require.ErrorContains(t, err, "consistent-hashing")
}

func TestAIGatewayModelSessionAffinityAllowedOnOtherAlgorithms(t *testing.T) {
	payload := aiGateway24BalancerModelJSON(
		`{"algorithm":"round-robin","session_affinity":{"ttl":3600,"storage":{"type":"local"}}}`,
	)
	var model AIGatewayModelResource
	require.NoError(t, json.Unmarshal([]byte(payload), &model))
	require.NoError(t, model.Validate())
}

func TestAIGatewayModelExplainNodeDescribesSessionAffinity(t *testing.T) {
	node, err := aiGatewayModelExplainNode(ExplainBuildContext{})
	require.NoError(t, err)
	require.NotEmpty(t, node.OneOf)

	for _, branch := range node.OneOf {
		configField := branch.propIndex["config"]
		require.NotNil(t, configField)
		balancerField := configField.Node.propIndex["balancer"]
		require.NotNil(t, balancerField)
		affinityField := balancerField.Node.propIndex["session_affinity"]
		require.NotNil(t, affinityField)
		require.Contains(t, affinityField.Node.propIndex, "ttl")
		require.Contains(t, affinityField.Node.propIndex, "enabled")
		storageField := affinityField.Node.propIndex["storage"]
		require.NotNil(t, storageField)
		require.NotEmpty(t, storageField.Node.OneOf, "storage should describe the local/redis union")
	}
}

// The write-only catalog entry for session_affinity.storage.password only governs what the
// loader accepts (!secret) and what the planner offers for secret-only writes; it does not
// itself redact anything on dump. Dump correctness instead depends on the live API never
// returning a value for a write-only field, the same contract every other secret field in
// this codebase relies on. This pins that contract for the dump conversion path: given an
// observed response that (like the real API) omits password, the dumped manifest has no
// password field, while the rest of the redis storage still comes through correctly.
func TestAIGatewayModelDumpOmitsSessionAffinityRedisPassword(t *testing.T) {
	const observedModelJSON = `{
		"id": "model-id", "type": "model", "name": "affinity-model", "display_name": "Affinity model",
		"enabled": true, "formats": [{"type": "openai"}], "capabilities": ["generate"],
		"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
		"config": {
			"route": {}, "model": {},
			"balancer": {
				"algorithm": "round-robin",
				"session_affinity": {
					"enabled": true, "ttl": 3600,
					"storage": {"type": "redis", "host": "redis.example.com", "port": 6379}
				}
			}
		},
		"targets": [
			{"name": "target-a", "provider": "provider", "config": {"type": "openai"}},
			{"name": "target-b", "provider": "provider", "config": {"type": "openai"}}
		]
	}`

	var model kkComps.AIGatewayModel
	require.NoError(t, json.Unmarshal([]byte(observedModelJSON), &model))
	require.NotContains(t, observedModelJSON, "password", "fixture must mirror the API's write-only contract")

	exported, err := AIGatewayModelResourceFromResponse("gateway", model)
	require.NoError(t, err)
	payload, err := exported.MutablePayloadMap()
	require.NoError(t, err)

	config := payload["config"].(map[string]any)
	balancer := config["balancer"].(map[string]any)
	affinity := balancer["session_affinity"].(map[string]any)
	storage := affinity["storage"].(map[string]any)
	require.NotContains(t, storage, "password")
	require.Equal(t, "redis.example.com", storage["host"], "non-secret fields must still dump")

	encoded, err := json.Marshal(exported)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "password")
}

func TestAIGatewayModelDatastorePurposeAllowsSessionAffinity(t *testing.T) {
	node, err := aiGatewayModelExplainNode(ExplainBuildContext{})
	require.NoError(t, err)
	require.NotEmpty(t, node.OneOf)

	for _, branch := range node.OneOf {
		datastoresField := branch.propIndex["datastores"]
		require.NotNil(t, datastoresField)
		purposeField := datastoresField.Node.Items.propIndex["purpose"]
		require.NotNil(t, purposeField)
		require.ElementsMatch(t, []any{"semantic-balancer", "session-affinity"}, purposeField.Node.Enum)
	}
}
