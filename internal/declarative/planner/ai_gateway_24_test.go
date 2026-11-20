package planner

import (
	"encoding/json"
	"maps"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

const sessionAffinityModelJSON = `{
	"type":"model", "name":"affinity-model", "display_name":"Affinity model",
	"enabled":true, "formats":[{"type":"openai"}], "capabilities":["generate"],
	"config":{
		"route":{},
		"model":{},
		"balancer":{"algorithm":"round-robin"}
	},
	"targets":[
		{"name":"gpt-4o","provider":"provider","config":{"type":"openai"}},
		{"name":"gpt-4o-mini","provider":"provider","config":{"type":"openai"}}
	]
}`

// sessionAffinityModelFixture mirrors the shape of a model observed from the API: the
// balancer carries its full session_affinity defaults (enabled, ttl, local storage) even
// though the manifest in sessionAffinityModelJSON does not declare session_affinity at all.
func sessionAffinityModelFixture(t *testing.T) (map[string]any, kkComps.AIGatewayModel) {
	t.Helper()
	var desired map[string]any
	require.NoError(t, json.Unmarshal([]byte(sessionAffinityModelJSON), &desired))
	current := normalizeAIGatewayJSONMap(desired)
	current[FieldID] = "model-id"
	current["created_at"] = "2026-01-01T00:00:00Z"
	current["updated_at"] = "2026-01-01T00:00:00Z"
	balancer := current[FieldConfig].(map[string]any)["balancer"].(map[string]any)
	balancer[FieldSessionAffinity] = map[string]any{
		"enabled": true,
		"ttl":     21600,
		"storage": map[string]any{"type": "local"},
	}
	data, err := json.Marshal(current)
	require.NoError(t, err)
	var observed kkComps.AIGatewayModel
	require.NoError(t, json.Unmarshal(data, &observed))
	return desired, observed
}

func TestAIGatewaySessionAffinityOmittedDefaultsDoNotDrift(t *testing.T) {
	desired, current := sessionAffinityModelFixture(t)
	data, err := json.Marshal(desired)
	require.NoError(t, err)
	var model resources.AIGatewayModelResource
	require.NoError(t, json.Unmarshal(data, &model))
	model.Ref, model.AIGateway = "affinity-model", "gateway"

	update, _, changed, err := (&Planner{}).shouldUpdateAIGatewayModel(
		state.AIGatewayModel{AIGatewayModel: current}, model,
	)
	require.NoError(t, err)
	require.Falsef(t, update, "unexpected drift from omitted session_affinity defaults: %#v", changed)
}

func TestAIGatewaySessionAffinityExplicitTTLDrifts(t *testing.T) {
	_, current := sessionAffinityModelFixture(t)
	var desired map[string]any
	require.NoError(t, json.Unmarshal([]byte(sessionAffinityModelJSON), &desired))
	balancer := desired[FieldConfig].(map[string]any)["balancer"].(map[string]any)
	balancer[FieldSessionAffinity] = map[string]any{"ttl": 3600}
	data, err := json.Marshal(desired)
	require.NoError(t, err)
	var model resources.AIGatewayModelResource
	require.NoError(t, json.Unmarshal(data, &model))
	model.Ref, model.AIGateway = "affinity-model", "gateway"

	update, _, changed, err := (&Planner{}).shouldUpdateAIGatewayModel(
		state.AIGatewayModel{AIGatewayModel: current}, model,
	)
	require.NoError(t, err)
	require.True(t, update)
	require.Contains(t, changed, FieldConfig)
}

func TestAIGatewaySessionAffinityDisabledDrifts(t *testing.T) {
	_, current := sessionAffinityModelFixture(t)
	var desired map[string]any
	require.NoError(t, json.Unmarshal([]byte(sessionAffinityModelJSON), &desired))
	balancer := desired[FieldConfig].(map[string]any)["balancer"].(map[string]any)
	balancer[FieldSessionAffinity] = map[string]any{"enabled": false}
	data, err := json.Marshal(desired)
	require.NoError(t, err)
	var model resources.AIGatewayModelResource
	require.NoError(t, json.Unmarshal(data, &model))
	model.Ref, model.AIGateway = "affinity-model", "gateway"

	update, _, changed, err := (&Planner{}).shouldUpdateAIGatewayModel(
		state.AIGatewayModel{AIGatewayModel: current}, model,
	)
	require.NoError(t, err)
	require.True(t, update)
	require.Contains(t, changed, FieldConfig)
}

// Mirrors the pgvector/semantic-balancer datastore handling: a datastore referenced with
// purpose "session-affinity" supplies redis connection details the API expands onto the
// observed model. The manifest must not need to repeat them for the plan to converge.
func TestAIGatewaySessionAffinityRedisDatastoreConnectionDoesNotDrift(t *testing.T) {
	var desired map[string]any
	require.NoError(t, json.Unmarshal([]byte(sessionAffinityModelJSON), &desired))
	desired[FieldDatastores] = []any{map[string]any{FieldName: "redis-store", "purpose": "session-affinity"}}
	desiredBalancer := desired[FieldConfig].(map[string]any)["balancer"].(map[string]any)
	desiredBalancer[FieldSessionAffinity] = map[string]any{"storage": map[string]any{"type": "redis"}}

	current := normalizeAIGatewayJSONMap(desired)
	current[FieldID] = "model-id"
	current["created_at"] = "2026-01-01T00:00:00Z"
	current["updated_at"] = "2026-01-01T00:00:00Z"
	currentBalancer := current[FieldConfig].(map[string]any)["balancer"].(map[string]any)
	currentStorage := currentBalancer[FieldSessionAffinity].(map[string]any)["storage"].(map[string]any)
	maps.Copy(currentStorage, map[string]any{
		"host": "redis.example.com", "port": 6380, "database": 2, "ssl": true,
	})
	data, err := json.Marshal(current)
	require.NoError(t, err)
	var observed kkComps.AIGatewayModel
	require.NoError(t, json.Unmarshal(data, &observed))

	var store resources.AIGatewayDatastoreResource
	require.NoError(t, json.Unmarshal([]byte(`{
		"ref":"store-ref","ai_gateway":"gateway","type":"redis-ee","name":"redis-store",
		"display_name":"Redis store","config":{"host":"redis.example.com","port":6380}
	}`), &store))
	rs := &resources.ResourceSet{AIGatewayDatastores: []resources.AIGatewayDatastoreResource{store}}

	data, err = json.Marshal(desired)
	require.NoError(t, err)
	var model resources.AIGatewayModelResource
	require.NoError(t, json.Unmarshal(data, &model))
	model.Ref, model.AIGateway = "affinity-model", "gateway"

	update, _, changed, err := (&Planner{resources: rs}).shouldUpdateAIGatewayModel(
		state.AIGatewayModel{AIGatewayModel: observed}, model,
	)
	require.NoError(t, err)
	require.Falsef(t, update, "datastore-supplied redis connection caused drift: %#v", changed)
}
