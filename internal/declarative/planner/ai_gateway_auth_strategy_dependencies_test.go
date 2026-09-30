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

func TestAIGatewayAuthStrategyDeleteOrdering(t *testing.T) {
	for _, kind := range []string{
		ResourceTypeAIGatewayAgent, ResourceTypeAIGatewayModel, ResourceTypeAIGatewayMCPServer,
	} {
		for _, tt := range []struct {
			name         string
			action       ActionType
			fields       map[string]any
			currentByID  bool
			otherGateway bool
			otherNS      bool
			reuseRef     bool
			wantError    string
		}{
			{name: "delete before authStrategy", action: ActionDelete},
			{name: "remote authStrategy ID", action: ActionDelete, currentByID: true},
			{name: "detach update", action: ActionUpdate, fields: authStrategyAccessFields([]any{})},
			{
				name: "replace authStrategy", action: ActionUpdate,
				fields: authStrategyAccessFields([]any{"__REF__:replacement#name"}),
			},
			{name: "retained resource", wantError: "still references it"},
			{
				name: "rename keeping the original ref", action: ActionUpdate, reuseRef: true,
				fields: authStrategyAccessFields([]string{"__REF__:old-authStrategy#name"}),
			},
			{
				name: "literal old name remains attached despite reused ref", action: ActionUpdate, reuseRef: true,
				fields: authStrategyAccessFields([]string{"old-authStrategy"}), wantError: "planned",
			},
			{
				name: "unrelated update", action: ActionUpdate,
				fields: map[string]any{FieldDisplayName: "New display name"}, wantError: "still references it",
			},
			{
				name: "null does not prove detachment", action: ActionUpdate,
				fields: authStrategyAccessFields(nil), wantError: "still references it",
			},
			{
				name: "malformed list does not prove detachment", action: ActionUpdate,
				fields: authStrategyAccessFields([]any{"replacement", 42}), wantError: "still references it",
			},
			{
				name: "update retains reference", action: ActionUpdate,
				fields: authStrategyAccessFields([]string{"old-authStrategy"}), wantError: "planned",
			},
			{
				name: "update retains ID reference", action: ActionUpdate,
				fields: authStrategyAccessFields([]any{"authStrategy-id"}), wantError: "planned",
			},
			{
				name: "create references deleted authStrategy", action: ActionCreate,
				fields: authStrategyAccessFields([]any{"__REF__:old-authStrategy#name"}), wantError: "planned",
			},
			{
				name: "other gateway delete cannot detach", action: ActionDelete,
				otherGateway: true, wantError: "still references it",
			},
			{
				name: "other namespace delete cannot detach", action: ActionDelete,
				otherNS: true, wantError: "still references it",
			},
		} {
			t.Run(kind+"/"+tt.name, func(t *testing.T) {
				authStrategyReference := "old-authStrategy"
				if tt.currentByID {
					authStrategyReference = "authStrategy-id"
				}
				cfg := authStrategyOrderClientConfig(t, kind, authStrategyReference)
				p := NewPlanner(state.NewClient(cfg), slog.Default())
				plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
				plan.Changes = []PlannedChange{
					{
						ID: "create-authStrategy", ResourceType: ResourceTypeAIGatewayAuthStrategy, Action: ActionCreate,
						ResourceRef: "replacement", Namespace: "default", Parent: &ParentInfo{Ref: "support-gateway"},
						Fields: map[string]any{FieldName: "new-strategy"},
					},
					{
						ID: "delete-authStrategy", ResourceType: ResourceTypeAIGatewayAuthStrategy, Action: ActionDelete,
						ResourceID: "authStrategy-id", Fields: map[string]any{FieldName: "old-authStrategy"},
						Namespace: "default", Parent: &ParentInfo{Ref: "support-gateway"},
					},
				}
				if tt.reuseRef {
					plan.Changes[0].ResourceRef = "old-authStrategy"
				}
				if tt.action != "" {
					change := PlannedChange{
						ID: "user-change", ResourceType: kind, Action: tt.action, ResourceID: "user-id",
						ResourceRef: "old-user", Fields: tt.fields, Namespace: "default",
						Parent: &ParentInfo{Ref: "support-gateway"},
					}
					if tt.action == ActionUpdate {
						change.DependsOn = []string{"create-authStrategy"}
					}
					if tt.otherGateway {
						change.Parent.Ref = "other-gateway"
					}
					if tt.otherNS {
						change.Namespace = "other"
					}
					plan.AddChange(change)
				}
				err := p.resolveAIGatewayAuthStrategyDeletes(
					t.Context(),
					"default",
					"support-gateway",
					"gateway-id",
					plan,
				)
				if tt.wantError != "" {
					require.ErrorContains(t, err, tt.wantError)
					return
				}
				require.NoError(t, err)
				require.Contains(t, plan.Changes[1].DependsOn, "user-change")
				result, err := NewDependencyResolver().ResolveDependenciesWithGroups(plan.Changes)
				require.NoError(t, err)
				require.Less(
					t,
					indexOf(result.ExecutionOrder, "user-change"),
					indexOf(result.ExecutionOrder, "delete-authStrategy"),
				)
				if tt.action == ActionUpdate {
					require.Less(
						t,
						indexOf(result.ExecutionOrder, "create-authStrategy"),
						indexOf(result.ExecutionOrder, "user-change"),
					)
				}
				for i := range plan.Changes {
					plan.Changes[i].DependsOn = result.FullDepsMap[plan.Changes[i].ID]
				}
				again, err := NewDependencyResolver().ResolveDependenciesWithGroups(plan.Changes)
				require.NoError(t, err)
				require.Equal(t, result, again)
			})
		}
	}
}

