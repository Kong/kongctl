package common

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				"RESOURCE CHANGES",
				"SUMMARY",
				"Namespace: default",
				"<configuration changes detected>",
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
				"RESOURCE CHANGES",
				"SUMMARY",
				"Namespace: default",
				"+ p1",
				"+ p2", 
				"+ p3",
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
				"No changes detected. Configuration matches current state.",
			},
		},
		{
			name: "plan with duplicate dependencies deduplicated",
			plan: &planner.Plan{
				Summary: planner.PlanSummary{
					ByAction: map[planner.ActionType]int{
						planner.ActionCreate: 2,
					},
					TotalChanges: 2,
				},
				Changes: []planner.PlannedChange{
					{
						ID:           "1:c:api:test-api",
						Action:       planner.ActionCreate,
						ResourceType: "api",
						ResourceRef:  "test-api",
					},
					{
						ID:           "2:c:api_document:test-doc",
						Action:       planner.ActionCreate,
						ResourceType: "api_document",
						ResourceRef:  "test-doc",
						Parent:       &planner.ParentInfo{Ref: "test-api"},
						DependsOn:    []string{"1:c:api:test-api"},
					},
				},
			},
			expected: []string{
				"RESOURCE CHANGES",
				"SUMMARY",
				"Namespace: default",
				"+ test-api",
				"+ test-doc",
				"depends on: api:test-api",
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

func TestDisplayPlanSummary_WithResourceMonikers(t *testing.T) {
	plan := &planner.Plan{
		Changes: []planner.PlannedChange{
			{
				ID:           "1:d:portal_page:page1",
				ResourceType: "portal_page",
				ResourceRef:  "[unknown]",
				ResourceMonikers: map[string]string{
					"slug":         "getting-started",
					"parent_portal": "simple",
				},
				Action: planner.ActionDelete,
				Parent: &planner.ParentInfo{
					Ref: "simple",
					ID:  "portal-123",
				},
			},
			{
				ID:           "2:d:portal_page:page2",
				ResourceType: "portal_page",
				ResourceRef:  "[unknown]",
				ResourceMonikers: map[string]string{
					"slug":         "api-guide",
					"parent_portal": "simple",
				},
				Action: planner.ActionDelete,
				Parent: &planner.ParentInfo{
					Ref: "simple",
					ID:  "portal-123",
				},
			},
		},
		Summary: planner.PlanSummary{
			TotalChanges: 2,
			ByAction: map[planner.ActionType]int{
				planner.ActionDelete: 2,
			},
			ByResource: map[string]int{
				"portal_page": 2,
			},
		},
	}

	var out bytes.Buffer
	DisplayPlanSummary(plan, &out)

	output := out.String()
	t.Log("Plan Summary Output:\n", output)

	// Check that monikers are properly displayed
	assert.Contains(t, output, "page 'getting-started' in portal:simple")
	assert.Contains(t, output, "page 'api-guide' in portal:simple")
	assert.Contains(t, output, "depends on: portal:simple")
	
	// Should not contain [unknown]
	assert.NotContains(t, output, "[unknown]")
}

func TestConfirmExecution_WithResourceMonikers(t *testing.T) {
	plan := &planner.Plan{
		Changes: []planner.PlannedChange{
			{
				ID:           "1:d:portal_page:page1",
				ResourceType: "portal_page",
				ResourceRef:  "[unknown]",
				ResourceMonikers: map[string]string{
					"slug":         "getting-started",
					"parent_portal": "simple",
				},
				Action: planner.ActionDelete,
			},
		},
		Summary: planner.PlanSummary{
			TotalChanges: 1,
			ByAction: map[planner.ActionType]int{
				planner.ActionDelete: 1,
			},
			ByResource: map[string]int{
				"portal_page": 1,
			},
		},
	}

	var stderr bytes.Buffer
	stdin := strings.NewReader("no\n")
	
	result := ConfirmExecution(plan, nil, &stderr, stdin)
	require.False(t, result)

	output := stderr.String()
	t.Log("Confirmation Output:\n", output)

	// Check that the DELETE warning shows monikers
	assert.Contains(t, output, "WARNING: This operation will DELETE resources:")
	assert.Contains(t, output, "- portal_page: page 'getting-started' in portal:simple")
	
	// Should not contain [unknown]
	assert.NotContains(t, output, "[unknown]")
}

func TestFormatResourceName_AllResourceTypes(t *testing.T) {
	testCases := []struct {
		name     string
		change   planner.PlannedChange
		expected string
	}{
		{
			name: "portal_page with monikers",
			change: planner.PlannedChange{
				ResourceRef: "[unknown]",
				ResourceType: "portal_page",
				ResourceMonikers: map[string]string{
					"slug":         "getting-started",
					"parent_portal": "dev-portal",
				},
			},
			expected: "page 'getting-started' in portal:dev-portal",
		},
		{
			name: "portal_snippet with monikers",
			change: planner.PlannedChange{
				ResourceRef: "[unknown]",
				ResourceType: "portal_snippet",
				ResourceMonikers: map[string]string{
					"name":         "header-snippet",
					"parent_portal": "dev-portal",
				},
			},
			expected: "snippet 'header-snippet' in portal:dev-portal",
		},
		{
			name: "api_document with monikers",
			change: planner.PlannedChange{
				ResourceRef: "[unknown]",
				ResourceType: "api_document",
				ResourceMonikers: map[string]string{
					"slug":       "api-guide",
					"parent_api": "my-api",
				},
			},
			expected: "document 'api-guide' in api:my-api",
		},
		{
			name: "api_publication with monikers",
			change: planner.PlannedChange{
				ResourceRef: "[unknown]",
				ResourceType: "api_publication",
				ResourceMonikers: map[string]string{
					"portal_name": "dev-portal",
					"api_ref":     "my-api",
				},
			},
			expected: "api:my-api published to portal:dev-portal",
		},
		{
			name: "generic resource with monikers",
			change: planner.PlannedChange{
				ResourceRef: "[unknown]",
				ResourceType: "some_resource",
				ResourceMonikers: map[string]string{
					"name": "test",
					"type": "custom",
				},
			},
			expected: "name=test, type=custom",
		},
		{
			name: "normal resource ref",
			change: planner.PlannedChange{
				ResourceRef:  "my-resource",
				ResourceType: "api",
			},
			expected: "my-resource",
		},
		{
			name: "empty ref with name in fields",
			change: planner.PlannedChange{
				ResourceRef:  "",
				ResourceType: "api",
				Fields: map[string]interface{}{
					"name": "my-api",
				},
			},
			expected: "my-api",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatResourceName(tc.change)
			assert.Equal(t, tc.expected, result)
		})
	}
}