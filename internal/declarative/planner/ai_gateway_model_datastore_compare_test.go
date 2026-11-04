package planner

import (
	"encoding/json"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

// The response includes connection settings expanded from the referenced
// datastore; the model declaration only owns vector search settings.
const semanticDatastoreModelJSON = `{
	"type":"model", "name":"semantic-model", "display_name":"Semantic model",
	"enabled":true, "formats":[{"type":"openai"}], "capabilities":["generate"],
	"datastores":[{"name":"vector-store"}],
	"config":{
		"route":{"model":{"values":["semantic-model","semantic-alias"]}},
		"balancer":{
			"algorithm":"semantic",
			"embeddings":{"provider":"provider", "name":"text-embedding-3-small", "config":{"type":"openai"}},
			"vectordb":{"type":"pgvector","dimensions":1536,"distance_metric":"cosine","threshold":0.7}
		}
	},
	"targets":[
		{"name":"gpt-4o","provider":"provider","semantic_description":"Business questions",
		 "config":{"type":"openai","input_cost_list":[{"modal":"text","cost":3.5}]}},
		{"name":"gpt-4o-mini","provider":"provider","semantic_description":"CATCHALL","config":{"type":"openai"}}
	]
}`

func semanticDatastoreModelFixture(t *testing.T) (map[string]any, kkComps.AIGatewayModel, *resources.ResourceSet) {
	t.Helper()
	var desired map[string]any
	require.NoError(t, json.Unmarshal([]byte(semanticDatastoreModelJSON), &desired))
	current := normalizeAIGatewayJSONMap(desired)
	current[FieldID] = "model-id"
	current["created_at"] = "2026-01-01T00:00:00Z"
	current["updated_at"] = "2026-01-01T00:00:00Z"
	config := current[FieldConfig].(map[string]any)
	config[FieldModel] = map[string]any{"name_header": true}
	config[FieldRoute].(map[string]any)["protocols"] = []any{"http", "https"}
	balancer := config["balancer"].(map[string]any)
	balancer["failover_criteria"] = []any{"error", "timeout"}
	vector := balancer["vectordb"].(map[string]any)
	maps.Copy(vector, map[string]any{
		"host": "database.example.com", "database": "smoke", "port": 5432,
		"user": "postgres", "timeout": 5000,
		"ssl": map[string]any{"enabled": false, "required": false, "verify": true, "version": "tlsv1_2"},
	})
	data, err := json.Marshal(current)
	require.NoError(t, err)
	var observed kkComps.AIGatewayModel
	require.NoError(t, json.Unmarshal(data, &observed))
	desired[resources.SchemaFieldRef] = "semantic-model"
	desired[resources.SchemaFieldAIGateway] = "gateway"
	desired[FieldDatastores] = []any{map[string]any{FieldName: tags.RefPlaceholderPrefix + "store-ref#name"}}
	var store resources.AIGatewayDatastoreResource
	require.NoError(t, json.Unmarshal([]byte(`{
		"ref":"store-ref","ai_gateway":"gateway","type":"vectordb","name":"vector-store",
		"display_name":"Vector store","config":{"host":"database.example.com","database":"smoke"}
	}`), &store))
	return desired, observed, &resources.ResourceSet{AIGatewayDatastores: []resources.AIGatewayDatastoreResource{store}}
}

func TestAIGatewaySemanticDatastoreComparison(t *testing.T) {
	desired, current, rs := semanticDatastoreModelFixture(t)
	data, err := json.Marshal(desired)
	require.NoError(t, err)
	var model resources.AIGatewayModelResource
	require.NoError(t, json.Unmarshal(data, &model))
	needsUpdate, _, changed, err := (&Planner{resources: rs}).shouldUpdateAIGatewayModel(
		state.AIGatewayModel{AIGatewayModel: current}, model,
	)
	require.NoError(t, err)
	require.Falsef(t, needsUpdate, "unexpected drift: %#v", changed)
}

func TestAIGatewaySemanticDatastorePlansConverge(t *testing.T) {
	for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
		for _, updated := range []bool{false, true} {
			t.Run(string(mode)+map[bool]string{false: "/created", true: "/updated"}[updated], func(t *testing.T) {
				desired, current, rs := semanticDatastoreModelFixture(t)
				if updated {
					desired[FieldDisplayName] = "Updated semantic model"
					current.AIGatewayModelAIGatewayModelModel.DisplayName = "Updated semantic model"
				}
				data, err := json.Marshal(desired)
				require.NoError(t, err)
				var model resources.AIGatewayModelResource
				require.NoError(t, json.Unmarshal(data, &model))
				current = ownedAIGatewayObservation(t, current, "default", false)
				client := state.NewClient(state.ClientConfig{AIGatewayModelAPI: &providerOrderModelAPI{
					testAIGatewayModelAPI: testAIGatewayModelAPI{models: []kkComps.AIGatewayModel{current}},
				}})
				p := NewPlanner(client, slog.Default())
				p.resources = rs
				plan := &Plan{Metadata: PlanMetadata{Mode: mode}}
				require.NoError(t, p.reconcileOwnedAIGatewayModels(t.Context(), "gateway", "gateway", "gateway-id",
					nil, nil, []resources.AIGatewayModelResource{model},
					[]state.AIGatewayModel{{AIGatewayModel: current}}, map[string]bool{"default": true}, plan))
				require.Empty(t, plan.Changes)
			})
		}
	}
}

