package executor

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
)

func TestConsoleReporter_StartExecution(t *testing.T) {
	tests := []struct {
		name         string
		plan         *planner.Plan
		dryRun       bool
		expectedOut  string
		shouldOutput bool
	}{
		{
			name: "plan with changes in apply mode",
			plan: &planner.Plan{
				Metadata: planner.PlanMetadata{
					Mode: planner.PlanModeApply,
				},
				Summary: planner.PlanSummary{
					TotalChanges: 3,
				},
			},
			dryRun:       false,
			expectedOut:  "Applying changes:\n",
			shouldOutput: true,
		},
		{
			name: "plan with changes in apply mode with dry-run",
			plan: &planner.Plan{
				Metadata: planner.PlanMetadata{
					Mode: planner.PlanModeApply,
				},
				Summary: planner.PlanSummary{
					TotalChanges: 3,
				},
			},
			dryRun:       true,
			expectedOut:  "Validating changes:\n",
			shouldOutput: true,
		},
		{
			name: "plan with changes in sync mode",
			plan: &planner.Plan{
				Metadata: planner.PlanMetadata{
					Mode: planner.PlanModeSync,
				},
				Summary: planner.PlanSummary{
					TotalChanges: 2,
				},
			},
			expectedOut:  "Applying changes:\n",
			shouldOutput: true,
		},
		{
			name: "empty plan",
			plan: &planner.Plan{
				Summary: planner.PlanSummary{
					TotalChanges: 0,
				},
			},
			expectedOut:  "No changes to execute.\n",
			shouldOutput: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			reporter := NewConsoleReporterWithOptions(&buf, tt.dryRun)
			
			reporter.StartExecution(tt.plan)
			
			output := buf.String()
			if tt.shouldOutput {
				assert.Equal(t, tt.expectedOut, output)
			} else {
				assert.Empty(t, output)
			}
		})
	}
}

func TestConsoleReporter_StartChange(t *testing.T) {
	tests := []struct {
		name        string
		change      planner.PlannedChange
		expectedOut string
	}{
		{
			name: "create with resource ref",
			change: planner.PlannedChange{
				Action:       planner.ActionCreate,
				ResourceType: "portal",
				ResourceRef:  "developer-portal",
			},
			expectedOut: "• Creating portal: developer-portal... ",
		},
		{
			name: "update with resource ref",
			change: planner.PlannedChange{
				Action:       planner.ActionUpdate,
				ResourceType: "portal",
				ResourceRef:  "staging-portal",
			},
			expectedOut: "• Updating portal: staging-portal... ",
		},
		{
			name: "delete without resource ref",
			change: planner.PlannedChange{
				ID:           "change-123",
				Action:       planner.ActionDelete,
				ResourceType: "portal_page",
			},
			expectedOut: "• Deleting portal_page: portal_page/change-123... ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			reporter := NewConsoleReporter(&buf)
			
			reporter.StartChange(tt.change)
			
			assert.Equal(t, tt.expectedOut, buf.String())
		})
	}
}

func TestConsoleReporter_CompleteChange(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expectedOut string
	}{
		{
			name:        "success",
			err:         nil,
			expectedOut: "✓\n",
		},
		{
			name:        "failure",
			err:         errors.New("resource not found"),
			expectedOut: "✗ Error: resource not found\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			reporter := NewConsoleReporter(&buf)
			
			change := planner.PlannedChange{
				Action:       planner.ActionCreate,
				ResourceType: "portal",
				ResourceRef:  "test-portal",
			}
			
			reporter.CompleteChange(change, tt.err)
			
			assert.Equal(t, tt.expectedOut, buf.String())
		})
	}
}

func TestConsoleReporter_SkipChange(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewConsoleReporter(&buf)
	
	change := planner.PlannedChange{
		Action:       planner.ActionCreate,
		ResourceType: "portal",
		ResourceRef:  "test-portal",
	}
	
	reporter.SkipChange(change, "dry-run mode")
	
	assert.Equal(t, "⚠ Skipped: dry-run mode\n", buf.String())
}

