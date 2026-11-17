package export

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestRootRejectsControlPlaneGroupWithoutObservedMemberships(t *testing.T) {
	current := kkComps.ControlPlane{ID: "group-id", Name: "group", Config: kkComps.ControlPlaneConfig{
		ClusterType: kkComps.ControlPlaneClusterTypeClusterTypeControlPlaneGroup,
	}}
	_, err := Root(current)
	require.ErrorContains(t, err, "verified memberships")
	declaration := MapControlPlaneToDeclarativeResource(current, []string{"member-id"})
	resource, err := Root(&declaration)
	require.NoError(t, err)
	require.Equal(
		t,
		[]resources.ControlPlaneGroupMember{{ID: "member-id"}},
		resource.(*resources.ControlPlaneResource).Members,
	)
}

func TestProjectionPreservesDumpConfigAndLeavesSecretPolicyToCaller(t *testing.T) {
	payload := map[string]any{
		"name": "provider", "display_name": "Provider", "type": "openai",
		"config": map[string]any{"auth": map[string]any{"type": "basic", "headers": []any{
			map[string]any{"name": "Authorization", "value": "existing-config-value"},
		}}},
	}
	declaration, err := ProjectAs[resources.AIGatewayProviderResource](payload)
	require.NoError(t, err)
	require.Equal(t, payload["config"], declaration.Config)
	fields, err := PublicFields(payload)
	require.NoError(t, err)
	OmitSecrets(resources.ResourceTypeAIGatewayProvider, fields, "")
	scrubbed, err := ProjectAs[resources.AIGatewayProviderResource](fields)
	require.NoError(t, err)
	require.NotEqual(t, declaration.Config, scrubbed.Config)
	require.Equal(t, payload["config"], declaration.Config)
}

func TestProjectionOmitsPolicyDatastoreConnections(t *testing.T) {
	payload := func(stores []any) map[string]any {
		return map[string]any{
			"name": "rate-limits", "display_name": "Rate limits", "type": "ai-rate-limiting-advanced",
			"datastores": stores,
			"config": map[string]any{
				"strategy": "redis",
				"redis":    map[string]any{"host": "redis.example.test", "port": 6380},
			},
		}
	}

	declaration, err := ProjectAs[resources.AIGatewayPolicyResource](
		payload([]any{map[string]any{"name": "shared-redis"}}),
	)
	require.NoError(t, err)
	require.Equal(t, []kkComps.AIGatewayDatastoreRef{{Name: "shared-redis"}}, declaration.Datastores)
	require.Equal(t, map[string]any{"strategy": "redis"}, declaration.Config)

	inline, err := ProjectAs[resources.AIGatewayPolicyResource](payload(nil))
	require.NoError(t, err)
	require.Contains(t, inline.Config, "redis")
}

func TestProjectionOmitsNestedPolicyDatastoreConnections(t *testing.T) {
	for _, tt := range []struct {
		policyType string
		config     string
		exported   string
	}{
		{
			policyType: "acme",
			config: `{
				"storage":"redis",
				"storage_config":{"redis":{"host":"redis.example.test"},"shm":{"shm_name":"kong"}}
			}`,
			exported: `{"storage":"redis","storage_config":{"shm":{"shm_name":"kong"}}}`,
		},
		{
			policyType: "datakit",
			config: `{
				"debug":false,
				"resources":{"cache":{"redis":{"host":"redis.example.test"},"memory":{"dictionary_name":"cache"}}}
			}`,
			exported: `{"debug":false,"resources":{"cache":{"memory":{"dictionary_name":"cache"}}}}`,
		},
	} {
		t.Run(tt.policyType, func(t *testing.T) {
			var config, exported map[string]any
			require.NoError(t, json.Unmarshal([]byte(tt.config), &config))
			require.NoError(t, json.Unmarshal([]byte(tt.exported), &exported))
			payload := map[string]any{
				"name": tt.policyType, "display_name": tt.policyType, "type": tt.policyType,
				"config":     config,
				"datastores": []any{map[string]any{"name": "shared-redis"}},
			}
			declaration, err := ProjectAs[resources.AIGatewayPolicyResource](payload)
			require.NoError(t, err)
			require.Equal(t, exported, declaration.Config)
			require.Equal(t, []kkComps.AIGatewayDatastoreRef{{Name: "shared-redis"}}, declaration.Datastores)

			payload["datastores"] = nil
			inline, err := ProjectAs[resources.AIGatewayPolicyResource](payload)
			require.NoError(t, err)
			data, err := json.Marshal(inline.Config)
			require.NoError(t, err)
			require.JSONEq(t, tt.config, string(data))
		})
	}
}

func TestProjectionOmitsModelDatastoreConnections(t *testing.T) {
	model, err := ProjectAs[resources.AIGatewayModelResource](map[string]any{
		"name": "semantic", "display_name": "Semantic", "type": "model",
		"formats":      []any{map[string]any{"type": "openai"}},
		"capabilities": []any{"generate"},
		"targets": []any{map[string]any{
			"name": "gpt-4o", "provider": "provider", "config": map[string]any{"type": "openai"},
		}},
		"datastores": []any{map[string]any{"name": "vector-store"}},
		"config": map[string]any{
			"route": map[string]any{"model": map[string]any{"values": []any{"semantic"}}},
			"balancer": map[string]any{
				"algorithm": "semantic",
				"embeddings": map[string]any{
					"provider": "provider", "name": "text-embedding-3-small", "config": map[string]any{"type": "openai"},
				},
				"vectordb": map[string]any{
					"type": "pgvector", "host": "pg.example.test", "database": "vectors", "dimensions": 1536,
					"distance_metric": "cosine", "threshold": 0.2,
				},
			},
		},
	})
	require.NoError(t, err)
	data, err := model.MutablePayloadMap()
	require.NoError(t, err)
	vector := data[resources.SchemaFieldConfig].(map[string]any)["balancer"].(map[string]any)["vectordb"]
	require.Equal(t, map[string]any{
		"type": "pgvector", "dimensions": float64(1536), "distance_metric": "cosine", "threshold": 0.2,
	}, vector)
}