func TestAIGatewaySemanticDatastorePreservesModelChanges(t *testing.T) {
	for _, tc := range []struct{ old, new, field string }{
		{`"dimensions":1536`, `"dimensions":768`, FieldConfig},
		{`"distance_metric":"cosine"`, `"distance_metric":"euclidean"`, FieldConfig},
		{`"threshold":0.7`, `"threshold":0.8`, FieldConfig},
		{`"provider":"provider"`, `"provider":"other-provider"`, FieldConfig},
		{`"name":"text-embedding-3-small"`, `"name":"text-embedding-3-large"`, FieldConfig},
		{`"config":{"type":"openai"}`, `"config":{"type":"openai","upstream_url":"https://embed.test"}`, FieldConfig},
		{`"values":["semantic-model","semantic-alias"]`, `"values":["other-model"]`, FieldConfig},
		{`"cost":3.5`, `"cost":4.5`, FieldTargets},
		{`"semantic_description":"Business questions"`, `"semantic_description":"Different questions"`, FieldTargets},
		{`"name":"vector-store"`, `"name":"other-store"`, FieldDatastores},
		{`"threshold":0.7`, `"threshold":0.7,"host":"127.0.0.1"`, FieldConfig},
		{`"threshold":0.7`, `"threshold":0.7,"database":"kong-pgvector"`, FieldConfig},
		{`"threshold":0.7`, `"threshold":0.7,"port":5433`, FieldConfig},
		{`"threshold":0.7`, `"threshold":0.7,"timeout":1000`, FieldConfig},
		{`"threshold":0.7`, `"threshold":0.7,"user":"custom-user"`, FieldConfig},
		{`"threshold":0.7`, `"threshold":0.7,"ssl":{"enabled":true}`, FieldConfig},
		{`"threshold":0.7`, `"threshold":0.7,"ssl":{"verify":false}`, FieldConfig},
	} {
		t.Run(tc.new, func(t *testing.T) {
			_, current, rs := semanticDatastoreModelFixture(t)
			var model resources.AIGatewayModelResource
			require.NoError(t, json.Unmarshal([]byte(strings.Replace(semanticDatastoreModelJSON, tc.old, tc.new, 1)), &model))
			model.Ref, model.AIGateway = "semantic-model", "gateway"
			update, fields, changed, err := (&Planner{resources: rs}).shouldUpdateAIGatewayModel(
				state.AIGatewayModel{AIGatewayModel: current}, model,
			)
			require.NoError(t, err)
			require.True(t, update)
			require.Contains(t, changed, tc.field)
			require.NotEmpty(t, fields)
		})
	}
}

