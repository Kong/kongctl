package resources

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestEventGatewayVirtualClusterResourceMarshalJSONIncludesTopicAliases(t *testing.T) {
	description := "virtual cluster"
	condition := "context.auth.type == 'anonymous'"
	conflict := kkComps.VirtualClusterTopicAliasConflictWarn

	cluster := EventGatewayVirtualClusterResource{
		CreateVirtualClusterRequest: kkComps.CreateVirtualClusterRequest{
			Name:        "virtual-cluster",
			Description: &description,
			Destination: kkComps.CreateBackendClusterReferenceModifyBackendClusterReferenceByName(
				kkComps.BackendClusterReferenceByName{Name: "backend-cluster"},
			),
			Authentication: []kkComps.VirtualClusterAuthenticationScheme{
				kkComps.CreateVirtualClusterAuthenticationSchemeAnonymous(
					kkComps.VirtualClusterAuthenticationAnonymous{},
				),
			},
			TopicAliases: []kkComps.VirtualClusterTopicAlias{{
				Alias:     "public-orders",
				Topic:     "tenant-a.orders",
				Condition: &condition,
				Conflict:  &conflict,
			}},
			ACLMode:  kkComps.VirtualClusterACLModePassthrough,
			DNSLabel: "vc-default",
		},
		Ref:          "virtual-cluster-ref",
		EventGateway: "event-gateway-ref",
	}

	raw, err := json.Marshal(cluster)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))

	aliases, ok := payload["topic_aliases"].([]any)
	require.True(t, ok)
	require.Len(t, aliases, 1)

	alias, ok := aliases[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "public-orders", alias["alias"])
	require.Equal(t, "tenant-a.orders", alias["topic"])
	require.Equal(t, condition, alias["condition"])
	require.Equal(t, string(conflict), alias["conflict"])
}

func TestEventGatewayVirtualClusterResourceUnmarshalJSONIncludesTopicAliases(t *testing.T) {
	raw := []byte(`{
		"ref": "virtual-cluster-ref",
		"name": "virtual-cluster",
		"destination": {"name": "backend-cluster"},
		"authentication": [{"type": "anonymous"}],
		"topic_aliases": [{
			"alias": "public-orders",
			"topic": "tenant-a.orders",
			"condition": "context.auth.type == 'anonymous'",
			"conflict": "ignore"
		}],
		"acl_mode": "passthrough",
		"dns_label": "vc-default"
	}`)

	var cluster EventGatewayVirtualClusterResource
	require.NoError(t, json.Unmarshal(raw, &cluster))
	require.Len(t, cluster.TopicAliases, 1)

	alias := cluster.TopicAliases[0]
	require.Equal(t, "public-orders", alias.Alias)
	require.Equal(t, "tenant-a.orders", alias.Topic)
	require.NotNil(t, alias.Condition)
	require.Equal(t, "context.auth.type == 'anonymous'", *alias.Condition)
	require.NotNil(t, alias.Conflict)
	require.Equal(t, kkComps.VirtualClusterTopicAliasConflictIgnore, *alias.Conflict)
}
