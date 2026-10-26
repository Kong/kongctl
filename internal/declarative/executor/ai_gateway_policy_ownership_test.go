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

type policyOwnershipAPI struct {
	helpers.AIGatewayPoliciesAPI
	policy           kkComps.AIGatewayPolicy
	updates, deletes int
	last             kkComps.UpdateAIGatewayPolicyRequest
}

func (a *policyOwnershipAPI) GetAiGatewayPolicy(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetAiGatewayPolicyResponse, error) {
	return &kkOps.GetAiGatewayPolicyResponse{AIGatewayPolicy: &a.policy}, nil
}

func (a *policyOwnershipAPI) ListAiGatewayPolicies(
	context.Context,
	kkOps.ListAiGatewayPoliciesRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayPoliciesResponse, error) {
	return &kkOps.ListAiGatewayPoliciesResponse{
		ListAIGatewayPoliciesResponse: &kkComps.ListAIGatewayPoliciesResponse{
			Data: []kkComps.AIGatewayPolicy{a.policy},
		},
	}, nil
}

func (a *policyOwnershipAPI) UpdateAiGatewayPolicy(
	_ context.Context,
	r kkOps.UpdateAiGatewayPolicyRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateAiGatewayPolicyResponse, error) {
	a.updates++
	a.last = r.UpdateAIGatewayPolicyRequest
	return &kkOps.UpdateAiGatewayPolicyResponse{AIGatewayPolicy: &a.policy}, nil
}

func (a *policyOwnershipAPI) DeleteAiGatewayPolicy(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayPolicyResponse, error) {
	a.deletes++
	return &kkOps.DeleteAiGatewayPolicyResponse{}, nil
}

func TestPolicyWritesRevalidateSavedOwnershipAndProtection(t *testing.T) {
	for _, variant := range []string{"ai-sanitizer", "rate-limiting"} {
		for _, owner := range []string{"team-a", "team-b", ""} {
			for _, protected := range []bool{false, true} {
				t.Run(variant+"/"+owner+"/protected="+map[bool]string{false: "false", true: "true"}[protected], func(t *testing.T) {
					fields := map[string]any{
						planner.FieldName: "shared-auth", planner.FieldType: "ai-sanitizer", planner.FieldDisplayName: "Shared auth",
						planner.FieldConfig:    map[string]any{"anonymize": []any{"email"}},
						planner.FieldManagedBy: resources.BuildChildManagedBy(nil, nil, "team-a", false, "dev"),
					}
					fields[planner.FieldType] = variant
					if variant == "rate-limiting" {
						fields[planner.FieldConfig] = map[string]any{"minute": 100, "policy": "local"}
					}
					change := planner.PlannedChange{
						ResourceType: planner.ResourceTypeAIGatewayPolicy,
						Namespace:    "team-a",
						ResourceID:   "policy-id",
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
						nil,
						map[string]string{"other-tool": "preserved"},
						owner,
						protected,
						"dev",
					)
					observed[planner.FieldID] = "policy-id"
					observed["created_at"] = "2026-01-01T00:00:00Z"
					observed["updated_at"] = "2026-01-01T00:00:00Z"
					data, err := json.Marshal(observed)
					require.NoError(t, err)
					api := &policyOwnershipAPI{}
					require.NoError(t, json.Unmarshal(data, &api.policy))
					adapter := NewAIGatewayPolicyAdapter(state.NewClient(state.ClientConfig{AIGatewayPoliciesAPI: api}))
					ctx := NewExecutionContext(&saved)
					var req kkComps.UpdateAIGatewayPolicyRequest
					require.NoError(t, adapter.MapUpdateFields(t.Context(), ctx, saved.Fields, &req, nil))
					_, err = adapter.Update(t.Context(), "policy-id", req, "team-a", ctx)
					if owner == "team-a" && !protected {
						require.NoError(t, err)
						require.Equal(t, 1, api.updates)
						p, err := ownedChildPayload(api.last)
						require.NoError(t, err)
						require.Equal(t, "preserved", childPayloadManagedBy(p)["other-tool"])
						require.Equal(t, map[string]any{"added-after-planning": "retained"}, p[planner.FieldLabels])
					} else {
						require.Error(t, err)
						require.Zero(t, api.updates)
					}
					err = adapter.Delete(t.Context(), "policy-id", ctx)
					if owner == "team-a" && !protected {
						require.NoError(t, err)
					} else {
						require.Error(t, err)
						require.Zero(t, api.deletes)
					}
					var create kkComps.CreateAIGatewayPolicyRequest
					require.NoError(t, adapter.MapCreateFields(t.Context(), ctx, saved.Fields, &create))
					_, err = adapter.Create(t.Context(), create, "team-a", ctx)
					require.ErrorContains(t, err, "already exists")
				})
			}
		}
	}
}

