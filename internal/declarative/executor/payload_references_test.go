package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

func TestPayloadBindingsHydrateSavedPlanAndRetry(t *testing.T) {
	plan := planner.NewPlan(planner.CurrentPlanVersion, "test", planner.PlanModeApply)
	plan.Changes = []planner.PlannedChange{
		{
			ID: "gateway-create", ResourceType: planner.ResourceTypeAIGateway, ResourceRef: "gateway",
			Action: planner.ActionCreate,
		},
		{
			ID: "policy-create", ResourceType: planner.ResourceTypeAIGatewayPolicy, ResourceRef: "policy",
			Action: planner.ActionCreate, DependsOn: []string{"gateway-create"},
			Fields: map[string]any{planner.FieldConfig: map[string]any{
				"a.b":  map[string]any{"0": "__REF__:gateway#id"},
				"a":    map[string]any{"b": "literal"},
				"list": []any{map[string]any{"a/b~c": "__REF__:gateway#id"}},
			}},
			PayloadReferences: []planner.PayloadReference{
				{Path: "/config/a.b/0", ResourceType: planner.ResourceTypeAIGateway, Ref: "gateway", Selector: planner.FieldID},
				{
					Path: "/config/list/0/a~1b~0c", ResourceType: planner.ResourceTypeAIGateway,
					Ref: "gateway", Selector: planner.FieldID,
				},
			},
		},
	}
	data, err := json.Marshal(plan)
	require.NoError(t, err)
	var restored planner.Plan
	require.NoError(t, json.Unmarshal(data, &restored))
	e := New(nil, nil, false)
	e.createdResources["gateway-create"] = "created-gateway-id"
	change := &restored.Changes[1]
	for range 2 {
		require.NoError(t, e.hydratePayloadReferences(change, &restored))
		config := change.Fields[planner.FieldConfig].(map[string]any)
		require.Equal(t, "created-gateway-id", config["a.b"].(map[string]any)["0"])
		require.Equal(t, "literal", config["a"].(map[string]any)["b"])
		require.NoError(t, validateResolvedPayload(change.Fields))
	}
}

func TestPayloadGuardHonorsDeferredValueProvenance(t *testing.T) {
	change := &planner.PlannedChange{Fields: map[string]any{
		planner.FieldDescription: tags.EnvPlaceholderPrefix + "deferred",
	}, SecretWrites: []planner.SecretWriteIntent{{Field: "/config/value"}}}
	ctx := withDeferredPayloadPaths(context.Background(), change)
	require.NoError(t, validateResolvedRequest(ctx, map[string]any{
		planner.FieldDescription: "__REF__:ordinary-env-value#id",
		planner.FieldConfig:      map[string]any{"value": "__REF__:ordinary-secret-value#id"},
	}))
	require.Error(t, validateResolvedRequest(ctx, map[string]any{
		planner.FieldConfig: map[string]any{"header": "__REF__:unresolved#id"},
	}))
}

func TestPayloadGuardRejectsExpressionsWithoutDisclosingValues(t *testing.T) {
	for _, expression := range []string{"__REF__:private#id", "__EXTERNAL__:private-selector"} {
		err := validateResolvedPayload(map[string]any{"config": []any{map[string]any{"header.name": expression}}})
		require.ErrorContains(t, err, "/config/0/header.name")
		require.NotContains(t, err.Error(), "private")
	}
	require.NoError(t, validateResolvedPayload(map[string]any{
		"literal": "a reference to gateway#id", "env": "__ENV__:deferred", "__REF__:key#id": "ordinary value",
	}))
}
