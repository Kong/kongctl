package planner

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

// The API replaces a policy's connection block with the referenced datastore's
// settings; these observed values mirror an AI Gateway 2.3 response.
var observedRedisDatastoreConnection = map[string]any{
	"host":        "redis.example.test",
	"port":        float64(6380),
	"database":    float64(2),
	"username":    "kong",
	"ssl":         true,
	"ssl_verify":  false,
	"server_name": "redis.example.test",
}

func TestAIGatewayPolicyPlannerIgnoresRedisDatastoreConnection(t *testing.T) {
	desired := testAIGatewayDatastorePolicyResource(t, `{
		"strategy": "redis",
		"sync_rate": 0,
		"policies": [{"limits": [{"limit": 20, "window_size": 60}]}]
	}`, "shared-redis")
	current := testAIGatewayDatastorePolicy("shared-redis", map[string]any{
		"strategy":  "redis",
		"sync_rate": float64(0),
		"policies":  []any{map[string]any{"limits": []any{map[string]any{"limit": float64(20), "window_size": float64(60)}}}},
		"redis":     observedRedisDatastoreConnection,
	})

	needsUpdate, _, changed, err := shouldUpdateAIGatewayPolicy(state.AIGatewayPolicy{AIGatewayPolicy: current}, desired)

	require.NoError(t, err)
	require.Falsef(t, needsUpdate, "changed fields: %#v", changed)
}

func TestAIGatewayPolicyPlannerIgnoresInlineRedisReplacedByDatastore(t *testing.T) {
	desired := testAIGatewayDatastorePolicyResource(t, `{
		"strategy": "redis",
		"redis": {"host": "inline.example.test", "keepalive_pool_size": 50}
	}`, "shared-redis")
	current := testAIGatewayDatastorePolicy("shared-redis", map[string]any{
		"strategy": "redis",
		"redis":    observedRedisDatastoreConnection,
	})

	needsUpdate, _, changed, err := shouldUpdateAIGatewayPolicy(state.AIGatewayPolicy{AIGatewayPolicy: current}, desired)

	require.NoError(t, err)
	require.Falsef(t, needsUpdate, "changed fields: %#v", changed)
}

func TestAIGatewayPolicyPlannerIgnoresVectorDBDatastoreConnection(t *testing.T) {
	desired := testAIGatewayDatastorePolicyResource(t, `{
		"vectordb": {"strategy": "pgvector", "dimensions": 1536, "distance_metric": "cosine", "threshold": 0.1}
	}`, "vector-store")
	desired.Type = "ai-semantic-cache"
	current := testAIGatewayDatastorePolicy("vector-store", map[string]any{
		"vectordb": map[string]any{
			"strategy":        "pgvector",
			"dimensions":      float64(1536),
			"distance_metric": "cosine",
			"threshold":       0.1,
			"pgvector": map[string]any{
				"host": "pg.example.test", "port": float64(5433), "database": "vectors", "user": "kong",
				"ssl": false, "ssl_required": false, "ssl_verify": true, "ssl_version": "tlsv1_2",
				"timeout": float64(5000),
			},
		},
	})
	current.Type = "ai-semantic-cache"

	needsUpdate, _, changed, err := shouldUpdateAIGatewayPolicy(state.AIGatewayPolicy{AIGatewayPolicy: current}, desired)

	require.NoError(t, err)
	require.Falsef(t, needsUpdate, "changed fields: %#v", changed)
}

func TestAIGatewayPolicyPlannerComparesDatastoreReferences(t *testing.T) {
	desired := testAIGatewayDatastorePolicyResource(t, `{"strategy": "redis"}`, "other-redis")
	current := testAIGatewayDatastorePolicy("shared-redis", map[string]any{
		"strategy": "redis",
		"redis":    observedRedisDatastoreConnection,
	})

	needsUpdate, fields, changed, err := shouldUpdateAIGatewayPolicy(
		state.AIGatewayPolicy{AIGatewayPolicy: current},
		desired,
	)

	require.NoError(t, err)
	require.True(t, needsUpdate)
	require.Contains(t, changed, FieldDatastores)
	require.Equal(t, []any{map[string]any{"name": "other-redis"}}, fields[FieldDatastores])
}

func TestAIGatewayPolicyPlannerComparesInlineRedisWithoutDatastore(t *testing.T) {
	desired := testAIGatewayDatastorePolicyResource(t, `{
		"strategy": "redis",
		"redis": {"host": "redis-b.example.test"}
	}`, "")
	current := testAIGatewayDatastorePolicy("", map[string]any{
		"strategy": "redis",
		"redis":    map[string]any{"host": "redis-a.example.test", "port": float64(6379)},
	})

	needsUpdate, _, changed, err := shouldUpdateAIGatewayPolicy(state.AIGatewayPolicy{AIGatewayPolicy: current}, desired)

	require.NoError(t, err)
	require.True(t, needsUpdate)
	require.Contains(t, changed, FieldConfig)
}

