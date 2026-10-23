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

func TestFinalGatewayDependenciesAcrossNamespacesAndAliases(t *testing.T) {
	for _, owner := range []string{"team-b", ""} {
		for _, action := range []ActionType{"", ActionUpdate, ActionDelete} {
			t.Run(owner+"/"+string(action), func(t *testing.T) {
				payload, err := testAIGatewayModelResource(t).MutablePayloadMap()
				require.NoError(t, err)
				payload["id"] = "model-id"
				payload["created_at"], payload["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
				payload[FieldManagedBy] = map[string]string{
					resources.ChildNamespaceKey: owner,
					resources.ChildProtectedKey: "false",
				}
				data, err := json.Marshal(payload)
				require.NoError(t, err)
				var model kkComps.AIGatewayModel
				require.NoError(t, json.Unmarshal(data, &model))
				api := &providerOrderModelAPI{testAIGatewayModelAPI: testAIGatewayModelAPI{models: []kkComps.AIGatewayModel{model}}}
				p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayModelAPI: api}), slog.Default())
				p.resourceCache = newPlanningResourceCache()
				plan := &Plan{Changes: []PlannedChange{{
					ID:           "delete-old",
					Action:       ActionDelete,
					ResourceType: ResourceTypeAIGatewayProvider,
					ResourceID:   "old-id",
					Namespace:    "platform",
					Parent: &ParentInfo{
						Ref: "platform-alias",
						ID:  "gateway-id",
					},
					Fields: map[string]any{FieldName: "support-openai"},
				}}}
				if action != "" {
					detached := map[string]any{FieldTargets: []any{map[string]any{FieldProvider: "replacement"}}}
					plan.Changes = append(plan.Changes,
						PlannedChange{
							ID:           "model-change",
							Action:       action,
							ResourceType: ResourceTypeAIGatewayModel,
							ResourceID:   "model-id",
							Namespace:    "team-b",
							Parent: &ParentInfo{
								Ref: "application-alias",
								ID:  "gateway-id",
							},
							Fields: detached,
						})
					if action == ActionUpdate {
						plan.Changes = append(plan.Changes,
							PlannedChange{
								ID:           "create-replacement",
								Action:       ActionCreate,
								ResourceType: ResourceTypeAIGatewayProvider,
								Namespace:    "platform",
								Parent: &ParentInfo{
									Ref: "third-alias",
									ID:  "gateway-id",
								},
								Fields: map[string]any{FieldName: "replacement"},
							})
					}
				}
				err = p.resolveFinalAIGatewayDependencies(t.Context(), plan)
				if action == "" {
					require.ErrorContains(t, err, "still references it")
					return
				}
				require.NoError(t, err)
				require.Contains(t, plan.Changes[0].DependsOn, "model-change")
				if action == ActionUpdate {
					require.Contains(t, plan.Changes[1].DependsOn, "create-replacement")
				}
				data, err = json.Marshal(plan)
				require.NoError(t, err)
				var saved Plan
				require.NoError(t, json.Unmarshal(data, &saved))
				order, err := NewDependencyResolver().ResolveDependenciesWithGroups(saved.Changes)
				require.NoError(t, err)
				require.Less(t, indexOf(order.ExecutionOrder, "model-change"), indexOf(order.ExecutionOrder, "delete-old"))
				if action == ActionUpdate {
					require.Less(t, indexOf(order.ExecutionOrder, "create-replacement"), indexOf(order.ExecutionOrder, "model-change"))
				}
			})
		}
	}
}

func TestCanonicalGatewayIdentityDoesNotConflateAliases(t *testing.T) {
	change := PlannedChange{Parent: &ParentInfo{Ref: "same-alias", ID: "foreign-gateway"}}
	require.False(t, aiGatewayChangeMatchesGateway(change, "same-alias", "our-gateway"))
	require.True(t, aiGatewayChangeMatchesGateway(change, "other-alias", "foreign-gateway"))
}

func TestFinalPlanSameNameReplacementOrdersWholeLifecycle(t *testing.T) {
	payload, err := testAIGatewayModelResource(t).MutablePayloadMap()
	require.NoError(t, err)
	payload["id"] = "old-model-id"
	payload["created_at"], payload["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var model kkComps.AIGatewayModel
	require.NoError(t, json.Unmarshal(data, &model))
	api := &providerOrderModelAPI{testAIGatewayModelAPI: testAIGatewayModelAPI{models: []kkComps.AIGatewayModel{model}}}
	p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayModelAPI: api}), slog.Default())
	plan := &Plan{Changes: []PlannedChange{
		{
			ID:           "new-model",
			Action:       ActionCreate,
			ResourceType: ResourceTypeAIGatewayModel,
			Namespace:    "team-b",
			Parent: &ParentInfo{
				Ref: "alias-b",
				ID:  "gateway-id",
			},
			Fields: payload,
		},

		{
			ID:           "new-provider",
			Action:       ActionCreate,
			ResourceType: ResourceTypeAIGatewayProvider,
			Namespace:    "platform",
			Parent: &ParentInfo{
				Ref: "alias-a",
				ID:  "gateway-id",
			},
			Fields: map[string]any{FieldName: "support-openai"},
		},

		{
			ID:           "old-provider",
			Action:       ActionDelete,
			ResourceType: ResourceTypeAIGatewayProvider,
			ResourceID:   "old-provider-id",
			Namespace:    "platform",
			Parent: &ParentInfo{
				Ref: "alias-a",
				ID:  "gateway-id",
			},
			Fields: map[string]any{FieldName: "support-openai"},
		},

		{
			ID:           "old-model",
			Action:       ActionDelete,
			ResourceType: ResourceTypeAIGatewayModel,
			ResourceID:   "old-model-id",
			Namespace:    "team-a",
			Parent: &ParentInfo{
				Ref: "alias-b",
				ID:  "gateway-id",
			},
			Fields: payload,
		},
	}}
	require.NoError(t, p.resolveFinalAIGatewayDependencies(t.Context(), plan))
	order, err := NewDependencyResolver().ResolveDependenciesWithGroups(plan.Changes)
	require.NoError(t, err)
	require.Less(t, indexOf(order.ExecutionOrder, "old-model"), indexOf(order.ExecutionOrder, "old-provider"))
	require.Less(t, indexOf(order.ExecutionOrder, "old-provider"), indexOf(order.ExecutionOrder, "new-provider"))
	require.Less(t, indexOf(order.ExecutionOrder, "new-provider"), indexOf(order.ExecutionOrder, "new-model"))
	// Removing the old user deletion must fail even if a same-name target is
	// recreated: retained foreign/unowned users cannot be silently detached.
	plan.Changes = plan.Changes[:3]
	require.ErrorContains(t, p.resolveFinalAIGatewayDependencies(t.Context(), plan), "still references it")
}
