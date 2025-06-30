package common

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
)

func TestConfirmExecution(t *testing.T) {
	tests := []struct {
		name      string
		plan      *planner.Plan
		input     string
		expected  bool
		checkStdout func(t *testing.T, stdout string)
		checkStderr func(t *testing.T, stderr string)
	}{
		{
			name: "user confirms with yes",
			plan: &planner.Plan{
				Summary: planner.PlanSummary{
					ByAction: map[planner.ActionType]int{
						planner.ActionCreate: 2,
						planner.ActionUpdate: 1,
					},
				},
				Changes: []planner.PlannedChange{
					{
						Action:       planner.ActionCreate,
						ResourceType: "portal",
						ResourceRef:  "new-portal-1",
					},
					{
						Action:       planner.ActionCreate,
						ResourceType: "portal",
						ResourceRef:  "new-portal-2",
					},
					{
						Action:       planner.ActionUpdate,
						ResourceType: "api",
						ResourceRef:  "existing-api",
					},
				},
			},
			input:    "yes\n",
			expected: true,
			checkStdout: func(t *testing.T, stdout string) {
				// ConfirmExecution doesn't call DisplayPlanSummary,
				// so stdout should be empty
				assert.Empty(t, stdout)
			},
			checkStderr: func(t *testing.T, stderr string) {
				assert.Contains(t, stderr, "Do you want to continue? Type 'yes' to confirm:")
			},
		},
		{
			name: "user denies with no",
			plan: &planner.Plan{
				Summary: planner.PlanSummary{
					ByAction: map[planner.ActionType]int{
						planner.ActionCreate: 1,
					},
				},
			},
			input:    "no\n",
			expected: false,
		},
		{
			name: "user denies with empty input",
			plan: &planner.Plan{
				Summary: planner.PlanSummary{
					ByAction: map[planner.ActionType]int{
						planner.ActionUpdate: 1,
					},
				},
			},
			input:    "\n",
			expected: false,
		},
		{
			name: "plan with delete operations shows warning",
			plan: &planner.Plan{
				Summary: planner.PlanSummary{
					ByAction: map[planner.ActionType]int{
						planner.ActionCreate: 1,
						planner.ActionDelete: 2,
					},
				},
				Changes: []planner.PlannedChange{
					{
						Action:       planner.ActionDelete,
						ResourceType: "portal",
						ResourceRef:  "old-portal",
					},
					{
						Action:       planner.ActionDelete,
						ResourceType: "api",
						ResourceRef:  "deprecated-api",
					},
				},
			},
			input:    "yes\n",
			expected: true,
			checkStderr: func(t *testing.T, stderr string) {
				assert.Contains(t, stderr, "WARNING: This operation will DELETE resources:")
				assert.Contains(t, stderr, "- portal: old-portal")
				assert.Contains(t, stderr, "- api: deprecated-api")
			},
		},
		{
			name: "plan with warnings",
			plan: &planner.Plan{
				Summary: planner.PlanSummary{
					ByAction: map[planner.ActionType]int{
						planner.ActionCreate: 1,
					},
				},
				Changes: []planner.PlannedChange{
					{
						Action:       planner.ActionCreate,
						ResourceType: "portal",
						ResourceRef:  "portal-with-warning",
					},
				},
				Warnings: []planner.PlanWarning{
					{Message: "Resource foo has unresolved references"},
					{Message: "Resource bar may be protected"},
				},
			},
			input:    "yes\n",
			expected: true,
			checkStdout: func(t *testing.T, stdout string) {
				// ConfirmExecution doesn't call DisplayPlanSummary,
				// so stdout should be empty
				assert.Empty(t, stdout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			stdin := strings.NewReader(tt.input)

			result := ConfirmExecution(tt.plan, stdout, stderr, stdin)
			assert.Equal(t, tt.expected, result)

			if tt.checkStdout != nil {
				tt.checkStdout(t, stdout.String())
			}
			if tt.checkStderr != nil {
				tt.checkStderr(t, stderr.String())
			}
		})
	}
}

func TestDisplayPlanSummary(t *testing.T) {
	tests := []struct {
		name     string
		plan     *planner.Plan
		expected []string
	}{
		{
			name: "plan with all action types",
			plan: &planner.Plan{
				Summary: planner.PlanSummary{
					ByAction: map[planner.ActionType]int{
						planner.ActionCreate: 3,
						planner.ActionUpdate: 2,
						planner.ActionDelete: 1,
					},
				},
				Changes: []planner.PlannedChange{
					{Action: planner.ActionCreate, ResourceType: "portal", ResourceRef: "p1"},
					{Action: planner.ActionCreate, ResourceType: "portal", ResourceRef: "p2"},
					{Action: planner.ActionCreate, ResourceType: "api", ResourceRef: "a1"},
					{Action: planner.ActionUpdate, ResourceType: "portal", ResourceRef: "p3"},
					{Action: planner.ActionUpdate, ResourceType: "api", ResourceRef: "a2"},
					{Action: planner.ActionDelete, ResourceType: "portal", ResourceRef: "p4"},
				},
			},
			expected: []string{
				"Plan Summary:",
				"portal (4):",
				"+ p1",
				"+ p2",
				"~ p3",
				"- p4",
				"api (2):",
				"+ a1",
				"~ a2",
			},
		},
		{
			name: "plan with only creates",
			plan: &planner.Plan{
				Summary: planner.PlanSummary{
					ByAction: map[planner.ActionType]int{
						planner.ActionCreate: 5,
					},
				},
				Changes: []planner.PlannedChange{
					{Action: planner.ActionCreate, ResourceType: "portal", ResourceRef: "p1"},
					{Action: planner.ActionCreate, ResourceType: "portal", ResourceRef: "p2"},
					{Action: planner.ActionCreate, ResourceType: "portal", ResourceRef: "p3"},
					{Action: planner.ActionCreate, ResourceType: "api", ResourceRef: "a1"},
					{Action: planner.ActionCreate, ResourceType: "api", ResourceRef: "a2"},
				},
			},
			expected: []string{
				"Plan Summary:",
				"portal (3):",
				"+ p1",
				"+ p2",
				"+ p3",
				"api (2):",
				"+ a1",
				"+ a2",
			},
		},
		{
			name: "empty plan",
			plan: &planner.Plan{
				Summary: planner.PlanSummary{},
			},
			expected: []string{
				"Plan Summary:",
				"No changes",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			DisplayPlanSummary(tt.plan, out)
			output := out.String()

			for _, exp := range tt.expected {
				assert.Contains(t, output, exp)
			}
		})
	}
}