func TestAIGatewayPolicyPlannerComparesNestedDatastoreConnections(t *testing.T) {
	for _, tt := range []struct {
		policyType string
		config     string
		parentPath []string
		field      string
		newValue   any
	}{
		{
			policyType: "acme",
			config: `{
				"account_email":"original@example.test","storage":"redis",
				"storage_config":{"redis":{"host":"inline.example.test","port":6379,"timeout":1234}}
			}`,
			parentPath: []string{"storage_config"},
			field:      "account_email",
			newValue:   "updated@example.test",
		},
		{
			policyType: "datakit",
			config: `{
				"debug":false,
				"resources":{"cache":{"redis":{"host":"inline.example.test","port":6379,"timeout":1234}}}
			}`,
			parentPath: []string{"resources", "cache"},
			field:      "debug",
			newValue:   true,
		},
	} {
		t.Run(tt.policyType, func(t *testing.T) {
			for _, datastore := range []string{"shared-redis", ""} {
				name := "referenced"
				if datastore == "" {
					name = "inline"
				}
				t.Run(name, func(t *testing.T) {
					desired := testAIGatewayDatastorePolicyResource(t, tt.config, datastore)
					desired.Type = tt.policyType
					var config map[string]any
					require.NoError(t, json.Unmarshal([]byte(tt.config), &config))
					parent := config
					for _, key := range tt.parentPath {
						parent = parent[key].(map[string]any)
					}
					parent["redis"] = observedRedisDatastoreConnection
					current := state.AIGatewayPolicy{AIGatewayPolicy: testAIGatewayDatastorePolicy(datastore, config)}
					current.Type = tt.policyType

					needsUpdate, _, changed, err := shouldUpdateAIGatewayPolicy(current, desired)
					require.NoError(t, err)
					require.Equal(t, datastore == "", needsUpdate, "changed fields: %#v", changed)

					desired.Config[tt.field] = tt.newValue
					needsUpdate, fields, changed, err := shouldUpdateAIGatewayPolicy(current, desired)
					require.NoError(t, err)
					require.True(t, needsUpdate)
					require.Contains(t, changed, FieldConfig)
					require.Equal(t, tt.newValue, fields[FieldConfig].(map[string]any)[tt.field])
				})
			}
		})
	}
}

func TestAIGatewayPolicyPlannerIgnoresScrubbedSemanticConnections(t *testing.T) {
	for _, strategy := range []string{"redis", "pgvector"} {
		t.Run(strategy, func(t *testing.T) {
			desired := testAIGatewayDatastorePolicyResource(t, `{
				"vectordb": {
					"redis":{"host":"inline.example.test","database":8,"ssl":true,"ssl_verify":false},
					"pgvector":{"host":"inline-pg.example.test","database":"inline","timeout":1234},
					"dimensions":1536,"distance_metric":"cosine","threshold":0.2
				}
			}`, "shared-store")
			desired.Type = "ai-semantic-cache"
			desired.Config["vectordb"].(map[string]any)["strategy"] = strategy
			current := testAIGatewayDatastorePolicy("shared-store", map[string]any{
				"vectordb": map[string]any{
					"strategy": strategy, "dimensions": float64(1536), "distance_metric": "cosine", "threshold": 0.2,
					strategy: map[string]any{"host": "datastore.example.test", "database": "from-datastore"},
				},
			})
			current.Type = desired.Type
			needsUpdate, _, changed, err := shouldUpdateAIGatewayPolicy(state.AIGatewayPolicy{AIGatewayPolicy: current}, desired)
			require.NoError(t, err)
			require.Falsef(t, needsUpdate, "changed fields: %#v", changed)

			desired.Config["vectordb"].(map[string]any)["threshold"] = 0.4
			needsUpdate, _, changed, err = shouldUpdateAIGatewayPolicy(state.AIGatewayPolicy{AIGatewayPolicy: current}, desired)
			require.NoError(t, err)
			require.True(t, needsUpdate)
			require.Contains(t, changed, FieldConfig)
		})
	}
}

func TestAIGatewayPolicyPlannerPrunesContainersEmptiedByDatastoreOmission(t *testing.T) {
	for _, tt := range []struct {
		policyType string
		config     string
	}{
		{"acme", `{"storage_config":{"redis":{"host":"inline.example.test"}}}`},
		{"datakit", `{"resources":{"cache":{"redis":{"host":"inline.example.test"}}}}`},
	} {
		t.Run(tt.policyType, func(t *testing.T) {
			desired := testAIGatewayDatastorePolicyResource(t, tt.config, "shared-redis")
			desired.Type = tt.policyType
			current := testAIGatewayDatastorePolicy("shared-redis", map[string]any{})
			current.Type = tt.policyType
			needsUpdate, _, changed, err := shouldUpdateAIGatewayPolicy(state.AIGatewayPolicy{AIGatewayPolicy: current}, desired)
			require.NoError(t, err)
			require.Falsef(t, needsUpdate, "changed fields: %#v", changed)
		})
	}
}

func testAIGatewayDatastorePolicyResource(
	t *testing.T,
	config string,
	datastore string,
) resources.AIGatewayPolicyResource {
	t.Helper()
	payload := map[string]any{
		"ref":          "rate-limits",
		"ai_gateway":   "support-gateway",
		"type":         "ai-rate-limiting-advanced",
		"name":         "rate-limits",
		"display_name": "Rate limits",
		"enabled":      true,
		"global":       false,
		"config":       json.RawMessage(config),
	}
	if datastore != "" {
		payload["datastores"] = []map[string]string{{"name": datastore}}
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var policy resources.AIGatewayPolicyResource
	require.NoError(t, json.Unmarshal(data, &policy))
	return policy
}

func testAIGatewayDatastorePolicy(datastore string, config map[string]any) kkComps.AIGatewayPolicy {
	enabled := true
	global := false
	policy := kkComps.AIGatewayPolicy{
		ManagedBy:   resources.BuildChildManagedBy("default", false, "dev"),
		ID:          "rate-limits-id",
		Name:        "rate-limits",
		Type:        "ai-rate-limiting-advanced",
		DisplayName: "Rate limits",
		Enabled:     &enabled,
		Global:      &global,
		Config:      config,
	}
	if datastore != "" {
		policy.Datastores = []kkComps.AIGatewayDatastoreRef{{Name: datastore}}
	}
	return policy
}
