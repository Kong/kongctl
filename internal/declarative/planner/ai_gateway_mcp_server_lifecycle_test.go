package planner

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestMCPServerLifecycleMatchesBeforeDeletionOrdering(t *testing.T) {
	source := mcpLifecycleObserved(t, "source-id", "duplicate", false, false)
	listener := mcpLifecycleObserved(t, "listener-id", "duplicate", true, false)
	detail := mcpLifecycleObserved(t, "detail-id", "duplicate", true, false)
	api := &mcpLifecycleAPI{
		testAIGatewayMCPServerAPI: testAIGatewayMCPServerAPI{servers: []kkComps.AIGatewayMCPServer{source, listener}},
		detail:                    &detail,
	}
	desired := mcpLifecycleDesired(t, listener)
	plan, err := runMCPLifecycle(t, api, []resources.AIGatewayMCPServerResource{desired})
	require.NoError(t, err)
	require.Equal(t, [][2]string{{"gateway-id", "listener-id"}}, api.reads)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionUpdate, change.Action)
	require.Equal(t, "listener-id", change.ResourceID, "updates must use the listed ID, not the detail ID")
	require.Equal(t, desired.Ref, change.ResourceRef)
	require.Equal(t, "Updated", change.Fields[FieldDisplayName])
	require.Equal(t, "test-namespace", change.Namespace)
	require.Equal(t, &ParentInfo{Ref: "support-gateway", ID: "gateway-id"}, change.Parent)
}

func TestMCPServerLifecycleStopsAtProtectedDeletion(t *testing.T) {
	for _, protectListener := range []bool{true, false} {
		name := "protected source after listener"
		if protectListener {
			name = "protected listener before source"
		}
		t.Run(name, func(t *testing.T) {
			api := &mcpLifecycleAPI{testAIGatewayMCPServerAPI: testAIGatewayMCPServerAPI{
				servers: []kkComps.AIGatewayMCPServer{
					mcpLifecycleObserved(t, "source-id", "source", false, !protectListener),
					mcpLifecycleObserved(t, "listener-id", "listener", true, protectListener),
				},
			}}
			plan, err := runMCPLifecycle(t, api, nil)
			require.ErrorContains(t, err, "protected")
			require.Empty(t, api.reads)
			if protectListener {
				require.ErrorContains(t, err, "listener")
				require.Empty(t, plan.Changes)
			} else {
				require.ErrorContains(t, err, "source")
				require.Len(t, plan.Changes, 1)
				require.Equal(t, ActionDelete, plan.Changes[0].Action)
				require.Equal(t, "listener-id", plan.Changes[0].ResourceID)
			}
		})
	}
}

func TestMCPServerLifecycleMissingDetailDependencies(t *testing.T) {
	for _, failRead := range []bool{false, true} {
		name := "missing detail recreates after source"
		if failRead {
			name = "failed detail stops before pruning"
		}
		t.Run(name, func(t *testing.T) {
			source := mcpLifecycleObserved(t, "source-id", "source", false, false)
			listener := mcpLifecycleObserved(t, "listener-id", "listener", true, false)
			api := &mcpLifecycleAPI{testAIGatewayMCPServerAPI: testAIGatewayMCPServerAPI{
				servers: []kkComps.AIGatewayMCPServer{
					listener, mcpLifecycleObserved(t, "obsolete-id", "obsolete", false, false),
				},
			}}
			if failRead {
				api.err = errors.New("detail unavailable")
			}
			// Input is intentionally reversed; the source must be planned first.
			desired := []resources.AIGatewayMCPServerResource{
				mcpLifecycleDesired(t, listener), mcpLifecycleDesired(t, source),
			}
			plan, err := runMCPLifecycle(t, api, desired)
			require.Equal(t, [][2]string{{"gateway-id", "listener-id"}}, api.reads)
			if failRead {
				require.ErrorContains(t, err, "failed to get AI Gateway MCP Server listener-id")
				require.ErrorContains(t, err, "detail unavailable")
				require.Len(t, plan.Changes, 1)
			} else {
				require.NoError(t, err)
				require.Len(t, plan.Changes, 3)
				require.Equal(t, ActionCreate, plan.Changes[1].Action)
				require.Equal(t, desired[0].Ref, plan.Changes[1].ResourceRef)
				require.Empty(t, plan.Changes[1].ResourceID)
				require.Equal(t, []string{plan.Changes[0].ID}, plan.Changes[1].DependsOn)
				require.Equal(t, ActionDelete, plan.Changes[2].Action)
				require.Equal(t, "obsolete-id", plan.Changes[2].ResourceID)
			}
			require.Equal(t, ActionCreate, plan.Changes[0].Action)
			require.Equal(t, desired[1].Ref, plan.Changes[0].ResourceRef)
		})
	}
}

