package common

import (
	"encoding/json"
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactPlanForDisplayPreservesExecutionValues(t *testing.T) {
	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
	config := map[string]any{"redis": map[string]any{
		"password": "literal-test-value", "port": 6380, "token_count": 17,
	}}
	plan.AddChange(planner.PlannedChange{
		ID: "update-policy", Action: planner.ActionUpdate,
		ResourceType: planner.ResourceTypeAIGatewayPolicy, ResourceRef: "policy",
		Fields: map[string]any{planner.FieldConfig: config},
		ChangedFields: map[string]planner.FieldChange{planner.FieldConfig: {
			Old: map[string]any{"redis": map[string]any{"password": "remote-test-value", "port": 6379}},
			New: config,
		}},
	})
	before, err := json.Marshal(plan)
	require.NoError(t, err)
	view := RedactPlanForDisplay(plan)
	output, err := json.Marshal(view)
	require.NoError(t, err)
	assert.NotContains(t, string(output), "literal-test-value")
	assert.NotContains(t, string(output), "remote-test-value")
	assert.Contains(t, string(output), "[REDACTED]")
	assert.Contains(t, string(output), `"port":6380`)
	assert.Contains(t, string(output), `"token_count":17`)
	after, err := json.Marshal(plan)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}