func TestAIGatewaySemanticDatastoreExplicitConnectionSettingsConverge(t *testing.T) {
	_, current, rs := semanticDatastoreModelFixture(t)
	input := strings.Replace(semanticDatastoreModelJSON, `"threshold":0.7`,
		`"threshold":0.7,"host":"database.example.com","database":"smoke",`+
			`"port":5432,"user":"postgres","timeout":5000,"ssl":{"enabled":false}`, 1)
	var model resources.AIGatewayModelResource
	require.NoError(t, json.Unmarshal([]byte(input), &model))
	model.Ref, model.AIGateway = "semantic-model", "gateway"
	update, _, changed, err := (&Planner{resources: rs}).shouldUpdateAIGatewayModel(
		state.AIGatewayModel{AIGatewayModel: current}, model,
	)
	require.NoError(t, err)
	require.Falsef(t, update, "explicit settings drift: %#v", changed)
}

func TestAIGatewaySemanticDatastoreDumpRoundTrip(t *testing.T) {
	_, current, rs := semanticDatastoreModelFixture(t)
	exported, err := resources.AIGatewayModelResourceFromResponse("gateway", current)
	require.NoError(t, err)
	payload, err := exported.MutablePayloadMap()
	require.NoError(t, err)
	vector := payload[FieldConfig].(map[string]any)[FieldBalancer].(map[string]any)["vectordb"].(map[string]any)
	for _, key := range []string{"host", "database", "port", "user", "timeout", FieldSSL} {
		require.NotContains(t, vector, key, "dump must not pin inherited connection fields")
	}
	rs.AIGatewayModels = []resources.AIGatewayModelResource{exported}
	rs.AIGateways = []resources.AIGatewayResource{{
		BaseResource: resources.BaseResource{Ref: "gateway"},
		External:     &resources.ExternalBlock{ID: "gateway-id"},
	}}
	data, err := yaml.Marshal(rs)
	require.NoError(t, err)
	for _, omitDefaults := range []bool{false, true} {
		t.Run(map[bool]string{false: "full dump", true: "omit defaults"}[omitDefaults], func(t *testing.T) {
			input := data
			if omitDefaults {
				input, err = resources.OmitAPIDefaults(input)
				require.NoError(t, err)
			}
			path := filepath.Join(t.TempDir(), "dump.yaml")
			require.NoError(t, os.WriteFile(path, input, 0o600))
			reloaded, err := loader.New().LoadFromSources([]loader.Source{{Path: path, Type: loader.SourceTypeFile}}, false)
			require.NoError(t, err)
			require.Len(t, reloaded.AIGatewayModels, 1)
			update, _, changed, err := (&Planner{resources: reloaded}).shouldUpdateAIGatewayModel(
				state.AIGatewayModel{AIGatewayModel: current}, reloaded.AIGatewayModels[0],
			)
			require.NoError(t, err)
			require.Falsef(t, update, "dump round-trip drift: %#v", changed)

			// Changing the datastore after a dump must not leave the model
			// pinned to the old, expanded connection settings.
			data, err := json.Marshal(current)
			require.NoError(t, err)
			var observed map[string]any
			require.NoError(t, json.Unmarshal(data, &observed))
			vector := observed[FieldConfig].(map[string]any)[FieldBalancer].(map[string]any)["vectordb"].(map[string]any)
			vector["host"], vector["database"] = "new.example.com", "new-database"
			reloaded.AIGatewayDatastores[0].VectorDBDatastore.Config.Host = "new.example.com"
			reloaded.AIGatewayDatastores[0].VectorDBDatastore.Config.Database = "new-database"
			data, err = json.Marshal(observed)
			require.NoError(t, err)
			var updated kkComps.AIGatewayModel
			require.NoError(t, json.Unmarshal(data, &updated))
			update, _, changed, err = (&Planner{resources: reloaded}).shouldUpdateAIGatewayModel(
				state.AIGatewayModel{AIGatewayModel: updated}, reloaded.AIGatewayModels[0],
			)
			require.NoError(t, err)
			require.Falsef(t, update, "dump pinned old datastore connection settings: %#v", changed)
		})
	}
}

