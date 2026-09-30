package planner

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayMCPSameRoute(t *testing.T) {
	payload := func(kind string, route string) map[string]any {
		var fields map[string]any
		require.NoError(t, json.Unmarshal([]byte(`{"type":"`+kind+`","config":{"route":`+route+`}}`), &fields))
		return fields
	}
	for _, tt := range []struct {
		name, oldType, newType, oldRoute, newRoute string
		want                                       bool
	}{
		{
			"passthrough rename", "passthrough-listener", "passthrough-listener",
			`{"paths":["/mcp"]}`,
			`{"paths":["/mcp"]}`, true,
		},
		{"variant change", "listener", "conversion-listener", `{"paths":["/mcp"]}`, `{"paths":["/mcp"]}`, true},
		{"source has no route", "conversion-only", "listener", `{"paths":["/mcp"]}`, `{"paths":["/mcp"]}`, false},
		{"upstream has no route", "listener", "upstream-server", `{"paths":["/mcp"]}`, `{"paths":["/mcp"]}`, false},
		{"no route", "listener", "listener", `{}`, `{}`, false},
		{"changed path", "listener", "listener", `{"paths":["/old"]}`, `{"paths":["/new"]}`, false},
		{"partial overlap", "listener", "listener", `{"paths":["/mcp","/old"]}`, `{"paths":["/mcp"]}`, false},
		{"changed host", "listener", "listener", `{"hosts":["a"]}`, `{"hosts":["b"]}`, false},
		{
			"changed methods", "listener", "listener",
			`{"paths":["/mcp"],"methods":["GET"]}`,
			`{"paths":["/mcp"],"methods":["POST"]}`, false,
		},
		{"changed headers", "listener", "listener", `{"headers":{"x-v":["a"]}}`, `{"headers":{"x-v":["b"]}}`, false},
		{
			"changed protocol", "listener", "listener",
			`{"paths":["/mcp"],"protocols":["http"]}`,
			`{"paths":["/mcp"],"protocols":["https"]}`, false,
		},
		{
			"regex priority", "listener", "listener",
			`{"paths":["~/mcp"],"regex_priority":1}`,
			`{"paths":["~/mcp"],"regex_priority":2}`, false,
		},
		{
			"API defaults", "listener", "listener",
			`{"paths":["/mcp"],"protocols":["http","https"],"strip_path":true,"regex_priority":0}`,
			`{"paths":["/mcp"]}`, true,
		},
		{
			"unordered matchers", "listener", "listener",
			`{"paths":["/a","/b"],"hosts":["a","b"],"methods":["GET","POST"],"headers":{"x-v":["a","b"]}}`,
			`{"paths":["/b","/a"],"hosts":["b","a"],"methods":["POST","GET"],"headers":{"x-v":["b","a"]}}`, true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			old, next := payload(tt.oldType, tt.oldRoute), payload(tt.newType, tt.newRoute)
			require.Equal(t, tt.want, aiGatewayMCPSameRoute(old, next))
			if tt.want {
				next[FieldEnabled] = false
				require.False(t, aiGatewayMCPSameRoute(old, next))
				delete(next, FieldEnabled)
				old[FieldEnabled] = false
				require.False(t, aiGatewayMCPSameRoute(old, next))
			}
		})
	}
}

