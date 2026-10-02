package planner

import (
	"encoding/json"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestMCPSourceRenamePlan(t *testing.T) {
	for _, kind := range []string{"upstream-server", "conversion-only"} {
		t.Run(kind, func(t *testing.T) {
			source := mcpLifecycleObserved(t, "source-id", "source", false, false)
			payload, err := resources.AIGatewayMCPServerMutablePayloadMap(source)
			require.NoError(t, err)
			payload[FieldType] = kind
			if kind == "upstream-server" {
				payload[FieldConfig].(map[string]any)["tools_cache_ttl_seconds"] = 60
			}
			payload[FieldID] = "source-id"
			payload["created_at"] = "2026-01-01T00:00:00Z"
			payload["updated_at"] = "2026-01-01T00:00:00Z"
			data, err := json.Marshal(payload)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(data, &source))
			listener := mcpLifecycleObserved(t, "listener-id", "listener", true, false)
			next := mcpLifecycleDesired(t, source)
			if kind == "upstream-server" {
				next.AIGatewayMCPServerUpstreamServer.Name = "renamed"
			} else {
				next.AIGatewayMCPServerConversionOnly.Name = "renamed"
			}
			desiredListener := mcpLifecycleDesired(t, listener)
			desiredListener.AIGatewayMCPServerListener.Sources = []string{"renamed"}
			api := &mcpLifecycleAPI{
				testAIGatewayMCPServerAPI: testAIGatewayMCPServerAPI{servers: []kkComps.AIGatewayMCPServer{source, listener}},
				detail:                    &listener,
			}
			client := state.NewClient(state.ClientConfig{
				AIGatewayAPI:           &testAIGatewayAPI{gateways: []kkComps.AIGateway{testAIGateway()}},
				AIGatewayMCPServersAPI: api,
			})
			scope := resources.NewSyncScope()
			scope.AddRoot(resources.ResourceTypeAIGateway)
			scope.AddChild(resources.ResourceTypeAIGateway, "support-gateway", resources.ResourceTypeAIGatewayMCPServer)
			rs := &resources.ResourceSet{
				AIGateways:          []resources.AIGatewayResource{testAIGatewayResource()},
				AIGatewayMCPServers: []resources.AIGatewayMCPServerResource{desiredListener, next},
				SyncScope:           scope,
			}
			plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
			require.NoError(t, err)
			require.Len(t, plan.Changes, 3)
			byAction := make(map[ActionType]PlannedChange)
			for _, change := range plan.Changes {
				byAction[change.Action] = change
			}
			creation, update, deletion := byAction[ActionCreate], byAction[ActionUpdate], byAction[ActionDelete]
			require.Contains(t, update.DependsOn, creation.ID)
			require.Contains(t, deletion.DependsOn, update.ID)
			data, err = json.Marshal(plan)
			require.NoError(t, err)
			var saved Plan
			require.NoError(t, json.Unmarshal(data, &saved))
			require.Less(t, indexOf(saved.ExecutionOrder, creation.ID), indexOf(saved.ExecutionOrder, update.ID))
			require.Less(t, indexOf(saved.ExecutionOrder, update.ID), indexOf(saved.ExecutionOrder, deletion.ID))
		})
	}
}

func TestMCPSourceDeleteRequiresDetachment(t *testing.T) {
	for _, tt := range []struct {
		name      string
		action    ActionType
		fields    map[string]any
		wantError bool
	}{
		{name: "unplanned retained listener", wantError: true},
		{
			name: "planned retained listener", action: ActionUpdate,
			fields: map[string]any{FieldSources: []any{"source"}}, wantError: true,
		},
		{name: "omitted sources", action: ActionUpdate, fields: map[string]any{FieldDisplayName: "changed"}, wantError: true},
		{name: "null sources", action: ActionUpdate, fields: map[string]any{FieldSources: nil}, wantError: true},
		{name: "malformed sources", action: ActionUpdate, fields: map[string]any{FieldSources: []any{1}}, wantError: true},
		{name: "empty sources", action: ActionUpdate, fields: map[string]any{FieldSources: []any{}}},
		{name: "changed sources", action: ActionUpdate, fields: map[string]any{FieldSources: []any{"new"}}},
		{name: "deleted listener", action: ActionDelete},
		{
			name: "created listener retains source", action: ActionCreate,
			fields: map[string]any{FieldSources: []any{"source"}}, wantError: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			listener := mcpLifecycleObserved(t, "listener-id", "listener", true, false)
			api := &testAIGatewayMCPServerAPI{servers: []kkComps.AIGatewayMCPServer{listener}}
			p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayMCPServersAPI: api}), slog.Default())
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
			parent := &ParentInfo{Ref: "gateway", ID: "gateway-id"}
			plan.AddChange(PlannedChange{
				ID: "delete-source", Action: ActionDelete, ResourceType: ResourceTypeAIGatewayMCPServer,
				ResourceID: "source-id", Fields: map[string]any{FieldName: "source"}, Namespace: "ns", Parent: parent,
			})
			if tt.action != "" {
				plan.AddChange(PlannedChange{
					ID: "listener-change", Action: tt.action, ResourceType: ResourceTypeAIGatewayMCPServer,
					ResourceID: "listener-id", ResourceRef: "listener", Fields: tt.fields, Namespace: "ns", Parent: parent,
				})
			}
			err := p.resolveAIGatewayMCPSourceDeletes(t.Context(), "ns", "gateway", "gateway-id", plan)
			if tt.wantError {
				require.ErrorContains(t, err, "still references it in sources")
			} else {
				require.NoError(t, err)
				require.Equal(t, []string{"listener-change"}, plan.Changes[0].DependsOn)
			}
		})
	}
}
