package declarative

import (
	"bytes"
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_validateDeletePlan(t *testing.T) {
	tests := []struct {
		name    string
		mode    planner.PlanMode
		wantErr bool
		errMsg  string
	}{
		{
			name:    "delete mode is accepted",
			mode:    planner.PlanModeDelete,
			wantErr: false,
		},
		{
			name:    "apply mode is rejected",
			mode:    planner.PlanModeApply,
			wantErr: true,
			errMsg:  `delete command requires a plan generated in delete mode, got "apply" mode`,
		},
		{
			name:    "sync mode is rejected",
			mode:    planner.PlanModeSync,
			wantErr: true,
			errMsg:  `delete command requires a plan generated in delete mode, got "sync" mode`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &planner.Plan{
				Metadata: planner.PlanMetadata{Mode: tt.mode},
			}
			err := validateDeletePlan(plan)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_parsePlanMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		expected planner.PlanMode
		errMsg   string
	}{
		{
			name:     "sync mode",
			mode:     "sync",
			expected: planner.PlanModeSync,
		},
		{
			name:     "apply mode",
			mode:     "apply",
			expected: planner.PlanModeApply,
		},
		{
			name:     "delete mode",
			mode:     "delete",
			expected: planner.PlanModeDelete,
		},
		{
			name:   "invalid mode",
			mode:   "invalid",
			errMsg: `invalid mode "invalid": must be 'sync', 'apply', or 'delete'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, err := parsePlanMode(tt.mode)
			if tt.errMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, mode)
		})
	}
}

func TestDisplayTextDiff_UsesChangedFieldsForUpdateOutput(t *testing.T) {
	plan := &planner.Plan{
		Changes: []planner.PlannedChange{
			{
				ID:           "1:u:event_gateway_listener:listener-a",
				ResourceType: planner.ResourceTypeEventGatewayListener,
				ResourceRef:  "listener-a",
				ResourceID:   "listener-id",
				Action:       planner.ActionUpdate,
				Namespace:    "default",
				Fields: map[string]any{
					"name":        "listener-a",
					"description": "new description",
					"addresses":   []string{"0.0.0.0"},
				},
				ChangedFields: map[string]planner.FieldChange{
					"description": {
						Old: "old description",
						New: "new description",
					},
				},
			},
		},
		ExecutionOrder: []string{"1:u:event_gateway_listener:listener-a"},
		Summary: planner.PlanSummary{
			TotalChanges: 1,
			ByAction: map[planner.ActionType]int{
				planner.ActionUpdate: 1,
			},
			ByResource: map[string]int{
				planner.ResourceTypeEventGatewayListener: 1,
			},
		},
	}

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := displayTextDiff(cmd, plan, false)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, `description: "old description" → "new description"`)
	assert.NotContains(t, output, "addresses:")
	assert.NotContains(t, output, `name: "listener-a"`)
}