func TestConsoleReporter_FinishExecution_Normal(t *testing.T) {
	tests := []struct {
		name        string
		result      *ExecutionResult
		containsStr []string
		notContains []string
	}{
		{
			name: "successful execution",
			result: &ExecutionResult{
				SuccessCount: 3,
				FailureCount: 0,
				SkippedCount: 0,
			},
			containsStr: []string{
				"Complete.",
				"Applied 3 changes.",
			},
			notContains: []string{
				"- Failed:",
				"- Skipped:",
				"Errors:",
			},
		},
		{
			name: "execution with failures",
			result: &ExecutionResult{
				SuccessCount: 2,
				FailureCount: 1,
				SkippedCount: 0,
				Errors: []ExecutionError{
					{
						Action:       "CREATE",
						ResourceType: "portal",
						ResourceName: "bad-portal",
						Error:        "validation failed",
					},
				},
			},
			containsStr: []string{
				"Complete.",
				"Applied 2 changes.",
				"Errors:",
				"  • portal bad-portal: validation failed",
			},
		},
		{
			name: "execution with skipped",
			result: &ExecutionResult{
				SuccessCount: 1,
				FailureCount: 0,
				SkippedCount: 2,
			},
			containsStr: []string{
				"Complete.",
				"Applied 1 changes.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			reporter := NewConsoleReporter(&buf)
			
			reporter.FinishExecution(tt.result)
			
			output := buf.String()
			for _, expected := range tt.containsStr {
				assert.Contains(t, output, expected)
			}
			for _, notExpected := range tt.notContains {
				assert.NotContains(t, output, notExpected)
			}
		})
	}
}

func TestConsoleReporter_FinishExecution_DryRun(t *testing.T) {
	tests := []struct {
		name        string
		result      *ExecutionResult
		containsStr []string
	}{
		{
			name: "successful dry-run",
			result: &ExecutionResult{
				DryRun:       true,
				SkippedCount: 3,
				ValidationResults: []ValidationResult{
					{Status: "would_succeed"},
					{Status: "would_succeed"},
					{Status: "would_succeed"},
				},
			},
			containsStr: []string{
				"Dry run complete.",
				"3 changes would be applied.",
			},
		},
		{
			name: "dry-run with validation failures",
			result: &ExecutionResult{
				DryRun:       true,
				FailureCount: 2,
				SkippedCount: 3,
				ValidationResults: []ValidationResult{
					{Status: "would_succeed"},
					{Status: "would_fail"},
					{Status: "would_fail"},
				},
				Errors: []ExecutionError{
					{
						ResourceType: "portal",
						ResourceName: "invalid-portal",
						Error:        "name too long",
					},
					{
						ResourceType: "portal_page",
						ResourceName: "invalid-page",
						Error:        "missing required field",
					},
				},
			},
			containsStr: []string{
				"Dry run complete.",
				"3 changes would be applied.",
				"Validation errors:",
				"  • portal invalid-portal: name too long",
				"  • portal_page invalid-page: missing required field",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			reporter := NewConsoleReporter(&buf)
			
			reporter.FinishExecution(tt.result)
			
			output := buf.String()
			for _, expected := range tt.containsStr {
				assert.Contains(t, output, expected)
			}
		})
	}
}

