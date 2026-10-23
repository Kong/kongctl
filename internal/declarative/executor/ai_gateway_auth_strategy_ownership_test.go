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

type authStrategyOwnershipAPI struct {
	helpers.AIGatewayAuthStrategiesAPI
	strategy         kkComps.AIGatewayAuthStrategy
	updates, deletes int
	last             kkComps.UpdateAIGatewayAuthStrategyRequest
}

func (a *authStrategyOwnershipAPI) GetAiGatewayAuthStrategy(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetAiGatewayAuthStrategyResponse, error) {
	return &kkOps.GetAiGatewayAuthStrategyResponse{AIGatewayAuthStrategy: &a.strategy}, nil
}

func (a *authStrategyOwnershipAPI) ListAiGatewayAuthStrategies(
	context.Context,
	kkOps.ListAiGatewayAuthStrategiesRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayAuthStrategiesResponse, error) {
	return &kkOps.ListAiGatewayAuthStrategiesResponse{
		ListAIGatewayAuthStrategiesResponse: &kkComps.ListAIGatewayAuthStrategiesResponse{
			Data: []kkComps.AIGatewayAuthStrategy{a.strategy},
		},
	}, nil
}

func (a *authStrategyOwnershipAPI) UpdateAiGatewayAuthStrategy(
	_ context.Context,
	r kkOps.UpdateAiGatewayAuthStrategyRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateAiGatewayAuthStrategyResponse, error) {
	a.updates++
	a.last = r.UpdateAIGatewayAuthStrategyRequest
	return &kkOps.UpdateAiGatewayAuthStrategyResponse{AIGatewayAuthStrategy: &a.strategy}, nil
}

func (a *authStrategyOwnershipAPI) DeleteAiGatewayAuthStrategy(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayAuthStrategyResponse, error) {
	a.deletes++
	return &kkOps.DeleteAiGatewayAuthStrategyResponse{}, nil
}

func TestAuthStrategyWritesRevalidateSavedOwnershipAndProtection(t *testing.T) {
	for _, variant := range []string{"key-auth", "openid-connect"} {
		for _, owner := range []string{"team-a", "team-b", ""} {
			for _, protected := range []bool{false, true} {
				t.Run(variant+"/"+owner+"/protected="+map[bool]string{false: "false", true: "true"}[protected], func(t *testing.T) {
					fields := map[string]any{
						planner.FieldName: "shared-auth", planner.FieldType: "key-auth", planner.FieldDisplayName: "Shared auth",
						planner.FieldConfig:    map[string]any{"key_names": []any{"apikey"}},
						planner.FieldManagedBy: resources.BuildChildManagedBy(nil, nil, "team-a", false, "dev"),
					}
					fields[planner.FieldType] = variant
					if variant == "openid-connect" {
						fields[planner.FieldConfig] = map[string]any{
							"issuer":            "https://issuer.example.test",
							"cache_tokens_salt": "test-cache-tokens-salt",
						}
					}
					change := planner.PlannedChange{
						ResourceType: planner.ResourceTypeAIGatewayAuthStrategy,
						Namespace:    "team-a",
						ResourceID:   "strategy-id",
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
					observed[planner.FieldID] = "strategy-id"
					observed["created_at"] = "2026-01-01T00:00:00Z"
					observed["updated_at"] = "2026-01-01T00:00:00Z"
					data, err := json.Marshal(observed)
					require.NoError(t, err)
					api := &authStrategyOwnershipAPI{}
					require.NoError(t, json.Unmarshal(data, &api.strategy))
					adapter := NewAIGatewayAuthStrategyAdapter(state.NewClient(state.ClientConfig{AIGatewayAuthStrategiesAPI: api}))
					ctx := NewExecutionContext(&saved)
					var req kkComps.UpdateAIGatewayAuthStrategyRequest
					require.NoError(t, adapter.MapUpdateFields(t.Context(), ctx, saved.Fields, &req, nil))
					_, err = adapter.Update(t.Context(), "strategy-id", req, "team-a", ctx)
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
					err = adapter.Delete(t.Context(), "strategy-id", ctx)
					if owner == "team-a" && !protected {
						require.NoError(t, err)
					} else {
						require.Error(t, err)
						require.Zero(t, api.deletes)
					}
					var create kkComps.CreateAIGatewayAuthStrategyRequest
					require.NoError(t, adapter.MapCreateFields(t.Context(), ctx, saved.Fields, &create))
					_, err = adapter.Create(t.Context(), create, "team-a", ctx)
					require.ErrorContains(t, err, "already exists")
				})
			}
		}
	}
}