func TestAIGatewaySemanticDatastoreComparisonDefaults(t *testing.T) {
	desired, current, _ := semanticDatastoreModelFixture(t)
	data, err := json.Marshal(desired)
	require.NoError(t, err)
	var model resources.AIGatewayModelResource
	require.NoError(t, json.Unmarshal(data, &model))
	currentPayload, err := resources.AIGatewayModelMutablePayloadMap(current)
	require.NoError(t, err)
	desiredPayload, err := model.MutablePayloadMap()
	require.NoError(t, err)
	currentCompare, desiredCompare := normalizeAIGatewayPayloadsForComparison(currentPayload, desiredPayload)
	currentConfig := currentCompare[FieldConfig].(map[string]any)
	desiredConfig := desiredCompare[FieldConfig].(map[string]any)
	currentVector := currentConfig["balancer"].(map[string]any)["vectordb"].(map[string]any)
	desiredVector := desiredConfig["balancer"].(map[string]any)["vectordb"].(map[string]any)
	require.NotContains(t, currentConfig, FieldModel)
	require.NotContains(t, currentConfig[FieldRoute], "protocols")
	require.NotContains(t, currentConfig["balancer"], "failover_criteria")
	// The patched SDK preserves omission. The API still expands datastore
	// connection settings, which require model-specific comparison handling.
	require.NotContains(t, desiredVector, "host")
	require.NotContains(t, desiredVector, "database")
	require.NotEqual(t, desiredVector, currentVector)
	for _, key := range []string{"host", "database", "port", "user", "timeout", "ssl"} {
		delete(currentVector, key)
	}
	require.Equal(t, desiredConfig, currentConfig)
}

func TestAIGatewaySemanticDatastoreComparisonScope(t *testing.T) {
	for _, tc := range []struct {
		name       string
		parent     string
		gatewayIDs []string
		declared   bool
		wantDrift  bool
	}{
		{name: "undeclared store"},
		{name: "different gateway", parent: "other", declared: true},
		{name: "unresolved alias", parent: "other", declared: true, gatewayIDs: []string{"", ""}},
		{
			name: "different gateway IDs", parent: "other", declared: true,
			gatewayIDs: []string{"gateway-id", "different-id"},
		},
		{
			name: "same gateway aliases", parent: "other", declared: true,
			gatewayIDs: []string{"gateway-id", "gateway-id"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, current, rs := semanticDatastoreModelFixture(t)
			var model resources.AIGatewayModelResource
			require.NoError(t, json.Unmarshal([]byte(semanticDatastoreModelJSON), &model))
			model.Ref, model.AIGateway = "semantic-model", "gateway"
			if !tc.declared {
				rs.AIGatewayDatastores = nil
			} else {
				rs.AIGatewayDatastores[0].AIGateway = tc.parent
			}
			if tc.gatewayIDs != nil {
				for i, ref := range []string{"gateway", "other"} {
					gateway := resources.AIGatewayResource{BaseResource: resources.BaseResource{Ref: ref}}
					gateway.SetKonnectID(tc.gatewayIDs[i])
					rs.AIGateways = append(rs.AIGateways, gateway)
				}
			}
			update, _, changed, err := (&Planner{resources: rs}).shouldUpdateAIGatewayModel(
				state.AIGatewayModel{AIGatewayModel: current}, model,
			)
			require.NoError(t, err)
			require.Equal(t, tc.wantDrift, update, "changed fields: %#v", changed)
		})
	}
}

func TestAIGatewaySemanticDatastorePasswordDoesNotCauseDrift(t *testing.T) {
	_, current, rs := semanticDatastoreModelFixture(t)
	input := strings.Replace(semanticDatastoreModelJSON, `"threshold":0.7`,
		`"threshold":0.7,"password":"example-write-only-password"`, 1)
	var model resources.AIGatewayModelResource
	require.NoError(t, json.Unmarshal([]byte(input), &model))
	model.Ref, model.AIGateway = "semantic-model", "gateway"
	update, _, changed, err := (&Planner{resources: rs}).shouldUpdateAIGatewayModel(
		state.AIGatewayModel{AIGatewayModel: current}, model,
	)
	require.NoError(t, err)
	require.Falsef(t, update, "write-only password caused drift: %#v", changed)
}