func TestConsoleReporter_CompleteWorkflow(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewConsoleReporter(&buf)
	
	// Simulate a complete execution workflow
	plan := &planner.Plan{
		Metadata: planner.PlanMetadata{
			Mode:        planner.PlanModeSync,
			GeneratedAt: time.Now(),
		},
		Summary: planner.PlanSummary{
			TotalChanges: 3,
		},
	}
	
	reporter.StartExecution(plan)
	
	// First change: successful create
	change1 := planner.PlannedChange{
		Action:       planner.ActionCreate,
		ResourceType: "portal",
		ResourceRef:  "developer-portal",
	}
	reporter.StartChange(change1)
	reporter.CompleteChange(change1, nil)
	
	// Second change: successful update
	change2 := planner.PlannedChange{
		Action:       planner.ActionUpdate,
		ResourceType: "portal",
		ResourceRef:  "staging-portal",
	}
	reporter.StartChange(change2)
	reporter.CompleteChange(change2, nil)
	
	// Third change: failed delete
	change3 := planner.PlannedChange{
		Action:       planner.ActionDelete,
		ResourceType: "portal_page",
		ResourceRef:  "old-docs",
	}
	reporter.StartChange(change3)
	reporter.CompleteChange(change3, errors.New("not found"))
	
	// Finish execution
	result := &ExecutionResult{
		SuccessCount: 2,
		FailureCount: 1,
		Errors: []ExecutionError{
			{
				Action:       "DELETE",
				ResourceType: "portal_page",
				ResourceName: "old-docs",
				Error:        "not found",
			},
		},
	}
	reporter.FinishExecution(result)
	
	output := buf.String()
	
	// Verify the complete output
	assert.Contains(t, output, "Applying changes:")
	assert.Contains(t, output, "[1/3] Creating portal: developer-portal... ✓")
	assert.Contains(t, output, "[2/3] Updating portal: staging-portal... ✓")
	assert.Contains(t, output, "[3/3] Deleting portal_page: old-docs... ✗ Error: not found")
	assert.Contains(t, output, "Complete.")
	assert.Contains(t, output, "Applied 2 changes.")
	assert.Contains(t, output, "Errors:")
	assert.Contains(t, output, "  • portal_page old-docs: not found")
}

func TestGetActionVerb(t *testing.T) {
	tests := []struct {
		action   planner.ActionType
		expected string
	}{
		{planner.ActionCreate, "Creating"},
		{planner.ActionUpdate, "Updating"},
		{planner.ActionDelete, "Deleting"},
		{planner.ActionType("UNKNOWN"), "UNKNOWNing"},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			result := getActionVerb(tt.action)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConsoleReporter_InterfaceCompliance(t *testing.T) {
	// This test ensures ConsoleReporter implements ProgressReporter
	var reporter ProgressReporter = NewConsoleReporter(&bytes.Buffer{})
	assert.NotNil(t, reporter)
}

func TestConsoleReporter_NilWriter(t *testing.T) {
	// Test that operations don't panic with nil writer
	reporter := &ConsoleReporter{writer: nil}
	
	plan := &planner.Plan{
		Summary: planner.PlanSummary{TotalChanges: 1},
	}
	change := planner.PlannedChange{
		Action:       planner.ActionCreate,
		ResourceType: "portal",
		ResourceRef:  "test",
	}
	result := &ExecutionResult{
		SuccessCount: 1,
	}
	
	// These should not panic
	assert.NotPanics(t, func() {
		reporter.StartExecution(plan)
		reporter.StartChange(change)
		reporter.CompleteChange(change, nil)
		reporter.SkipChange(change, "test")
		reporter.FinishExecution(result)
	})
}

// TestConsoleReporter_MultilineOutput ensures proper formatting
func TestConsoleReporter_MultilineOutput(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewConsoleReporter(&buf)
	
	// Create a dry-run result with mixed outcomes
	result := &ExecutionResult{
		DryRun:       true,
		FailureCount: 1,
		SkippedCount: 5,
		ValidationResults: []ValidationResult{
			{Status: "would_succeed"},
			{Status: "would_succeed"},
			{Status: "would_succeed"},
			{Status: "would_fail"},
			{Status: "would_succeed"},
		},
		Errors: []ExecutionError{
			{
				ResourceType: "portal",
				ResourceName: "test-portal",
				Error:        "validation error: name contains invalid characters",
			},
		},
	}
	
	reporter.FinishExecution(result)
	
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	
	// Verify structure
	assert.Equal(t, "Dry run complete.", lines[0])
	assert.Contains(t, lines[1], "5 changes would be applied.")  // Message about changes
	
	// Find validation errors section
	foundErrors := false
	for i, line := range lines {
		if line == "Validation errors:" {
			foundErrors = true
			assert.True(t, i+1 < len(lines))
			assert.True(t, strings.HasPrefix(lines[i+1], "  • "))  // Indented error
			break
		}
	}
	assert.True(t, foundErrors, "Should find validation errors section")
}