func TestMCPReplacementLifecyclePersistsOrdering(t *testing.T) {
	old := mcpLifecycleObserved(t, "old-id", "old", true, false)
	next := mcpLifecycleDesired(t, mcpLifecycleObserved(t, "new-id", "new", true, false))
	api := &mcpLifecycleAPI{testAIGatewayMCPServerAPI: testAIGatewayMCPServerAPI{
		servers: []kkComps.AIGatewayMCPServer{old},
	}}
	plan, err := runMCPLifecycle(t, api, []resources.AIGatewayMCPServerResource{next})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 2)
	create, deletion := plan.Changes[0], plan.Changes[1]
	require.Contains(t, deletion.DependsOn, create.ID)
	// Add prerequisite/deletion edges in an order that formerly put deletion
	// at an earlier topological level than the replacement.
	plan.Changes[0].DependsOn = []string{"create-auth", "create-policy"}
	for _, kind := range []string{ResourceTypeAIGatewayAuthStrategy, ResourceTypeAIGatewayPolicy} {
		name := "auth"
		if kind == ResourceTypeAIGatewayPolicy {
			name = "policy"
		}
		plan.AddChange(PlannedChange{
			ID: "delete-" + name, ResourceType: kind, Action: ActionDelete,
			Namespace: "test-namespace", Parent: &ParentInfo{Ref: "support-gateway", ID: "gateway-id"},
			DependsOn: []string{deletion.ID},
		})
		plan.AddChange(PlannedChange{
			ID: "create-" + name, ResourceType: kind, Action: ActionCreate,
			Namespace: "test-namespace", Parent: &ParentInfo{Ref: "support-gateway", ID: "gateway-id"},
		})
	}
	resolved, err := NewDependencyResolver().ResolveDependenciesWithGroups(plan.Changes)
	require.NoError(t, err, "replacement edges must precede sibling serialization to avoid cycles")
	for i := range plan.Changes {
		plan.Changes[i].DependsOn = resolved.FullDepsMap[plan.Changes[i].ID]
	}
	plan.ExecutionOrder = resolved.ExecutionOrder
	plan.SetExecutionGroups(resolved.ExecutionGroups)
	data, err := json.Marshal(plan)
	require.NoError(t, err)
	var saved Plan
	require.NoError(t, json.Unmarshal(data, &saved))
	require.Contains(t, saved.Changes[1].DependsOn, create.ID)
	for _, prerequisite := range []string{"create-auth", "create-policy"} {
		require.Less(t, indexOf(saved.ExecutionOrder, prerequisite), indexOf(saved.ExecutionOrder, create.ID))
	}
	require.Less(t, indexOf(saved.ExecutionOrder, create.ID), indexOf(saved.ExecutionOrder, deletion.ID))
	for _, dependency := range []string{"delete-auth", "delete-policy"} {
		require.Less(t, indexOf(saved.ExecutionOrder, deletion.ID), indexOf(saved.ExecutionOrder, dependency))
	}
}

func TestMCPReplacementScopeAndMultipleCreates(t *testing.T) {
	old := mcpLifecycleObserved(t, "old-id", "old", true, false)
	payload, err := resources.AIGatewayMCPServerMutablePayloadMap(old)
	require.NoError(t, err)
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	plan.AddChange(PlannedChange{
		ID: "delete-old", ResourceID: "old-id", ResourceType: ResourceTypeAIGatewayMCPServer,
		Action: ActionDelete, Namespace: "ns", Parent: &ParentInfo{Ref: "gateway", ID: "gateway-id"},
	})
	for _, tt := range []struct{ id, namespace, parentRef, parentID string }{
		{"same-1", "ns", "gateway", "gateway-id"},
		{"same-2", "ns", "gateway", "gateway-id"},
		{"other-gateway", "ns", "other", "other-id"},
		{"other-namespace", "other", "gateway", "gateway-id"},
		{"different-ID", "ns", "gateway", "other-id"},
	} {
		plan.AddChange(PlannedChange{
			ID: tt.id, ResourceType: ResourceTypeAIGatewayMCPServer,
			Action: ActionCreate, Fields: payload, Namespace: tt.namespace,
			Parent: &ParentInfo{Ref: tt.parentRef, ID: tt.parentID},
		})
	}
	current := []state.AIGatewayMCPServer{{AIGatewayMCPServer: old}}
	require.NoError(t, addAIGatewayMCPReplacementDependencies("ns", "gateway", "gateway-id", current, plan))
	require.Equal(t, []string{"same-1", "same-2"}, plan.Changes[0].DependsOn)
	require.NoError(t, addAIGatewayMCPReplacementDependencies("ns", "gateway", "gateway-id", current, plan))
	require.Equal(t, []string{"same-1", "same-2"}, plan.Changes[0].DependsOn, "edges must be deduplicated")
	_, err = NewDependencyResolver().ResolveDependenciesWithGroups(plan.Changes)
	require.NoError(t, err)
}
