package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

const aiGatewayModelYAML = `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    models:
      - ref: support-gpt
        type: model
        name: support-gpt
        display_name: Support GPT
        enabled: true
        config:
          route: {}
          model: {}
        formats:
          - type: openai
        targets:
          - name: gpt-4o
            provider: support-openai
            config:
              type: openai
        policies: []
        capabilities:
          - generate
`

func TestLoaderExtractsNestedAIGatewayModels(t *testing.T) {
	path := writeLoaderTestFile(t, aiGatewayModelYAML)

	rs, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGateways, 1)
	require.Empty(t, rs.AIGateways[0].Models)
	require.Len(t, rs.AIGatewayModels, 1)
	require.Equal(t, "support-gateway", rs.AIGatewayModels[0].AIGateway)
	require.Equal(t, "support-gpt", rs.AIGatewayModels[0].Name())
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"support-gateway",
		resources.ResourceTypeAIGatewayModel,
	))
}

func TestLoaderAcceptsDottedAIGatewayModelRef(t *testing.T) {
	input := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    models:
      - ref: gpt-5.4
        type: model
        name: gpt-5.4
        display_name: GPT 5.4
        config: {route: {}, model: {}}
        formats: [{type: openai}]
        targets: [{name: gpt-4o, provider: support-openai, config: {type: openai}}]
        policies: []
        capabilities: [generate]
`

	rs, err := New().LoadFromSources(
		[]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}},
		false,
	)
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayModels, 1)
	require.Equal(t, "gpt-5.4", rs.AIGatewayModels[0].Ref)
	require.Equal(t, "gpt-5.4", rs.AIGatewayModels[0].Name())
}

func TestLoaderValidatesAIGatewayModelParentAndDuplicates(t *testing.T) {
	rootOnly := `
ai_gateway_models:
  - ref: support-gpt
    ai_gateway: missing-gateway
    type: model
    name: support-gpt
    display_name: Support GPT
    config: {route: {}, model: {}}
    formats: [{type: openai}]
    targets: [{name: gpt-4o, provider: support-openai, config: {type: openai}}]
    policies: []
    capabilities: [generate]
`
	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, rootOnly), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "references unknown ai_gateway")

	duplicates := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    models:
      - ref: support-gpt
        type: model
        name: support-gpt
        display_name: Support GPT
        config: {route: {}, model: {}}
        formats: [{type: openai}]
        targets: [{name: gpt-4o, provider: support-openai, config: {type: openai}}]
        policies: []
        capabilities: [generate]
      - ref: support-gpt-2
        type: model
        name: support-gpt
        display_name: Support GPT 2
        config: {route: {}, model: {}}
        formats: [{type: openai}]
        targets: [{name: gpt-4o, provider: support-openai, config: {type: openai}}]
        policies: []
        capabilities: [generate]
`
	_, err = New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, duplicates), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate ai_gateway_model name")
}

func TestLoaderRejectsRootLevelEmptyAIGatewayModels(t *testing.T) {
	input := `ai_gateway_models: []`

	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ai_gateway_models cannot be empty")
}

func TestLoaderAcceptsAIGatewayModelDeferredExternalParentRef(t *testing.T) {
	input := `
ai_gateways:
  - ref: external-shared-gateway
    _external:
      selector:
        matchFields:
          display_name: Shared Gateway
ai_gateway_models:
  - ref: support-gpt
    ai_gateway: !ref external-shared-gateway#id
    type: model
    name: support-gpt
    display_name: Support GPT
    config: {route: {}, model: {}}
    formats: [{type: openai}]
    targets: [{name: gpt-4o, provider: support-openai, config: {type: openai}}]
    policies: []
    capabilities: [generate]
`

	rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayModels, 1)
	require.Equal(t, tags.RefPlaceholderPrefix+"external-shared-gateway#id", rs.AIGatewayModels[0].AIGateway)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"external-shared-gateway",
		resources.ResourceTypeAIGatewayModel,
	))
}

func writeLoaderTestFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kongctl.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}
