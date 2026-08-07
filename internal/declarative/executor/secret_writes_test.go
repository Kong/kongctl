package executor

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretWritePreflightResolvesAllSourcesBeforeInjection(t *testing.T) {
	t.Setenv("FIRST_SECRET", "first")
	t.Setenv("SECOND_SECRET", "second")
	plan := secretExecutionPlan(
		secretExecutionIntent("/config/first", "FIRST_SECRET"),
		secretExecutionIntent("/config/second", "SECOND_SECRET"),
	)
	executor := &Executor{}

	require.NoError(t, executor.preflightSecretWrites(plan))
	change, err := cloneChangeForExecution(&plan.Changes[0])
	require.NoError(t, err)
	require.NoError(t, executor.injectResolvedSecretWrites(change))

	config := change.Fields[planner.FieldConfig].(map[string]any)
	assert.Equal(t, "first", config["first"])
	assert.Equal(t, "second", config["second"])
	assert.Empty(t, plan.Changes[0].Fields[planner.FieldConfig])
}

func TestSecretWritePreflightFailurePublishesNoPartialValues(t *testing.T) {
	t.Setenv("AVAILABLE_SECRET", "available")
	plan := secretExecutionPlan(
		secretExecutionIntent("/config/available", "AVAILABLE_SECRET"),
		secretExecutionIntent("/config/missing", "MISSING_SECRET"),
	)
	executor := &Executor{resolvedSecrets: map[string]map[string]string{"old": {"/value": "old"}}}

	err := executor.preflightSecretWrites(plan)
	require.ErrorContains(t, err, "MISSING_SECRET")
	assert.Empty(t, executor.resolvedSecrets)
}

func TestSecretWritePreflightRejectsInvalidTargetBeforeExecution(t *testing.T) {
	t.Setenv("SECRET", "value")
	plan := secretExecutionPlan(secretExecutionIntent("/config/headers/2/value", "SECRET"))
	plan.Changes[0].Fields[planner.FieldConfig] = map[string]any{
		"headers": []any{map[string]any{}},
	}

	err := (&Executor{}).preflightSecretWrites(plan)
	require.ErrorContains(t, err, "array index")
}

func TestSecretWritePreflightRejectsEmptySourceAndComposesPrivately(t *testing.T) {
	t.Setenv("EMPTY_SECRET", "")
	plan := secretExecutionPlan(secretExecutionIntent("/config/value", "EMPTY_SECRET"))
	executor := &Executor{}
	require.ErrorContains(t, executor.preflightSecretWrites(plan), "empty value")

	t.Setenv("TOKEN", "opaque-token")
	literal := "Bearer "
	plan.Changes[0].SecretWrites[0].Expression.Parts = []tags.SecretPart{
		{Literal: &literal},
		{Source: &tags.SecretSource{Kind: "env", Reference: "TOKEN"}},
	}
	require.NoError(t, executor.preflightSecretWrites(plan))
	assert.Equal(t, "Bearer opaque-token", executor.resolvedSecrets["change-1"]["/config/value"])
	assert.NotContains(t, plan.Changes[0].Fields, "opaque-token")
}

func secretExecutionPlan(intents ...planner.SecretWriteIntent) *planner.Plan {
	return &planner.Plan{
		Changes: []planner.PlannedChange{{
			ID: "change-1", Fields: map[string]any{planner.FieldConfig: map[string]any{}}, SecretWrites: intents,
		}},
	}
}

func secretExecutionIntent(field, reference string) planner.SecretWriteIntent {
	return planner.SecretWriteIntent{
		Field: field,
		Expression: tags.SecretExpression{Parts: []tags.SecretPart{{Source: &tags.SecretSource{
			Kind: "env", Reference: reference,
		}}}},
	}
}
