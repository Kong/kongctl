package loader

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

const aiGatewayConsumerGroupYAML = `
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
    consumer_groups:
      - ref: premium-support-users
        name: premium-support-users
        display_name: Premium Support Users
        policies:
          - !ref mask-sensitive-data
`

func TestLoaderExtractsNestedAIGatewayConsumerGroups(t *testing.T) {
	path := writeLoaderTestFile(t, aiGatewayConsumerGroupYAML)

	rs, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGateways, 1)
	require.Empty(t, rs.AIGateways[0].ConsumerGroups)
	require.Len(t, rs.AIGatewayConsumerGroups, 1)
	require.Equal(t, "support-gateway", rs.AIGatewayConsumerGroups[0].AIGateway)
	require.Equal(t, "premium-support-users", rs.AIGatewayConsumerGroups[0].Name)
	require.Equal(
		t,
		[]string{tags.RefPlaceholderPrefix + "mask-sensitive-data#id"},
		rs.AIGatewayConsumerGroups[0].Policies,
	)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"support-gateway",
		resources.ResourceTypeAIGatewayConsumerGroup,
	))
}

func TestLoaderValidatesAIGatewayConsumerGroupParentAndDuplicates(t *testing.T) {
	rootOnly := `
ai_gateway_consumer_groups:
  - ref: premium-support-users
    ai_gateway: missing-gateway
    name: premium-support-users
    display_name: Premium Support Users
`
	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, rootOnly), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "references unknown ai_gateway")

	duplicates := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    consumer_groups:
      - ref: premium-support-users
        name: premium-support-users
        display_name: Premium Support Users
      - ref: premium-support-users-2
        name: premium-support-users
        display_name: Premium Support Users 2
`
	_, err = New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, duplicates), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate ai_gateway_consumer_group name")
}

func TestLoaderRejectsRootLevelEmptyAIGatewayConsumerGroups(t *testing.T) {
	input := `ai_gateway_consumer_groups: []`

	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ai_gateway_consumer_groups cannot be empty")
}

func TestLoaderAcceptsAIGatewayConsumerGroupDeferredExternalParentRef(t *testing.T) {
	input := `
ai_gateways:
  - ref: external-shared-gateway
    _external:
      selector:
        matchFields:
          display_name: Shared Gateway
ai_gateway_consumer_groups:
  - ref: premium-support-users
    ai_gateway: !ref external-shared-gateway#id
    name: premium-support-users
    display_name: Premium Support Users
`

	rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayConsumerGroups, 1)
	require.Equal(t, tags.RefPlaceholderPrefix+"external-shared-gateway#id", rs.AIGatewayConsumerGroups[0].AIGateway)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"external-shared-gateway",
		resources.ResourceTypeAIGatewayConsumerGroup,
	))
}
