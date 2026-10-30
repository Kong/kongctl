package executor

import (
	"context"
	"encoding/json"
	"maps"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

type agentOwnershipAPI struct {
	helpers.AIGatewayAgentsAPI
	agent            kkComps.AIGatewayAgent
	updates, deletes int
	last             kkComps.UpdateAIGatewayAgentRequest
}

func (a *agentOwnershipAPI) GetAiGatewayAgent(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetAiGatewayAgentResponse, error) {
	return &kkOps.GetAiGatewayAgentResponse{AIGatewayAgent: &a.agent}, nil
}

func (a *agentOwnershipAPI) ListAiGatewayAgents(
	context.Context,
	kkOps.ListAiGatewayAgentsRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayAgentsResponse, error) {
	return &kkOps.ListAiGatewayAgentsResponse{
		ListAIGatewayAgentsResponse: &kkComps.ListAIGatewayAgentsResponse{
			Data: []kkComps.AIGatewayAgent{a.agent},
		},
	}, nil
}

func (a *agentOwnershipAPI) UpdateAiGatewayAgent(
	_ context.Context,
	r kkOps.UpdateAiGatewayAgentRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateAiGatewayAgentResponse, error) {
	a.updates++
	a.last = r.UpdateAIGatewayAgentRequest
	return &kkOps.UpdateAiGatewayAgentResponse{AIGatewayAgent: &a.agent}, nil
}

func (a *agentOwnershipAPI) DeleteAiGatewayAgent(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayAgentResponse, error) {
	a.deletes++
	return &kkOps.DeleteAiGatewayAgentResponse{}, nil
}

func TestAgentWritesRevalidateSavedOwnershipAndProtection(t *testing.T) {
	for _, variant := range []string{"a2a", "http"} {
		for _, owner := range []string{"team-a", "team-b", ""} {
			for _, protected := range []bool{false, true} {
				t.Run(
					variant+"/"+owner+"/protected="+map[bool]string{false: "false", true: "true"}[protected],
					func(t *testing.T) {
						fields := map[string]any{
							planner.FieldName: "shared-auth", planner.FieldType: "a2a", planner.FieldDisplayName: "Shared auth",
							planner.FieldConfig:    map[string]any{"url": "https://example.com"},
							planner.FieldManagedBy: resources.BuildChildManagedBy("team-a", false, "dev"),
						}
						fields[planner.FieldType] = variant
						change := planner.PlannedChange{
							ResourceType: planner.ResourceTypeAIGatewayAgent,
							Namespace:    "team-a",
							ResourceID:   "agent-id",
							Fields:       fields,
							Parent:       &planner.ParentInfo{ID: "gateway-id"},
						}
						encoded, err := json.Marshal(change)
						require.NoError(t, err)
						var saved planner.PlannedChange
						require.NoError(t, json.Unmarshal(encoded, &saved))
						observed := map[string]any{}
						maps.Copy(observed, fields)
						observed[planner.FieldLabels] = map[string]string{"added-after-planning": "retained"}
						observed[planner.FieldManagedBy] = resources.BuildChildManagedBy(
							owner,
							protected,
							"dev",
						)
						observed[planner.FieldID] = "agent-id"
						observed["created_at"] = "2026-01-01T00:00:00Z"
						observed["updated_at"] = "2026-01-01T00:00:00Z"
						data, err := json.Marshal(observed)
						require.NoError(t, err)
						api := &agentOwnershipAPI{}
						require.NoError(t, json.Unmarshal(data, &api.agent))
						adapter := NewAIGatewayAgentAdapter(
							state.NewClient(state.ClientConfig{AIGatewayAgentsAPI: api}),
						)
						ctx := NewExecutionContext(&saved)
						var req kkComps.UpdateAIGatewayAgentRequest
						require.NoError(t, adapter.MapUpdateFields(t.Context(), ctx, saved.Fields, &req, nil))
						_, err = adapter.Update(t.Context(), "agent-id", req, "team-a", ctx)
						if owner == "team-a" && !protected {
							require.NoError(t, err)
							require.Equal(t, 1, api.updates)
							p, err := ownedChildPayload(api.last)
							require.NoError(t, err)
							require.NotContains(t, childPayloadManagedBy(p), "other-tool")
							require.Equal(t, map[string]any{"added-after-planning": "retained"}, p[planner.FieldLabels])
						} else {
							require.Error(t, err)
							require.Zero(t, api.updates)
						}
						err = adapter.Delete(t.Context(), "agent-id", ctx)
						if owner == "team-a" && !protected {
							require.NoError(t, err)
						} else {
							require.Error(t, err)
							require.Zero(t, api.deletes)
						}
						var create kkComps.CreateAIGatewayAgentRequest
						require.NoError(t, adapter.MapCreateFields(t.Context(), ctx, saved.Fields, &create))
						_, err = adapter.Create(t.Context(), create, "team-a", ctx)
						require.ErrorContains(t, err, "already exists")
					},
				)
			}
		}
	}
}

func TestAgentUnprotectionPreservesFreshDefaultsAndRejectsPayloadInjection(t *testing.T) {
	for _, mutation := range []string{"none", "url", "password", "type", "secret-write"} {
		t.Run(mutation, func(t *testing.T) {
			metadata := resources.BuildChildManagedBy("team-a", true, "dev")
			api := &agentOwnershipAPI{
				agent: kkComps.AIGatewayAgent{
					ID:          "agent-id",
					Name:        "agent",
					DisplayName: "Agent",
					Type:        "a2a",
					Enabled:     new(true),
					ManagedBy:   metadata,
					Config: kkComps.AIGatewayAgentConfig{
						URL:                "https://example.com",
						MaxRequestBodySize: new(int64(8388608)),
					},
				},
			}
			fields := map[string]any{
				planner.FieldName:        "agent",
				planner.FieldDisplayName: "Agent",
				planner.FieldType:        "a2a",
				planner.FieldConfig:      map[string]any{"url": "https://example.com"},
				planner.FieldManagedBy:   resources.BuildChildManagedBy("team-a", false, "dev"),
			}
			change := planner.PlannedChange{
				Namespace: "team-a",
				Fields:    fields,
				Parent:    &planner.ParentInfo{ID: "gateway-id"},
				ChangedFields: map[string]planner.FieldChange{
					planner.FieldManagedBy: {Old: metadata, New: fields[planner.FieldManagedBy]},
				},
				Protection: planner.ProtectionChange{Old: true, New: false},
			}
			adapter := NewAIGatewayAgentAdapter(state.NewClient(state.ClientConfig{AIGatewayAgentsAPI: api}))
			ctx := NewExecutionContext(&change)
			var req kkComps.UpdateAIGatewayAgentRequest
			require.NoError(t, adapter.MapUpdateFields(t.Context(), ctx, fields, &req, nil))
			switch mutation {
			case "url":
				req.Config.URL = "https://changed.example.com"
			case "password":
				req.Config.Proxy = &kkComps.AIGatewayProxyConfig{
					Auth: &kkComps.AIGatewayProxyConfigAuth{Username: new("user"), Password: new("synthetic-injected")},
				}
			case "type":
				req.Type = "http"
			case "secret-write":
				change.SecretWrites = []planner.SecretWriteIntent{{Field: "/config/proxy/auth/password"}}
			}
			_, err := adapter.Update(t.Context(), "agent-id", req, "team-a", ctx)
			if mutation != "none" {
				require.Error(t, err)
				require.Zero(t, api.updates)
				return
			}
			require.NoError(t, err)
			require.Equal(t, api.agent.Config.MaxRequestBodySize, api.last.Config.MaxRequestBodySize)
			require.Equal(t, api.agent.Enabled, api.last.Enabled)
		})
	}
}

func TestAgentSecretOnlyWritesRevalidateOwnershipAndProtection(t *testing.T) {
	for _, owner := range []string{"team-a", "team-b", ""} {
		for _, protected := range []bool{false, true} {
			t.Run(owner+map[bool]string{false: "/writable", true: "/protected"}[protected], func(t *testing.T) {
				metadata := resources.BuildChildManagedBy(owner, protected, "dev")
				api := &agentOwnershipAPI{
					agent: kkComps.AIGatewayAgent{
						ID:          "agent-id",
						Name:        "agent",
						DisplayName: "Agent",
						Type:        "a2a",
						ManagedBy:   metadata,
						Config:      kkComps.AIGatewayAgentConfig{URL: "https://example.com"},
					},
				}
				fields := map[string]any{
					planner.FieldName:        "agent",
					planner.FieldDisplayName: "Agent",
					planner.FieldType:        "a2a",
					planner.FieldConfig:      map[string]any{"url": "https://example.com"},
					planner.FieldManagedBy:   resources.BuildChildManagedBy("team-a", false, "dev"),
				}
				change := planner.PlannedChange{
					Namespace:    "team-a",
					Fields:       fields,
					Parent:       &planner.ParentInfo{ID: "gateway-id"},
					SecretWrites: []planner.SecretWriteIntent{{Field: "/config/proxy/auth/password"}},
				}
				adapter := NewAIGatewayAgentAdapter(state.NewClient(state.ClientConfig{AIGatewayAgentsAPI: api}))
				ctx := NewExecutionContext(&change)
				var req kkComps.UpdateAIGatewayAgentRequest
				require.NoError(t, adapter.MapUpdateFields(t.Context(), ctx, fields, &req, nil))
				req.Config.Proxy = &kkComps.AIGatewayProxyConfig{
					Auth: &kkComps.AIGatewayProxyConfigAuth{Username: new("user"), Password: new("synthetic-rotation")},
				}
				_, err := adapter.Update(t.Context(), "agent-id", req, "team-a", ctx)
				if owner != "team-a" || protected {
					require.Error(t, err)
					require.Zero(t, api.updates)
					return
				}
				require.NoError(t, err)
				require.Equal(t, 1, api.updates)
			})
		}
	}
}
