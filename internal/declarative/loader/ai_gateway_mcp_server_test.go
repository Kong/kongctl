package loader

import (
	"fmt"
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

func TestLoaderExtractsIssue1499NestedAIGatewayMCPServers(t *testing.T) {
	input := `
ai_gateways:
  - ref: poc-default-ai-gateway
    display_name: POC Default AI Gateway
    auth_strategies:
      - ref: poc-key-auth
        name: poc-key-auth
        type: key-auth
        display_name: POC Key Auth
        config:
          key_names: [apikey]
    mcp_servers:
      - ref: poc-mcp-conversion
        type: conversion-only
        name: poc-mcp-conversion
        display_name: POC MCP Conversion-Only (httpbin)
        enabled: true
        tools:
          - name: get-status
            description: Return the processing status for a given request ID.
            method: GET
            path: /status/{requestId}
            parameters:
              - name: requestId
                in: path
                required: true
                description: The unique request identifier
                schema: {type: string}
        config:
          url: https://httpbin.konghq.com/anything
          route:
            paths: [/mcp-conversion]
            methods: [GET, POST]
      - ref: poc-mcp-server
        type: passthrough-listener
        name: poc-mcp-server
        display_name: POC MCP Server (Context7 passthrough)
        enabled: true
        policies: []
        tools: []
        access:
          acl_attribute_type: oauth_access_token
          access_token_claim_field: sub
        config:
          url: https://mcp.context7.com/mcp
          route:
            paths: [/mcp]
            methods: [GET, POST]
      - ref: poc-mcp-conversion-listener
        type: conversion-listener
        name: poc-mcp-conversion-listener
        display_name: POC MCP Conversion Listener
        enabled: true
        tools:
          - name: get-conversion-status
            description: Get conversion status
            method: GET
            path: /status
        access:
          acl_attribute_type: consumer
          auth_strategies:
            - !ref poc-key-auth
        config:
          url: https://httpbin.konghq.com/anything
          route:
            paths: [/mcp-conversion-listener]
            methods: [GET, POST]
      - ref: poc-mcp-listener
        type: listener
        name: poc-mcp-listener
        display_name: POC MCP Listener
        enabled: true
        sources: [poc-mcp-conversion, poc-mcp-upstream]
        access:
          acl_attribute_type: consumer
        config:
          route:
            paths: [/mcp-listener]
            methods: [GET, POST]
      - ref: poc-mcp-upstream
        type: upstream-server
        name: poc-mcp-upstream
        display_name: POC MCP Upstream Server
        enabled: true
        tools: []
        access:
          acl_attribute_type: consumer
        config:
          url: https://mcp.example.com/mcp
          tools_cache_ttl_seconds: 60
          route:
            paths: [/mcp-upstream]
            methods: [GET, POST]
`

	rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGateways, 1)
	require.Empty(t, rs.AIGateways[0].MCPServers)
	require.Len(t, rs.AIGatewayMCPServers, 5)

	byRef := map[string]resources.AIGatewayMCPServerResource{}
	for _, server := range rs.AIGatewayMCPServers {
		require.Equal(t, "poc-default-ai-gateway", server.AIGateway)
		byRef[server.Ref] = server
	}
	require.Equal(t, "conversion-only", byRef["poc-mcp-conversion"].MCPServerType())
	require.Equal(t, "passthrough-listener", byRef["poc-mcp-server"].MCPServerType())
	require.Equal(t, "conversion-listener", byRef["poc-mcp-conversion-listener"].MCPServerType())
	conversionListenerPayload, err := byRef["poc-mcp-conversion-listener"].PayloadMap()
	require.NoError(t, err)
	conversionListenerAccess := conversionListenerPayload["access"].(map[string]any)
	require.Equal(
		t,
		[]any{tags.RefPlaceholderPrefix + "poc-key-auth#id"},
		conversionListenerAccess["auth_strategies"],
	)
	require.Equal(t, "listener", byRef["poc-mcp-listener"].MCPServerType())
	listenerPayload, err := byRef["poc-mcp-listener"].PayloadMap()
	require.NoError(t, err)
	require.Equal(t, []any{"poc-mcp-conversion", "poc-mcp-upstream"}, listenerPayload["sources"])
	require.Equal(t, "upstream-server", byRef["poc-mcp-upstream"].MCPServerType())
}

func TestLoaderRejectsEmptyConversionMCPServerTools(t *testing.T) {
	for _, serverType := range []string{"conversion-only", "conversion-listener"} {
		t.Run(serverType, func(t *testing.T) {
			input := fmt.Sprintf(`
ai_gateway_mcp_servers:
  - ref: tools
    ai_gateway: gateway
    type: %s
    name: tools
    display_name: Tools
    config:
      url: https://tools.example.com
    tools: []
`, serverType)

			_, err := New().LoadFile(writeLoaderTestFile(t, input))
			require.ErrorContains(t, err, "tools must contain at least one entry")
		})
	}
}

func TestLoaderRejectsAIGatewayMCPListenerWithoutSources(t *testing.T) {
	input := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    mcp_servers:
      - ref: support-listener
        type: listener
        name: support-listener
        display_name: Support Listener
        config:
          route:
            paths: [/support]
`

	_, err := New().LoadFromSources(
		[]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}},
		false,
	)
	require.Error(t, err)
	require.ErrorContains(t, err, "sources")
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
