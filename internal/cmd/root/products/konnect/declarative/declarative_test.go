package declarative

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
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
		name        string
		mode        string
		allowDelete bool
		expected    planner.PlanMode
		errMsg      string
	}{
		{
			name:        "sync allowed for diff",
			mode:        "sync",
			allowDelete: false,
			expected:    planner.PlanModeSync,
		},
		{
			name:        "apply allowed for diff",
			mode:        "apply",
			allowDelete: false,
			expected:    planner.PlanModeApply,
		},
		{
			name:        "delete rejected for diff",
			mode:        "delete",
			allowDelete: false,
			errMsg:      `invalid mode "delete": must be 'sync' or 'apply'`,
		},
		{
			name:        "delete allowed for plan",
			mode:        "delete",
			allowDelete: true,
			expected:    planner.PlanModeDelete,
		},
		{
			name:        "invalid rejected for plan",
			mode:        "invalid",
			allowDelete: true,
			errMsg:      `invalid mode "invalid": must be 'sync', 'apply', or 'delete'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, err := parsePlanMode(tt.mode, tt.allowDelete)
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