func TestPolicyMetadataWritesPreserveFreshConfigWithoutAddingItToPlans(t *testing.T) {
	for _, protected := range []bool{false, true} {
		for _, mutation := range []string{"none", "config", "global", "condition", "name"} {
			t.Run(map[bool]string{false: "protect", true: "unprotect"}[protected]+"/"+mutation, func(t *testing.T) {
				const observedValue = "synthetic-observed-value"
				current := resources.BuildChildManagedBy(
					nil, map[string]string{"inventory": "preserved"}, "team-a", protected, "dev",
				)
				requested := resources.BuildChildManagedBy(current, nil, "team-a", !protected, "dev")
				fields := map[string]any{
					planner.FieldName: "policy", planner.FieldType: "ai-sanitizer", planner.FieldDisplayName: "Policy",
					planner.FieldConfig: map[string]any{"anonymize": []any{"email"}}, planner.FieldEnabled: true,
					planner.FieldGlobal: false, planner.FieldManagedBy: requested,
				}
				change := planner.PlannedChange{
					ResourceType: planner.ResourceTypeAIGatewayPolicy, Namespace: "team-a", Fields: fields,
					Parent:        &planner.ParentInfo{ID: "gateway-id"},
					Protection:    planner.ProtectionChange{Old: protected, New: !protected},
					ChangedFields: map[string]planner.FieldChange{planner.FieldManagedBy: {Old: current, New: requested}},
				}
				encoded, err := json.Marshal(change)
				require.NoError(t, err)
				require.NotContains(t, string(encoded), observedValue)
				api := &policyOwnershipAPI{policy: kkComps.AIGatewayPolicy{
					ID: "policy-id", Name: "policy", Type: "ai-sanitizer", DisplayName: "Policy",
					Enabled: new(true), Global: new(false),
					Condition: new("http.path == '/v1/chat/completions'"),
					Labels:    map[string]string{"fresh": "retained"}, ManagedBy: current,
					Config: map[string]any{"anonymize": []any{"email"}, "backend_only_credential": observedValue},
				}}
				adapter := NewAIGatewayPolicyAdapter(state.NewClient(state.ClientConfig{AIGatewayPoliciesAPI: api}))
				ctx := NewExecutionContext(&change)
				var req kkComps.UpdateAIGatewayPolicyRequest
				require.NoError(t, adapter.MapUpdateFields(t.Context(), ctx, fields, &req, nil))
				switch mutation {
				case "config":
					req.Config = map[string]any{"anonymize": []any{"phone"}}
				case "global":
					req.Global = new(true)
				case "condition":
					req.Condition = new("true")
				case "name":
					req.Name = "renamed"
				}
				_, err = adapter.Update(t.Context(), "policy-id", req, "team-a", ctx)
				if mutation != "none" {
					require.Error(t, err)
					require.Zero(t, api.updates)
					return
				}
				require.NoError(t, err)
				require.Equal(t, observedValue, api.last.Config["backend_only_credential"])
				require.Equal(t, api.policy.Condition, api.last.Condition)
				require.Equal(t, api.policy.Labels, api.last.Labels)
				require.Equal(t, !protected, api.last.ManagedBy[resources.ChildProtectedKey] == "true")
			})
		}
	}
}

func TestPolicyTypeUpdatesRetainBackendBehavior(t *testing.T) {
	for _, protected := range []bool{false, true} {
		t.Run(map[bool]string{false: "unprotected", true: "protected"}[protected], func(t *testing.T) {
			metadata := resources.BuildChildManagedBy(nil, nil, "team-a", protected, "dev")
			api := &policyOwnershipAPI{policy: kkComps.AIGatewayPolicy{
				ID: "policy-id", Name: "policy", DisplayName: "Policy",
				Type: "ai-sanitizer", Config: map[string]any{"anonymize": []any{"email"}}, ManagedBy: metadata,
			}}
			fields := map[string]any{
				planner.FieldName: "policy", planner.FieldDisplayName: "Policy", planner.FieldType: "rate-limiting",
				planner.FieldConfig: map[string]any{"minute": 100, "policy": "local"}, planner.FieldManagedBy: metadata,
			}
			change := planner.PlannedChange{
				Namespace: "team-a", Fields: fields, Parent: &planner.ParentInfo{ID: "gateway-id"},
				ChangedFields: map[string]planner.FieldChange{planner.FieldType: {Old: "ai-sanitizer", New: "rate-limiting"}},
			}
			adapter := NewAIGatewayPolicyAdapter(state.NewClient(state.ClientConfig{AIGatewayPoliciesAPI: api}))
			ctx := NewExecutionContext(&change)
			var req kkComps.UpdateAIGatewayPolicyRequest
			require.NoError(t, adapter.MapUpdateFields(t.Context(), ctx, fields, &req, nil))
			_, err := adapter.Update(t.Context(), "policy-id", req, "team-a", ctx)
			if protected {
				require.ErrorContains(t, err, "protected")
				require.Zero(t, api.updates)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "rate-limiting", api.last.Type)
		})
	}
}
