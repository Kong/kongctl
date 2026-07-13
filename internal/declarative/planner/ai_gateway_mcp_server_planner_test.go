package planner

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayMCPServerPlannerCreatesChildForExistingGateway(t *testing.T) {
	server := testAIGatewayMCPServerResource(t)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayMCPServersAPI: &testAIGatewayMCPServerAPI{},
	})
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{
				Ref:     "support-gateway",
				Kongctl: &resources.KongctlMeta{Namespace: new("default")},
			},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "support-gateway",
				DisplayName: "Support Gateway",
			},
		}},
		AIGatewayMCPServers: []resources.AIGatewayMCPServerResource{server},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayMCPServer, change.ResourceType)
	require.Equal(t, "support-tools", change.ResourceRef)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
	require.Equal(t, "support-gateway", change.Parent.Ref)
	require.Equal(t, "conversion-only", change.Fields[FieldType])
}

func TestAIGatewayMCPServerPlannerCreatesIssue1499NestedServers(t *testing.T) {
	input := `
ai_gateways:
  - ref: poc-default-ai-gateway
    display_name: POC Default AI Gateway
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
        tools: []
        access:
          acl_attribute_type: consumer
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
        tools: []
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
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(input), 0o600))

	rs, err := loader.New().LoadFromSources([]loader.Source{{
		Path: path,
		Type: loader.SourceTypeFile,
	}}, false)
	require.NoError(t, err)

	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{{
				ID:          "gateway-id",
				Name:        "poc-default-ai-gateway",
				DisplayName: "POC Default AI Gateway",
				Labels: map[string]string{
					labels.NamespaceKey: "default",
				},
			}},
		},
		AIGatewayMCPServersAPI: &testAIGatewayMCPServerAPI{},
	})

	for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
		plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: mode})
		require.NoError(t, err)
		require.Len(t, plan.Changes, 5)

		byRef := map[string]PlannedChange{}
		for _, change := range plan.Changes {
			require.Equal(t, ActionCreate, change.Action)
			require.Equal(t, ResourceTypeAIGatewayMCPServer, change.ResourceType)
			byRef[change.ResourceRef] = change
		}

		require.Contains(t, byRef, "poc-mcp-conversion")
		require.Equal(t, "conversion-only", byRef["poc-mcp-conversion"].Fields[FieldType])
		require.Contains(t, byRef, "poc-mcp-server")
		require.Equal(t, "passthrough-listener", byRef["poc-mcp-server"].Fields[FieldType])
		require.Contains(t, byRef, "poc-mcp-conversion-listener")
		require.Equal(t, "conversion-listener", byRef["poc-mcp-conversion-listener"].Fields[FieldType])
		require.Contains(t, byRef, "poc-mcp-listener")
		require.Equal(t, "listener", byRef["poc-mcp-listener"].Fields[FieldType])
		require.Contains(t, byRef, "poc-mcp-upstream")
		require.Equal(t, "upstream-server", byRef["poc-mcp-upstream"].Fields[FieldType])
	}
}

func TestAIGatewayMCPServerPlannerSyncDeletesScopedServers(t *testing.T) {
	scope := resources.NewSyncScope()
	scope.AddRoot(resources.ResourceTypeAIGateway)
	scope.AddChild(resources.ResourceTypeAIGateway, "support-gateway", resources.ResourceTypeAIGatewayMCPServer)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayMCPServersAPI: &testAIGatewayMCPServerAPI{
			servers: []kkComps.AIGatewayMCPServer{testAIGatewayMCPServer(t, "server-id", "support-tools")},
		},
	})
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{
				Ref:     "support-gateway",
				Kongctl: &resources.KongctlMeta{Namespace: new("default")},
			},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "support-gateway",
				DisplayName: "Support Gateway",
			},
		}},
		SyncScope: scope,
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionDelete, change.Action)
	require.Equal(t, ResourceTypeAIGatewayMCPServer, change.ResourceType)
	require.Equal(t, "server-id", change.ResourceID)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
}

func TestAIGatewayMCPServerPlannerIgnoresAPIDefaults(t *testing.T) {
	server := testAIGatewayMCPServerResource(t)
	var current kkComps.AIGatewayMCPServer
	require.NoError(t, json.Unmarshal([]byte(`{
		"id": "server-id",
		"type": "conversion-only",
		"name": "support-tools",
		"display_name": "Support Tools",
		"enabled": true,
		"config": {
			"url": "https://support-tools.example.com",
			"route": {
				"paths": ["/support-tools"],
				"https_redirect_status_code": 426,
				"preserve_host": false,
				"protocols": ["http", "https"],
				"regex_priority": 0,
				"request_buffering": true,
				"response_buffering": true,
				"strip_path": true
			},
			"max_request_body_size": 8388608,
			"logging": {
				"payloads": false,
				"statistics": true,
				"audits": false
			}
		},
		"tools": [{"name": "lookup-customer", "description": "Look up a customer profile", "method": "GET"}],
		"policies": [],
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-01T00:00:00Z"
	}`), &current))

	needsUpdate, fields, changed, err := (&Planner{}).shouldUpdateAIGatewayMCPServer(
		state.AIGatewayMCPServer{AIGatewayMCPServer: current},
		server,
	)

	require.NoError(t, err)
	require.False(t, needsUpdate)
	require.Nil(t, fields)
	require.Nil(t, changed)
}

func TestAIGatewayMCPServerPlannerIgnoresDefaultAccessWhenServerOmitsIt(t *testing.T) {
	server := testAIGatewayMCPServerResourceWithAccess(t)
	payload, err := server.MutablePayloadMap()
	require.NoError(t, err)
	require.Contains(t, payload, FieldAccess)

	var current kkComps.AIGatewayMCPServer
	require.NoError(t, json.Unmarshal([]byte(`{
		"id": "server-id",
		"type": "conversion-only",
		"name": "support-tools",
		"display_name": "Support Tools",
		"enabled": true,
		"config": {
			"url": "https://support-tools.example.com",
			"route": {
				"paths": ["/support-tools"],
				"https_redirect_status_code": 426,
				"preserve_host": false,
				"protocols": ["http", "https"],
				"regex_priority": 0,
				"request_buffering": true,
				"response_buffering": true,
				"strip_path": true
			},
			"max_request_body_size": 8388608
		},
		"tools": [{"name": "lookup-customer", "description": "Look up a customer profile", "method": "GET"}],
		"policies": [],
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-01T00:00:00Z"
	}`), &current))
	needsUpdate, fields, changed, err := (&Planner{}).shouldUpdateAIGatewayMCPServer(
		state.AIGatewayMCPServer{AIGatewayMCPServer: current},
		server,
	)

	require.NoError(t, err)
	require.Falsef(t, needsUpdate, "changed fields: %#v", changed)
	require.Nil(t, fields)
	require.Nil(t, changed)
}

func testAIGatewayMCPServerResource(t *testing.T) resources.AIGatewayMCPServerResource {
	t.Helper()
	payload := `{
		"ref": "support-tools",
		"ai_gateway": "support-gateway",
		"type": "conversion-only",
		"name": "support-tools",
		"display_name": "Support Tools",
		"enabled": true,
		"config": {
			"url": "https://support-tools.example.com",
			"route": {"paths": ["/support-tools"]}
		},
		"tools": [{"name": "lookup-customer", "description": "Look up a customer profile", "method": "GET"}],
		"policies": []
	}`
	var server resources.AIGatewayMCPServerResource
	require.NoError(t, json.Unmarshal([]byte(payload), &server))
	return server
}

func testAIGatewayMCPServerResourceWithAccess(t *testing.T) resources.AIGatewayMCPServerResource {
	t.Helper()
	payload := `{
		"ref": "support-tools",
		"ai_gateway": "support-gateway",
		"type": "conversion-only",
		"name": "support-tools",
		"display_name": "Support Tools",
		"enabled": true,
		"access": {
			"acl_attribute_type": "consumer",
			"acls": [],
			"default_tool_acls": []
		},
		"config": {
			"url": "https://support-tools.example.com",
			"route": {"paths": ["/support-tools"]}
		},
		"tools": [{"name": "lookup-customer", "description": "Look up a customer profile", "method": "GET"}],
		"policies": []
	}`
	var server resources.AIGatewayMCPServerResource
	require.NoError(t, json.Unmarshal([]byte(payload), &server))
	return server
}

func testAIGatewayMCPServer(t *testing.T, id string, name string) kkComps.AIGatewayMCPServer {
	t.Helper()
	payload := `{
		"id": "` + id + `",
		"type": "conversion-only",
		"name": "` + name + `",
		"display_name": "` + name + `",
		"enabled": true,
		"config": {"url": "https://support-tools.example.com"},
		"tools": [{"name": "lookup-customer", "description": "Look up a customer profile", "method": "GET"}],
		"policies": [],
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-01T00:00:00Z"
	}`
	var server kkComps.AIGatewayMCPServer
	require.NoError(t, json.Unmarshal([]byte(payload), &server))
	return server
}

type testAIGatewayMCPServerAPI struct {
	servers []kkComps.AIGatewayMCPServer
}

func (t *testAIGatewayMCPServerAPI) ListAiGatewayMcpServers(
	context.Context,
	kkOps.ListAiGatewayMcpServersRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayMcpServersResponse, error) {
	return &kkOps.ListAiGatewayMcpServersResponse{
		ListAIGatewayMCPServersResponse: &kkComps.ListAIGatewayMCPServersResponse{
			Data: t.servers,
		},
	}, nil
}

func (t *testAIGatewayMCPServerAPI) CreateAiGatewayMcpServer(
	context.Context,
	string,
	kkComps.CreateAIGatewayMCPServerRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayMcpServerResponse, error) {
	return nil, nil
}

func (t *testAIGatewayMCPServerAPI) GetAiGatewayMcpServer(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetAiGatewayMcpServerResponse, error) {
	return nil, nil
}

func (t *testAIGatewayMCPServerAPI) UpdateAiGatewayMcpServer(
	context.Context,
	kkOps.UpdateAiGatewayMcpServerRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayMcpServerResponse, error) {
	return nil, nil
}

func (t *testAIGatewayMCPServerAPI) DeleteAiGatewayMcpServer(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayMcpServerResponse, error) {
	return nil, nil
}