func TestAIGatewayInlinePgvectorDisabledSSLUsesAPIDefault(t *testing.T) {
	for _, required := range []bool{true, false} {
		t.Run(map[bool]string{true: "API default", false: "non-default drift"}[required], func(t *testing.T) {
			_, current, _ := semanticDatastoreModelFixture(t)
			data, err := json.Marshal(current)
			require.NoError(t, err)
			var observed map[string]any
			require.NoError(t, json.Unmarshal(data, &observed))
			delete(observed, FieldDatastores)
			vector := observed[FieldConfig].(map[string]any)[FieldBalancer].(map[string]any)["vectordb"].(map[string]any)
			vector[FieldSSL].(map[string]any)["required"] = required
			data, err = json.Marshal(observed)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(data, &current))
			var desired map[string]any
			require.NoError(t, json.Unmarshal([]byte(semanticDatastoreModelJSON), &desired))
			delete(desired, FieldDatastores)
			desiredVector := desired[FieldConfig].(map[string]any)[FieldBalancer].(map[string]any)["vectordb"].(map[string]any)
			desiredVector["host"], desiredVector["database"] = "database.example.com", "smoke"
			desiredVector[FieldSSL] = map[string]any{"enabled": false}
			data, err = json.Marshal(desired)
			require.NoError(t, err)
			var model resources.AIGatewayModelResource
			require.NoError(t, json.Unmarshal(data, &model))
			update, _, changed, err := (&Planner{}).shouldUpdateAIGatewayModel(
				state.AIGatewayModel{AIGatewayModel: current}, model,
			)
			require.NoError(t, err)
			require.Equal(t, !required, update, "changed fields: %#v", changed)
		})
	}
}

func TestAIGatewayInlinePgvectorDefaults(t *testing.T) {
	for _, customHost := range []bool{false, true} {
		t.Run(map[bool]string{false: "defaults converge", true: "custom host drifts"}[customHost], func(t *testing.T) {
			var payload map[string]any
			require.NoError(t, json.Unmarshal([]byte(semanticDatastoreModelJSON), &payload))
			delete(payload, FieldDatastores)
			currentPayload := normalizeAIGatewayJSONMap(payload)
			currentPayload[FieldID] = "model-id"
			currentPayload["created_at"], currentPayload["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
			vector := currentPayload[FieldConfig].(map[string]any)["balancer"].(map[string]any)["vectordb"].(map[string]any)
			maps.Copy(vector, map[string]any{
				"host": "127.0.0.1", "database": "kong-pgvector", "port": 5432, "user": "postgres", "timeout": 5000,
				"ssl": map[string]any{"enabled": true, "required": true, "verify": true, "version": "tlsv1_2"},
			})
			if customHost {
				vector["host"] = "custom.example.com"
			}
			data, err := json.Marshal(currentPayload)
			require.NoError(t, err)
			var current kkComps.AIGatewayModel
			require.NoError(t, json.Unmarshal(data, &current))
			exported, err := resources.AIGatewayModelResourceFromResponse("gateway", current)
			require.NoError(t, err)
			exportedPayload, err := exported.MutablePayloadMap()
			require.NoError(t, err)
			exportedConfig := exportedPayload[FieldConfig].(map[string]any)
			exportedVector := exportedConfig[FieldBalancer].(map[string]any)["vectordb"].(map[string]any)
			require.Equal(t, vector["host"], exportedVector["host"], "inline connections must remain in dumps")
			data, err = json.Marshal(payload)
			require.NoError(t, err)
			var model resources.AIGatewayModelResource
			require.NoError(t, json.Unmarshal(data, &model))
			update, _, changed, err := (&Planner{}).shouldUpdateAIGatewayModel(
				state.AIGatewayModel{AIGatewayModel: current}, model,
			)
			require.NoError(t, err)
			require.Equal(t, customHost, update, "changed fields: %#v", changed)
		})
	}
}
