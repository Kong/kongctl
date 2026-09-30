package planner

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAIGatewayDeletePoliciesSkipUnneededObservation(t *testing.T) {
	// A nil client catches any attempt to read remote users. Creation dependencies
	// still run for custom policies without requiring observations.
	p := NewPlanner(nil, slog.Default())
	for _, adapter := range aiGatewayDeleteResolvers {
		t.Run(adapter.targetType, func(t *testing.T) {
			plan := &Plan{Changes: []PlannedChange{{
				ID: "other-delete", ResourceType: adapter.targetType, Action: ActionDelete,
				Namespace: "other", Parent: &ParentInfo{Ref: "gateway"},
			}, {
				ID: "other-gateway-delete", ResourceType: adapter.targetType, Action: ActionDelete,
				Namespace: "default", Parent: &ParentInfo{Ref: "other-gateway"},
			}}}
			require.NoError(t, adapter.resolve(p, t.Context(), "default", "gateway", "gateway-id", plan))
		})
	}
}

func TestObservedReferenceDeletesPersistMultipleUsers(t *testing.T) {
	plan := &Plan{Changes: []PlannedChange{
		{
			ID: "target", ResourceType: ResourceTypeAIGatewayPolicy, Action: ActionDelete,
			Namespace: "default", Parent: &ParentInfo{Ref: "gateway"}, Fields: map[string]any{FieldName: "old"},
		},
		{
			ID: "agent", ResourceType: ResourceTypeAIGatewayAgent, ResourceID: "shared-id", Action: ActionDelete,
			Namespace: "default", Parent: &ParentInfo{Ref: "gateway"},
		},
		{
			ID: "model", ResourceType: ResourceTypeAIGatewayModel, ResourceID: "shared-id", Action: ActionUpdate,
			Namespace: "default", Parent: &ParentInfo{Ref: "gateway"}, Fields: map[string]any{FieldPolicies: []string{}},
		},
	}}
	policy := observedReferenceDeletePolicy{
		targetType: ResourceTypeAIGatewayPolicy, usesTarget: aiGatewayResourceUsesPolicies,
		observe: func(context.Context) ([]observedReferenceUser, error) {
			return []observedReferenceUser{
				{ResourceTypeAIGatewayAgent, "shared-id", "agent", []string{"old"}},
				{ResourceTypeAIGatewayModel, "shared-id", "model", []string{"old"}},
			}, nil
		},
		observedReferences: func(deletion PlannedChange, values []string) bool {
			return slices.Contains(values, deletion.Fields[FieldName].(string))
		},
		plannedReferences: func(deletion, change PlannedChange) bool {
			return slices.Contains(
				stringSliceFromField(change.Fields, FieldPolicies),
				deletion.Fields[FieldName].(string),
			)
		},
		updateDetaches: func(deletion, change PlannedChange) bool {
			return aiGatewayPolicyUpdateDetaches(change, func(values []string) bool {
				return slices.Contains(values, deletion.Fields[FieldName].(string))
			})
		},
		conflict: func(PlannedChange, observedReferenceUser, bool) error { return errors.New("retained user") },
	}
	for range 2 {
		require.NoError(t, resolveObservedReferenceDeletes(t.Context(), "default", "gateway", plan, policy))
	}
	require.ElementsMatch(t, []string{"agent", "model"}, plan.Changes[0].DependsOn)
	data, err := json.Marshal(plan)
	require.NoError(t, err)
	var saved Plan
	require.NoError(t, json.Unmarshal(data, &saved))
	first, err := NewDependencyResolver().ResolveDependenciesWithGroups(saved.Changes)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"agent", "model"}, first.FullDepsMap["target"])
	for i := range saved.Changes {
		saved.Changes[i].DependsOn = first.FullDepsMap[saved.Changes[i].ID]
	}
	again, err := NewDependencyResolver().ResolveDependenciesWithGroups(saved.Changes)
	require.NoError(t, err)
	require.Equal(t, first, again)

	plan.Changes[2].Fields = map[string]any{FieldDisplayName: "unrelated"}
	require.ErrorContains(
		t,
		resolveObservedReferenceDeletes(t.Context(), "default", "gateway", plan, policy),
		"retained user",
	)
	observationError := errors.New("observation unavailable")
	policy.observe = func(context.Context) ([]observedReferenceUser, error) { return nil, observationError }
	require.ErrorIs(
		t,
		resolveObservedReferenceDeletes(t.Context(), "default", "gateway", plan, policy),
		observationError,
	)
}