func TestAIGatewayAuthStrategyDeleteInspectsOutOfScopeResources(t *testing.T) {
	for _, kind := range []string{
		ResourceTypeAIGatewayAgent, ResourceTypeAIGatewayModel, ResourceTypeAIGatewayMCPServer,
	} {
		t.Run(kind, func(t *testing.T) {
			cfg := authStrategyOrderClientConfig(t, kind, "old-authStrategy")
			scope := resources.NewSyncScope()
			scope.AddRoot(resources.ResourceTypeAIGateway)
			scope.AddChild(
				resources.ResourceTypeAIGateway,
				"support-gateway",
				resources.ResourceTypeAIGatewayAuthStrategy,
			)
			rs := &resources.ResourceSet{
				AIGateways: []resources.AIGatewayResource{testAIGatewayResource()}, SyncScope: scope,
			}
			_, err := NewPlanner(
				state.NewClient(cfg),
				slog.Default(),
			).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
			require.ErrorContains(t, err, "still references it")
			require.ErrorContains(t, err, kind)
		})
	}
}

func TestAIGatewayAuthStrategyDeleteObservationRequired(t *testing.T) {
	for _, kind := range []string{
		ResourceTypeAIGatewayAgent, ResourceTypeAIGatewayModel, ResourceTypeAIGatewayMCPServer,
	} {
		t.Run(kind, func(t *testing.T) {
			cfg := authStrategyOrderClientConfig(t, "", "")
			switch kind {
			case ResourceTypeAIGatewayAgent:
				cfg.AIGatewayAgentsAPI = nil
			case ResourceTypeAIGatewayModel:
				cfg.AIGatewayModelAPI = nil
			case ResourceTypeAIGatewayMCPServer:
				cfg.AIGatewayMCPServersAPI = nil
			}
			p := NewPlanner(state.NewClient(cfg), slog.Default())
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
			require.NoError(
				t,
				p.resolveAIGatewayAuthStrategyDeletes(t.Context(), "default", "support-gateway", "gateway-id", plan),
			)
			plan.AddChange(PlannedChange{
				ID: "delete-authStrategy", ResourceType: ResourceTypeAIGatewayAuthStrategy, Action: ActionDelete,
				Namespace: "default", Parent: &ParentInfo{Ref: "support-gateway"},
				Fields: map[string]any{FieldName: "old-authStrategy"},
			})
			err := p.resolveAIGatewayAuthStrategyDeletes(t.Context(), "default", "support-gateway", "gateway-id", plan)
			require.ErrorContains(t, err, "failed to inspect auth strategy attachments")
		})
	}
}

func TestAIGatewayAuthStrategyDeleteWaitsForAllUsersWithinEachGateway(t *testing.T) {
	cfg := authStrategyOrderClientConfig(t, ResourceTypeAIGatewayAgent, "old-authStrategy")
	serverCfg := authStrategyOrderClientConfig(t, ResourceTypeAIGatewayMCPServer, "old-authStrategy")
	cfg.AIGatewayMCPServersAPI = serverCfg.AIGatewayMCPServersAPI
	p := NewPlanner(state.NewClient(cfg), slog.Default())
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	for _, gateway := range []string{"gateway-a", "gateway-b"} {
		plan.AddChange(PlannedChange{
			ID: gateway + "-authStrategy", ResourceType: ResourceTypeAIGatewayAuthStrategy, Action: ActionDelete,
			Fields: map[string]any{FieldName: "old-authStrategy"}, ResourceID: "authStrategy-id",
			Namespace: "default", Parent: &ParentInfo{Ref: gateway},
		})
		for _, kind := range []string{ResourceTypeAIGatewayAgent, ResourceTypeAIGatewayMCPServer} {
			plan.AddChange(PlannedChange{
				ID: gateway + "-" + kind, ResourceType: kind, Action: ActionDelete, ResourceID: "user-id",
				Namespace: "default", Parent: &ParentInfo{Ref: gateway},
			})
		}
	}
	for _, gateway := range []string{"gateway-a", "gateway-b"} {
		require.NoError(t, p.resolveAIGatewayAuthStrategyDeletes(t.Context(), "default", gateway, gateway, plan))
	}
	result, err := NewDependencyResolver().ResolveDependenciesWithGroups(plan.Changes)
	require.NoError(t, err)
	for _, change := range plan.Changes {
		if change.ResourceType == ResourceTypeAIGatewayAuthStrategy {
			require.ElementsMatch(t, []string{
				change.Parent.Ref + "-" + ResourceTypeAIGatewayAgent,
				change.Parent.Ref + "-" + ResourceTypeAIGatewayMCPServer,
			}, change.DependsOn)
		}
		for _, dependency := range result.FullDepsMap[change.ID] {
			require.Contains(t, dependency, change.Parent.Ref)
			require.Less(t, indexOf(result.ExecutionOrder, dependency), indexOf(result.ExecutionOrder, change.ID))
		}
	}
}

