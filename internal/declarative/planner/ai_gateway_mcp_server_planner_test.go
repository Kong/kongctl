package planner

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayMCPServerPlannerCreatesChildForExistingGateway(t *testing.T) {
	server := testAIGatewayMCPServerResource(t)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway("gateway-id", "Support Gateway")},
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

func TestAIGatewayMCPServerPlannerSyncDeletesScopedServers(t *testing.T) {
	scope := resources.NewSyncScope()
	scope.AddRoot(resources.ResourceTypeAIGateway)
	scope.AddChild(resources.ResourceTypeAIGateway, "support-gateway", resources.ResourceTypeAIGatewayMCPServer)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway("gateway-id", "Support Gateway")},
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

func testAIGatewayMCPServerResource(t *testing.T) resources.AIGatewayMCPServerResource {
	t.Helper()
	payload := `{
		"ref": "support-tools",
		"ai_gateway": "support-gateway",
		"type": "conversion-only",
		"name": "support-tools",
		"display_name": "Support Tools",
		"enabled": true,
		"config": {"url": "https://support-tools.example.com"},
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
