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

func TestAIGatewayNodePlannerCreatesChildForExistingGateway(t *testing.T) {
	node := testAIGatewayNodeResource(t, "3.11.0", "support-node-1", "2024.06.01")
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayNodesAPI: &testAIGatewayNodeAPI{},
	})
	rs := testAIGatewayNodeResourceSet(node, nil)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayNode, change.ResourceType)
	require.Equal(t, "support-node", change.ResourceRef)
	require.Equal(t, testAIGatewayNodeID, change.ResourceID)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
	require.Equal(t, "support-gateway", change.Parent.Ref)
	require.Equal(t, testAIGatewayNodeID, change.Fields[FieldID])
	require.Equal(t, "3.11.0", change.Fields[FieldVersion])
	require.Equal(t, "support-node-1", change.Fields[FieldHostname])
	require.Equal(t, "data-plane", change.Fields[FieldType])
}

func TestAIGatewayNodePlannerUpdatesChangedNode(t *testing.T) {
	node := testAIGatewayNodeResource(t, "3.11.1", "support-node-1-updated", "2024.06.02")
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayNodesAPI: &testAIGatewayNodeAPI{
			nodes: []kkComps.AIGatewayDataPlaneNode{
				testAIGatewayNode("3.11.0", "support-node-1", "2024.06.01"),
			},
		},
	})
	rs := testAIGatewayNodeResourceSet(node, nil)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionUpdate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayNode, change.ResourceType)
	require.Equal(t, testAIGatewayNodeID, change.ResourceID)
	require.Contains(t, change.ChangedFields, FieldVersion)
	require.Contains(t, change.ChangedFields, FieldHostname)
	require.Contains(t, change.ChangedFields, "config_version")
}

func TestAIGatewayNodePlannerSyncDeletesScopedNodes(t *testing.T) {
	scope := resources.NewSyncScope()
	scope.AddRoot(resources.ResourceTypeAIGateway)
	scope.AddChild(resources.ResourceTypeAIGateway, "support-gateway", resources.ResourceTypeAIGatewayNode)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayNodesAPI: &testAIGatewayNodeAPI{
			nodes: []kkComps.AIGatewayDataPlaneNode{
				testAIGatewayNode("3.11.0", "support-node-1", "2024.06.01"),
			},
		},
	})
	rs := testAIGatewayNodeResourceSet(resources.AIGatewayNodeResource{}, scope)
	rs.AIGatewayNodes = nil

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionDelete, change.Action)
	require.Equal(t, ResourceTypeAIGatewayNode, change.ResourceType)
	require.Equal(t, testAIGatewayNodeID, change.ResourceID)
	require.Equal(t, testAIGatewayNodeID, change.ResourceRef)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
}

const testAIGatewayNodeID = "3f2b1c9a-8d7e-4f6a-9b0c-1d2e3f4a5b6c"

func testAIGatewayNodeResource(
	t *testing.T,
	version string,
	hostname string,
	configVersion string,
) resources.AIGatewayNodeResource {
	t.Helper()
	payload := `{
		"ref": "support-node",
		"ai_gateway": "support-gateway",
		"id": "` + testAIGatewayNodeID + `",
		"version": "` + version + `",
		"hostname": "` + hostname + `",
		"type": "data-plane",
		"config_version": "` + configVersion + `"
	}`
	var node resources.AIGatewayNodeResource
	require.NoError(t, json.Unmarshal([]byte(payload), &node))
	return node
}

func testAIGatewayNodeResourceSet(
	node resources.AIGatewayNodeResource,
	scope *resources.SyncScope,
) *resources.ResourceSet {
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
	if node.Ref != "" {
		rs.AIGatewayNodes = []resources.AIGatewayNodeResource{node}
	}
	return rs
}

func testAIGatewayNode(version string, hostname string, configVersion string) kkComps.AIGatewayDataPlaneNode {
	return kkComps.AIGatewayDataPlaneNode{
		ID:            testAIGatewayNodeID,
		Version:       version,
		Hostname:      hostname,
		Type:          "data-plane",
		ConfigVersion: &configVersion,
	}
}

type testAIGatewayNodeAPI struct {
	nodes []kkComps.AIGatewayDataPlaneNode
}

func (t *testAIGatewayNodeAPI) ListAiGatewayNodes(
	context.Context,
	kkOps.ListAiGatewayNodesRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayNodesResponse, error) {
	return &kkOps.ListAiGatewayNodesResponse{
		ListAIGatewayDataPlaneNodesResponse: &kkComps.ListAIGatewayDataPlaneNodesResponse{
			Data: t.nodes,
		},
	}, nil
}

func (t *testAIGatewayNodeAPI) GetAiGatewayNode(
	_ context.Context,
	_ string,
	dataPlaneNodeID string,
	_ ...kkOps.Option,
) (*kkOps.GetAiGatewayNodeResponse, error) {
	for _, node := range t.nodes {
		if node.ID == dataPlaneNodeID {
			return &kkOps.GetAiGatewayNodeResponse{AIGatewayDataPlaneNode: &node}, nil
		}
	}
	return &kkOps.GetAiGatewayNodeResponse{}, nil
}

func (t *testAIGatewayNodeAPI) UpsertAiGatewayNode(
	context.Context,
	string,
	string,
	map[string]any,
	...kkOps.Option,
) (*kkComps.AIGatewayDataPlaneNode, error) {
	return nil, nil
}

func (t *testAIGatewayNodeAPI) DeleteAiGatewayNode(
	context.Context,
	string,
	string,
	...kkOps.Option,
) error {
	return nil
}
