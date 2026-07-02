package loader

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

const aiGatewayConsumerYAML = `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    policies:
      - ref: mask-sensitive-data
        type: ai-sanitizer
        name: mask-sensitive-data
        display_name: Mask Sensitive Data
        config:
          anonymize:
            - email
    consumers:
      - ref: support-user
        name: support-user
        type: api-key
        display_name: Support User
        policies:
          - !ref mask-sensitive-data
`

func TestLoaderExtractsNestedAIGatewayConsumers(t *testing.T) {
	path := writeLoaderTestFile(t, aiGatewayConsumerYAML)

	rs, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGateways, 1)
	require.Empty(t, rs.AIGateways[0].Consumers)
	require.Len(t, rs.AIGatewayConsumers, 1)
	require.Equal(t, "support-gateway", rs.AIGatewayConsumers[0].AIGateway)
	require.Equal(t, "support-user", rs.AIGatewayConsumers[0].Name)
	require.Equal(t, "api-key", string(rs.AIGatewayConsumers[0].Type))
	require.Equal(
		t,
		[]string{tags.RefPlaceholderPrefix + "mask-sensitive-data#id"},
		rs.AIGatewayConsumers[0].Policies,
	)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"support-gateway",
		resources.ResourceTypeAIGatewayConsumer,
	))
}

func TestLoaderExtractsNestedAIGatewayConsumerCredentials(t *testing.T) {
	input := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    consumers:
      - ref: support-user
        name: support-user
        type: api-key
        display_name: Support User
        credentials:
          - ref: support-user-key
            name: support-user-key
            type: api-key
            display_name: Support User API Key
            ttl: 60
`

	rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayConsumers, 1)
	require.Empty(t, rs.AIGatewayConsumers[0].Credentials)
	require.Len(t, rs.AIGatewayConsumerCredentials, 1)
	require.Equal(t, "support-user", rs.AIGatewayConsumerCredentials[0].AIGatewayConsumer)
	require.Equal(t, "support-user-key", rs.AIGatewayConsumerCredentials[0].Name)
	require.Equal(t, "api-key", string(rs.AIGatewayConsumerCredentials[0].Type))
	require.NotNil(t, rs.AIGatewayConsumerCredentials[0].TTL)
	require.Equal(t, int64(60), *rs.AIGatewayConsumerCredentials[0].TTL)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGatewayConsumer,
		"support-user",
		resources.ResourceTypeAIGatewayConsumerCredential,
	))
}

func TestLoaderValidatesAIGatewayConsumerParentAndDuplicates(t *testing.T) {
	rootOnly := `
ai_gateway_consumers:
  - ref: support-user
    ai_gateway: missing-gateway
    name: support-user
    type: api-key
    display_name: Support User
`
	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, rootOnly), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "references unknown ai_gateway")

	duplicates := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    consumers:
      - ref: support-user
        name: support-user
        type: api-key
        display_name: Support User
      - ref: support-user-2
        name: support-user
        type: api-key
        display_name: Support User 2
`
	_, err = New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, duplicates), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate ai_gateway_consumer name")
}

func TestLoaderValidatesAIGatewayConsumerCredentialParentAndDuplicates(t *testing.T) {
	rootOnly := `
ai_gateway_consumer_credentials:
  - ref: support-user-key
    ai_gateway_consumer: missing-consumer
    name: support-user-key
    type: api-key
    display_name: Support User API Key
`
	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, rootOnly), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "references unknown ai_gateway_consumer")

	duplicates := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    consumers:
      - ref: support-user
        name: support-user
        type: api-key
        display_name: Support User
        credentials:
          - ref: support-user-key
            name: support-user-key
            type: api-key
            display_name: Support User API Key
          - ref: support-user-key-2
            name: support-user-key
            type: api-key
            display_name: Support User API Key 2
`
	_, err = New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, duplicates), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate ai_gateway_consumer_credential name")
}

func TestLoaderRejectsRootLevelEmptyAIGatewayConsumers(t *testing.T) {
	input := `ai_gateway_consumers: []`

	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ai_gateway_consumers cannot be empty")
}

func TestLoaderRejectsRootLevelEmptyAIGatewayConsumerCredentials(t *testing.T) {
	input := `ai_gateway_consumer_credentials: []`

	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ai_gateway_consumer_credentials cannot be empty")
}

func TestLoaderAcceptsAIGatewayConsumerDeferredExternalParentRef(t *testing.T) {
	input := `
ai_gateways:
  - ref: external-shared-gateway
    _external:
      selector:
        matchFields:
          display_name: Shared Gateway
ai_gateway_consumers:
  - ref: support-user
    ai_gateway: !ref external-shared-gateway#id
    name: support-user
    type: api-key
    display_name: Support User
`

	rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayConsumers, 1)
	require.Equal(t, tags.RefPlaceholderPrefix+"external-shared-gateway#id", rs.AIGatewayConsumers[0].AIGateway)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"external-shared-gateway",
		resources.ResourceTypeAIGatewayConsumer,
	))
}
