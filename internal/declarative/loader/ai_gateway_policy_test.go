package loader

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

const aiGatewayPolicyYAML = `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    policies:
      - ref: mask-sensitive-data
        type: ai-sanitizer
        name: mask-sensitive-data
        display_name: Mask Sensitive Data
        enabled: true
        global: false
        config:
          anonymize:
            - email
`

func TestLoaderExtractsNestedAIGatewayPolicies(t *testing.T) {
	path := writeLoaderTestFile(t, aiGatewayPolicyYAML)

	rs, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGateways, 1)
	require.Empty(t, rs.AIGateways[0].Policies)
	require.Len(t, rs.AIGatewayPolicies, 1)
	require.Equal(t, "support-gateway", rs.AIGatewayPolicies[0].AIGateway)
	require.Equal(t, "mask-sensitive-data", rs.AIGatewayPolicies[0].Name)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"support-gateway",
		resources.ResourceTypeAIGatewayPolicy,
	))
}

func TestLoaderValidatesAIGatewayPolicyParentAndDuplicates(t *testing.T) {
	rootOnly := `
ai_gateway_policies:
  - ref: mask-sensitive-data
    ai_gateway: missing-gateway
    type: ai-sanitizer
    name: mask-sensitive-data
    display_name: Mask Sensitive Data
    config: {anonymize: [email]}
`
	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, rootOnly), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "references unknown ai_gateway")

	duplicates := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    policies:
      - ref: mask-sensitive-data
        type: ai-sanitizer
        name: mask-sensitive-data
        display_name: Mask Sensitive Data
        config: {anonymize: [email]}
      - ref: mask-sensitive-data-2
        type: ai-sanitizer
        name: mask-sensitive-data
        display_name: Mask Sensitive Data 2
        config: {anonymize: [email]}
`
	_, err = New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, duplicates), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate ai_gateway_policy name")
}

func TestLoaderRejectsRootLevelEmptyAIGatewayPolicies(t *testing.T) {
	input := `ai_gateway_policies: []`

	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ai_gateway_policies cannot be empty")
}

func TestLoaderAcceptsAIGatewayPolicyDeferredExternalParentRef(t *testing.T) {
	input := `
ai_gateways:
  - ref: external-shared-gateway
    _external:
      selector:
        matchFields:
          display_name: Shared Gateway
ai_gateway_policies:
  - ref: mask-sensitive-data
    ai_gateway: !ref external-shared-gateway#id
    type: ai-sanitizer
    name: mask-sensitive-data
    display_name: Mask Sensitive Data
    config: {anonymize: [email]}
`

	rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayPolicies, 1)
	require.Equal(t, tags.RefPlaceholderPrefix+"external-shared-gateway#id", rs.AIGatewayPolicies[0].AIGateway)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"external-shared-gateway",
		resources.ResourceTypeAIGatewayPolicy,
	))
}