func TestNameMatchedChildrenPruneOrderPhase(t *testing.T) {
	for _, tt := range []struct {
		name     string
		mode     PlanMode
		custom   bool
		failRead bool
		want     []string
	}{
		{name: "default retains observed order", mode: PlanModeSync, want: []string{"source", "listener"}},
		{
			name: "custom order is used for sync", mode: PlanModeSync, custom: true,
			want: []string{"order", "listener", "source"},
		},
		{name: "apply skips ordering", mode: PlanModeApply, custom: true},
		{name: "observation failure skips ordering", mode: PlanModeSync, custom: true, failRead: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var events, desired []string
			readErr := errors.New("observation failed")
			if tt.failRead {
				desired = []string{"source"}
			}
			ops := nameMatchedChildOperations[string, string]{
				desiredName: func(s string) string { return s },
				currentName: func(s string) string { return s },
				fetch:       func(string, string) (*string, error) { return nil, readErr },
				protected:   func(string) bool { return false },
				remove:      func(s string) { events = append(events, s) },
			}
			if tt.custom {
				ops.pruneOrder = func(current []string) []string {
					events = append(events, "order")
					result := slices.Clone(current)
					slices.Reverse(result)
					return result
				}
			}
			p := NewPlanner(nil, slog.Default())
			err := reconcileNameMatchedChildren(p, ResourceTypeAIGatewayMCPServer,
				desired, []string{"source", "listener"}, ops, NewPlan(CurrentPlanVersion, "test", tt.mode))
			if tt.failRead {
				require.ErrorIs(t, err, readErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, events)
		})
	}
}

type mcpLifecycleAPI struct {
	testAIGatewayMCPServerAPI
	detail *kkComps.AIGatewayMCPServer
	err    error
	reads  [][2]string
}

func (a *mcpLifecycleAPI) GetAiGatewayMcpServer(
	_ context.Context, gatewayID, serverID string, _ ...kkOps.Option,
) (*kkOps.GetAiGatewayMcpServerResponse, error) {
	a.reads = append(a.reads, [2]string{gatewayID, serverID})
	return &kkOps.GetAiGatewayMcpServerResponse{AIGatewayMCPServer: a.detail}, a.err
}

func runMCPLifecycle(
	t *testing.T,
	api *mcpLifecycleAPI,
	desired []resources.AIGatewayMCPServerResource,
) (*Plan, error) {
	t.Helper()
	p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayMCPServersAPI: api}), slog.Default())
	p.resources = &resources.ResourceSet{AIGatewayMCPServers: desired}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	err := p.planAIGatewayMCPServerChanges(t.Context(), "test-namespace",
		"support-gateway", "Gateway", "gateway-id", "", nil, desired, plan)
	return plan, err
}

func mcpLifecycleObserved(t *testing.T, id, name string, listener, protected bool) kkComps.AIGatewayMCPServer {
	t.Helper()
	payload := map[string]any{
		FieldID: id, FieldName: name, FieldDisplayName: name, FieldEnabled: true,
		FieldType: "conversion-only", FieldConfig: map[string]any{"url": "https://example.test"},
		FieldTools: []any{}, FieldPolicies: []any{},
		"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
	}
	if listener {
		payload[FieldType] = "listener"
		payload[FieldSources] = []string{"source"}
		payload[FieldConfig] = map[string]any{"route": map[string]any{"paths": []string{"/mcp"}}}
	}
	if protected {
		payload[FieldLabels] = map[string]string{labels.ProtectedKey: labels.TrueValue}
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var server kkComps.AIGatewayMCPServer
	require.NoError(t, json.Unmarshal(data, &server))
	return server
}

func mcpLifecycleDesired(t *testing.T, observed kkComps.AIGatewayMCPServer) resources.AIGatewayMCPServerResource {
	t.Helper()
	payload, err := resources.AIGatewayMCPServerMutablePayloadMap(observed)
	require.NoError(t, err)
	payload["ref"] = "local-" + resources.AIGatewayMCPServerName(observed)
	payload["ai_gateway"] = "support-gateway"
	payload[FieldDisplayName] = "Updated"
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var desired resources.AIGatewayMCPServerResource
	require.NoError(t, json.Unmarshal(data, &desired))
	return desired
}