func TestAIGatewayAuthStrategyDeleteIgnoresUnrelatedAttachments(t *testing.T) {
	cfg := authStrategyOrderClientConfig(t, ResourceTypeAIGatewayMCPServer, "other-authStrategy")
	p := NewPlanner(state.NewClient(cfg), slog.Default())
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	plan.AddChange(PlannedChange{
		ID: "delete-authStrategy", ResourceType: ResourceTypeAIGatewayAuthStrategy, Action: ActionDelete,
		Fields: map[string]any{FieldName: "old-authStrategy"}, ResourceID: "authStrategy-id",
		Namespace: "default", Parent: &ParentInfo{Ref: "support-gateway"},
	})
	require.NoError(
		t,
		p.resolveAIGatewayAuthStrategyDeletes(t.Context(), "default", "support-gateway", "gateway-id", plan),
	)
	require.Empty(t, plan.Changes[0].DependsOn)
}

func authStrategyAccessFields(value any) map[string]any {
	return map[string]any{FieldAccess: map[string]any{FieldAuthStrategies: value}}
}

func authStrategyOrderClientConfig(t *testing.T, kind, reference string) state.ClientConfig {
	t.Helper()
	agents := &testAIGatewayAgentAPI{}
	models := &testAIGatewayModelAPI{}
	servers := &testAIGatewayMCPServerAPI{}
	payload := map[string]any{
		"id": "user-id", FieldName: "old-user", FieldDisplayName: "Old User",
		"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
		FieldAccess: map[string]any{FieldAuthStrategies: []string{reference}},
	}
	switch kind {
	case ResourceTypeAIGatewayAgent:
		payload[FieldType] = "a2a"
		payload[FieldConfig] = map[string]any{"url": "https://example.com"}
	case ResourceTypeAIGatewayModel:
		model := testAIGatewayModel("user-id", "old-user")
		fields, err := resources.AIGatewayModelMutablePayloadMap(model)
		require.NoError(t, err)
		for key, value := range fields {
			if _, present := payload[key]; !present {
				payload[key] = value
			}
		}
	case ResourceTypeAIGatewayMCPServer:
		payload[FieldType] = "listener"
		payload[FieldAccess].(map[string]any)["acl_attribute_type"] = "consumer"
		payload[FieldSources] = []string{}
		payload[FieldConfig] = map[string]any{FieldRoute: map[string]any{"paths": []string{"/test"}}}
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	switch kind {
	case ResourceTypeAIGatewayAgent:
		var agent kkComps.AIGatewayAgent
		require.NoError(t, json.Unmarshal(data, &agent))
		agents.agents = []kkComps.AIGatewayAgent{agent}
	case ResourceTypeAIGatewayModel:
		var model kkComps.AIGatewayModel
		require.NoError(t, json.Unmarshal(data, &model))
		models.models = []kkComps.AIGatewayModel{model}
	case ResourceTypeAIGatewayMCPServer:
		var server kkComps.AIGatewayMCPServer
		require.NoError(t, json.Unmarshal(data, &server))
		servers.servers = []kkComps.AIGatewayMCPServer{server}
	}
	var strategy kkComps.AIGatewayAuthStrategy
	require.NoError(t, json.Unmarshal([]byte(`{
		"id":"authStrategy-id","name":"old-authStrategy","display_name":"Old Auth",
		"type":"key-auth","config":{"key_names":["apikey"]},
		"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"
	}`), &strategy))
	return state.ClientConfig{
		AIGatewayAPI:               &testAIGatewayAPI{gateways: []kkComps.AIGateway{testAIGateway()}},
		AIGatewayAuthStrategiesAPI: &nameIdentityAuthStrategyAPI{children: []kkComps.AIGatewayAuthStrategy{strategy}},
		AIGatewayAgentsAPI:         agents, AIGatewayModelAPI: models, AIGatewayMCPServersAPI: servers,
	}
}
