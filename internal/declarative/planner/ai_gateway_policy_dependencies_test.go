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

func TestAIGatewayPolicyDeleteOrdering(t *testing.T) {
	for _, kind := range []string{
		ResourceTypeAIGatewayAgent, ResourceTypeAIGatewayConsumer, ResourceTypeAIGatewayConsumerGroup,
		ResourceTypeAIGatewayModel, ResourceTypeAIGatewayMCPServer,
	} {
		for _, tt := range []struct {
			name         string
			action       ActionType
			fields       map[string]any
			currentByID  bool
			otherGateway bool
			otherNS      bool
			wantError    string
		}{
			{name: "delete before policy", action: ActionDelete},
			{name: "remote policy ID", action: ActionDelete, currentByID: true},
			{name: "detach update", action: ActionUpdate, fields: map[string]any{FieldPolicies: []any{}}},
			{
				name: "replace policy", action: ActionUpdate,
				fields: map[string]any{FieldPolicies: []any{"__REF__:replacement#name"}},
			},
			{name: "retained resource", wantError: "still references it"},
			{
				name: "unrelated update", action: ActionUpdate,
				fields: map[string]any{FieldDisplayName: "New display name"}, wantError: "still references it",
			},
			{
				name: "null does not prove detachment", action: ActionUpdate,
				fields: map[string]any{FieldPolicies: nil}, wantError: "still references it",
			},
			{
				name: "malformed list does not prove detachment", action: ActionUpdate,
				fields: map[string]any{FieldPolicies: []any{"replacement", 42}}, wantError: "still references it",
			},
			{
				name: "update retains reference", action: ActionUpdate,
				fields: map[string]any{FieldPolicies: []string{"old-policy"}}, wantError: "planned",
			},
			{
				name: "update retains ID reference", action: ActionUpdate,
				fields: map[string]any{FieldPolicies: []any{"policy-id"}}, wantError: "planned",
			},
			{
				name: "create references deleted policy", action: ActionCreate,
				fields: map[string]any{FieldPolicies: []any{"__REF__:old-policy#name"}}, wantError: "planned",
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
				policyReference := "old-policy"
				if tt.currentByID {
					policyReference = "policy-id"
				}
				cfg := policyOrderClientConfig(t, kind, policyReference)
				p := NewPlanner(state.NewClient(cfg), slog.Default())
				plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
				plan.Changes = []PlannedChange{
					{
						ID: "create-policy", ResourceType: ResourceTypeAIGatewayPolicy, Action: ActionCreate,
						ResourceRef: "replacement", Namespace: "default", Parent: &ParentInfo{Ref: "support-gateway"},
					},
					{
						ID: "delete-policy", ResourceType: ResourceTypeAIGatewayPolicy, Action: ActionDelete,
						ResourceID: "policy-id", Fields: map[string]any{FieldName: "old-policy"},
						Namespace: "default", Parent: &ParentInfo{Ref: "support-gateway"},
					},
				}
				if tt.action != "" {
					change := PlannedChange{
						ID: "user-change", ResourceType: kind, Action: tt.action, ResourceID: "user-id",
						ResourceRef: "old-user", Fields: tt.fields, Namespace: "default",
						Parent: &ParentInfo{Ref: "support-gateway"},
					}
					if tt.action == ActionUpdate {
						change.DependsOn = []string{"create-policy"}
					}
					if tt.otherGateway {
						change.Parent.Ref = "other-gateway"
					}
					if tt.otherNS {
						change.Namespace = "other"
					}
					plan.AddChange(change)
				}
				err := p.resolveAIGatewayPolicyDeletes(t.Context(), "default", "support-gateway", "gateway-id", plan)
				if tt.wantError != "" {
					require.ErrorContains(t, err, tt.wantError)
					return
				}
				require.NoError(t, err)
				require.Contains(t, plan.Changes[1].DependsOn, "user-change")
				result, err := NewDependencyResolver().ResolveDependenciesWithGroups(plan.Changes)
				require.NoError(t, err)
				require.Less(t, indexOf(result.ExecutionOrder, "user-change"), indexOf(result.ExecutionOrder, "delete-policy"))
				if tt.action == ActionUpdate {
					require.Less(t, indexOf(result.ExecutionOrder, "create-policy"), indexOf(result.ExecutionOrder, "user-change"))
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

func TestAIGatewayPolicyDeleteInspectsOutOfScopeResources(t *testing.T) {
	for _, kind := range []string{
		ResourceTypeAIGatewayAgent, ResourceTypeAIGatewayConsumer, ResourceTypeAIGatewayConsumerGroup,
		ResourceTypeAIGatewayModel, ResourceTypeAIGatewayMCPServer,
	} {
		t.Run(kind, func(t *testing.T) {
			cfg := policyOrderClientConfig(t, kind, "old-policy")
			scope := resources.NewSyncScope()
			scope.AddRoot(resources.ResourceTypeAIGateway)
			scope.AddChild(resources.ResourceTypeAIGateway, "support-gateway", resources.ResourceTypeAIGatewayPolicy)
			rs := &resources.ResourceSet{
				AIGateways: []resources.AIGatewayResource{testAIGatewayResource()}, SyncScope: scope,
			}
			_, err := NewPlanner(state.NewClient(cfg), slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
			require.ErrorContains(t, err, "still references it")
			require.ErrorContains(t, err, kind)
		})
	}
}

func TestAIGatewayPolicyDeleteObservationRequired(t *testing.T) {
	for _, kind := range []string{
		ResourceTypeAIGatewayAgent, ResourceTypeAIGatewayConsumer, ResourceTypeAIGatewayConsumerGroup,
		ResourceTypeAIGatewayModel, ResourceTypeAIGatewayMCPServer,
	} {
		t.Run(kind, func(t *testing.T) {
			cfg := policyOrderClientConfig(t, "", "")
			switch kind {
			case ResourceTypeAIGatewayAgent:
				cfg.AIGatewayAgentsAPI = nil
			case ResourceTypeAIGatewayConsumer:
				cfg.AIGatewayConsumersAPI = nil
			case ResourceTypeAIGatewayConsumerGroup:
				cfg.AIGatewayConsumerGroupsAPI = nil
			case ResourceTypeAIGatewayModel:
				cfg.AIGatewayModelAPI = nil
			case ResourceTypeAIGatewayMCPServer:
				cfg.AIGatewayMCPServersAPI = nil
			}
			p := NewPlanner(state.NewClient(cfg), slog.Default())
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
			require.NoError(t, p.resolveAIGatewayPolicyDeletes(t.Context(), "default", "support-gateway", "gateway-id", plan))
			plan.AddChange(PlannedChange{
				ID: "delete-policy", ResourceType: ResourceTypeAIGatewayPolicy, Action: ActionDelete,
				Namespace: "default", Parent: &ParentInfo{Ref: "support-gateway"},
				Fields: map[string]any{FieldName: "old-policy"},
			})
			err := p.resolveAIGatewayPolicyDeletes(t.Context(), "default", "support-gateway", "gateway-id", plan)
			require.ErrorContains(t, err, "failed to inspect policy attachments")
		})
	}
}

func TestAIGatewayPolicyDeleteWaitsForAllUsersWithinEachGateway(t *testing.T) {
	cfg := policyOrderClientConfig(t, ResourceTypeAIGatewayAgent, "old-policy")
	serverCfg := policyOrderClientConfig(t, ResourceTypeAIGatewayMCPServer, "old-policy")
	cfg.AIGatewayMCPServersAPI = serverCfg.AIGatewayMCPServersAPI
	p := NewPlanner(state.NewClient(cfg), slog.Default())
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	for _, gateway := range []string{"gateway-a", "gateway-b"} {
		plan.AddChange(PlannedChange{
			ID: gateway + "-policy", ResourceType: ResourceTypeAIGatewayPolicy, Action: ActionDelete,
			Fields: map[string]any{FieldName: "old-policy"}, ResourceID: "policy-id",
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
		require.NoError(t, p.resolveAIGatewayPolicyDeletes(t.Context(), "default", gateway, gateway, plan))
	}
	result, err := NewDependencyResolver().ResolveDependenciesWithGroups(plan.Changes)
	require.NoError(t, err)
	for _, change := range plan.Changes {
		if change.ResourceType == ResourceTypeAIGatewayPolicy {
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

func TestAIGatewayPolicyDeleteIgnoresUnrelatedAttachments(t *testing.T) {
	cfg := policyOrderClientConfig(t, ResourceTypeAIGatewayMCPServer, "other-policy")
	p := NewPlanner(state.NewClient(cfg), slog.Default())
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	plan.AddChange(PlannedChange{
		ID: "delete-policy", ResourceType: ResourceTypeAIGatewayPolicy, Action: ActionDelete,
		Fields: map[string]any{FieldName: "old-policy"}, ResourceID: "policy-id",
		Namespace: "default", Parent: &ParentInfo{Ref: "support-gateway"},
	})
	require.NoError(t, p.resolveAIGatewayPolicyDeletes(t.Context(), "default", "support-gateway", "gateway-id", plan))
	require.Empty(t, plan.Changes[0].DependsOn)
}

func policyOrderClientConfig(t *testing.T, kind, policyReference string) state.ClientConfig {
	t.Helper()
	agents := &testAIGatewayAgentAPI{}
	consumers := &testAIGatewayConsumerAPI{}
	groups := &testAIGatewayConsumerGroupAPI{}
	models := &testAIGatewayModelAPI{}
	servers := &testAIGatewayMCPServerAPI{}
	policies := []string{policyReference}
	switch kind {
	case ResourceTypeAIGatewayAgent:
		agent := testAIGatewayAgent(policies)
		agent.ID, agent.Name = "user-id", "old-user"
		agents.agents = []kkComps.AIGatewayAgent{agent}
	case ResourceTypeAIGatewayConsumer:
		consumer := testAIGatewayConsumer(policies)
		consumer.ID, consumer.Name = "user-id", "old-user"
		consumers.consumers = []kkComps.AIGatewayConsumer{consumer}
	case ResourceTypeAIGatewayConsumerGroup:
		group := testAIGatewayConsumerGroup(policies)
		group.ID, group.Name = "user-id", "old-user"
		groups.groups = []kkComps.AIGatewayConsumerGroup{group}
	case ResourceTypeAIGatewayModel:
		model := testAIGatewayModel("user-id", "old-user")
		payload, err := resources.AIGatewayModelMutablePayloadMap(model)
		require.NoError(t, err)
		payload["id"], payload[FieldPolicies] = "user-id", policies
		payload["created_at"], payload["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
		data, err := json.Marshal(payload)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, &model))
		models.models = []kkComps.AIGatewayModel{model}
	case ResourceTypeAIGatewayMCPServer:
		server := testAIGatewayMCPServer(t, "user-id", "old-user")
		payload, err := resources.AIGatewayMCPServerMutablePayloadMap(server)
		require.NoError(t, err)
		payload["id"], payload[FieldPolicies] = "user-id", policies
		payload["created_at"], payload["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
		data, err := json.Marshal(payload)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, &server))
		servers.servers = []kkComps.AIGatewayMCPServer{server}
	}
	policy := testAIGatewayPolicy()
	policy.ID, policy.Name = "policy-id", "old-policy"
	return state.ClientConfig{
		AIGatewayAPI:         &testAIGatewayAPI{gateways: []kkComps.AIGateway{testAIGateway()}},
		AIGatewayPoliciesAPI: &testAIGatewayPolicyAPI{policies: []kkComps.AIGatewayPolicy{policy}},
		AIGatewayAgentsAPI:   agents, AIGatewayConsumersAPI: consumers, AIGatewayConsumerGroupsAPI: groups,
		AIGatewayModelAPI: models, AIGatewayMCPServersAPI: servers,
	}
}

// Exercise SDK serialization and the real diff builders instead of supplying an
// empty policies field that omitempty removes in production.
func TestAIGatewayPolicyFullDetachFromDesiredPayload(t *testing.T) {
	for _, kind := range []string{
		ResourceTypeAIGatewayAgent, ResourceTypeAIGatewayConsumer, ResourceTypeAIGatewayConsumerGroup,
		ResourceTypeAIGatewayModel, ResourceTypeAIGatewayMCPServer,
	} {
		for _, explicitEmpty := range []bool{false, true} {
			name := "omitted"
			if explicitEmpty {
				name = "empty"
			}
			t.Run(kind+"/"+name, func(t *testing.T) {
				cfg := policyOrderClientConfig(t, kind, "old-policy")
				p := NewPlanner(state.NewClient(cfg), slog.Default())
				p.resources = &resources.ResourceSet{}
				payload := map[string]any{
					"ref": "old-user", "ai_gateway": "support-gateway",
					FieldName: "old-user", FieldDisplayName: "Updated User",
				}
				if explicitEmpty {
					payload[FieldPolicies] = []string{}
				}
				var fields map[string]any
				var changed map[string]FieldChange
				var needsUpdate bool
				var err error
				switch kind {
				case ResourceTypeAIGatewayAgent:
					payload[FieldType] = "a2a"
					payload[FieldConfig] = map[string]any{"url": "https://example.com"}
					data, marshalErr := json.Marshal(payload)
					require.NoError(t, marshalErr)
					var desired resources.AIGatewayAgentResource
					require.NoError(t, json.Unmarshal(data, &desired))
					current := cfg.AIGatewayAgentsAPI.(*testAIGatewayAgentAPI).agents[0]
					observed := state.AIGatewayAgent{AIGatewayAgent: current}
					needsUpdate, fields, changed, err = p.shouldUpdateAIGatewayAgent(observed, desired)
				case ResourceTypeAIGatewayConsumer:
					payload[FieldType] = "api-key"
					data, marshalErr := json.Marshal(payload)
					require.NoError(t, marshalErr)
					var desired resources.AIGatewayConsumerResource
					require.NoError(t, json.Unmarshal(data, &desired))
					current := cfg.AIGatewayConsumersAPI.(*testAIGatewayConsumerAPI).consumers[0]
					observed := state.AIGatewayConsumer{AIGatewayConsumer: current}
					needsUpdate, fields, changed, err = p.shouldUpdateAIGatewayConsumer(observed, desired)
				case ResourceTypeAIGatewayConsumerGroup:
					data, marshalErr := json.Marshal(payload)
					require.NoError(t, marshalErr)
					var desired resources.AIGatewayConsumerGroupResource
					require.NoError(t, json.Unmarshal(data, &desired))
					current := cfg.AIGatewayConsumerGroupsAPI.(*testAIGatewayConsumerGroupAPI).groups[0]
					observed := state.AIGatewayConsumerGroup{AIGatewayConsumerGroup: current}
					needsUpdate, fields, changed, err = p.shouldUpdateAIGatewayConsumerGroup(observed, desired, nil)
				case ResourceTypeAIGatewayModel:
					payload[FieldType] = "model"
					payload[FieldCapabilities] = []string{"generate"}
					payload[FieldConfig] = map[string]any{FieldRoute: map[string]any{}, FieldModel: map[string]any{}}
					payload[FieldTargets] = []any{map[string]any{
						FieldName: "gpt-4o", FieldProvider: "support-openai",
						FieldConfig: map[string]any{FieldType: "openai"},
					}}
					payload[FieldFormats] = []any{map[string]any{FieldType: "openai"}}
					data, marshalErr := json.Marshal(payload)
					require.NoError(t, marshalErr)
					var desired resources.AIGatewayModelResource
					require.NoError(t, json.Unmarshal(data, &desired))
					current := cfg.AIGatewayModelAPI.(*testAIGatewayModelAPI).models[0]
					observed := state.AIGatewayModel{AIGatewayModel: current}
					needsUpdate, fields, changed, err = p.shouldUpdateAIGatewayModel(observed, desired)
				case ResourceTypeAIGatewayMCPServer:
					payload[FieldType] = "conversion-only"
					payload[FieldTools] = []any{map[string]any{FieldName: "tool", FieldDescription: "Lookup", "method": "GET"}}
					payload[FieldConfig] = map[string]any{"url": "https://example.com"}
					data, marshalErr := json.Marshal(payload)
					require.NoError(t, marshalErr)
					var desired resources.AIGatewayMCPServerResource
					require.NoError(t, json.Unmarshal(data, &desired))
					current := cfg.AIGatewayMCPServersAPI.(*testAIGatewayMCPServerAPI).servers[0]
					observed := state.AIGatewayMCPServer{AIGatewayMCPServer: current}
					needsUpdate, fields, changed, err = p.shouldUpdateAIGatewayMCPServer(observed, desired)
				}
				require.NoError(t, err)
				require.True(t, needsUpdate)
				require.NotContains(t, fields, FieldPolicies)
				require.Contains(t, changed, FieldPolicies)
				require.Nil(t, changed[FieldPolicies].New)
				plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
				plan.AddChange(PlannedChange{
					ID: "delete-policy", ResourceType: ResourceTypeAIGatewayPolicy, Action: ActionDelete,
					ResourceID: "policy-id", Fields: map[string]any{FieldName: "old-policy"},
					Namespace: "default", Parent: &ParentInfo{Ref: "support-gateway"},
				})
				plan.AddChange(PlannedChange{
					ID: "detach-user", ResourceType: kind, Action: ActionUpdate, ResourceID: "user-id",
					Fields: fields, ChangedFields: changed,
					Namespace: "default", Parent: &ParentInfo{Ref: "support-gateway"},
				})
				require.NoError(t, p.resolveAIGatewayPolicyDeletes(t.Context(), "default", "support-gateway", "gateway-id", plan))
				require.Contains(t, plan.Changes[0].DependsOn, "detach-user")
			})
		}
	}
}
