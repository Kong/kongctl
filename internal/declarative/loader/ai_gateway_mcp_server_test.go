package loader

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

const aiGatewayMCPServerYAML = `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    mcp_servers:
      - ref: support-tools
        type: conversion-only
        name: support-tools
        display_name: Support Tools
        enabled: true
        config:
          url: https://support-tools.example.com
        tools:
          - name: lookup-customer
            description: Look up a customer profile
            method: GET
            path: /customers/{customer_id}
        policies: []
`

func TestLoaderExtractsNestedAIGatewayMCPServers(t *testing.T) {
	path := writeLoaderTestFile(t, aiGatewayMCPServerYAML)

	rs, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGateways, 1)
	require.Empty(t, rs.AIGateways[0].MCPServers)
	require.Len(t, rs.AIGatewayMCPServers, 1)
	require.Equal(t, "support-gateway", rs.AIGatewayMCPServers[0].AIGateway)
	require.Equal(t, "support-tools", rs.AIGatewayMCPServers[0].Name())
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"support-gateway",
		resources.ResourceTypeAIGatewayMCPServer,
	))
}

func TestLoaderValidatesAIGatewayMCPServerParentAndDuplicates(t *testing.T) {
	rootOnly := `
ai_gateway_mcp_servers:
  - ref: support-tools
    ai_gateway: missing-gateway
    type: conversion-only
    name: support-tools
    display_name: Support Tools
    config: {url: https://support-tools.example.com}
    tools: [{name: lookup-customer, description: Look up a customer profile, method: GET}]
    policies: []
`
	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, rootOnly), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "references unknown ai_gateway")

	duplicates := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    mcp_servers:
      - ref: support-tools
        type: conversion-only
        name: support-tools
        display_name: Support Tools
        config: {url: https://support-tools.example.com}
        tools: [{name: lookup-customer, description: Look up a customer profile, method: GET}]
        policies: []
      - ref: support-tools-2
        type: conversion-only
        name: support-tools
        display_name: Support Tools 2
        config: {url: https://support-tools.example.com}
        tools: [{name: lookup-customer, description: Look up a customer profile, method: GET}]
        policies: []
`
	_, err = New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, duplicates), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate ai_gateway_mcp_server name")
}

func TestLoaderRejectsRootLevelEmptyAIGatewayMCPServers(t *testing.T) {
	input := `ai_gateway_mcp_servers: []`

	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ai_gateway_mcp_servers cannot be empty")
}

func TestLoaderAcceptsAIGatewayMCPServerDeferredExternalParentRef(t *testing.T) {
	input := `
ai_gateways:
  - ref: external-shared-gateway
    _external:
      selector:
        matchFields:
          display_name: Shared Gateway
ai_gateway_mcp_servers:
  - ref: support-tools
    ai_gateway: !ref external-shared-gateway#id
    type: conversion-only
    name: support-tools
    display_name: Support Tools
    config: {url: https://support-tools.example.com}
    tools: [{name: lookup-customer, description: Look up a customer profile, method: GET}]
    policies: []
`

	rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayMCPServers, 1)
	require.Equal(t, tags.RefPlaceholderPrefix+"external-shared-gateway#id", rs.AIGatewayMCPServers[0].AIGateway)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"external-shared-gateway",
		resources.ResourceTypeAIGatewayMCPServer,
	))
}
