package executor

import (
	"fmt"
	"io"

	"github.com/kong/kongctl/internal/declarative/planner"
)

// ConsoleReporter provides console output for plan execution progress
type ConsoleReporter struct {
	writer io.Writer
	dryRun bool
}

// NewConsoleReporter creates a new console reporter that writes to the provided writer
func NewConsoleReporter(w io.Writer) *ConsoleReporter {
	return &ConsoleReporter{
		writer: w,
		dryRun: false,
	}
}

// NewConsoleReporterWithOptions creates a new console reporter with options
func NewConsoleReporterWithOptions(w io.Writer, dryRun bool) *ConsoleReporter {
	return &ConsoleReporter{
		writer: w,
		dryRun: dryRun,
	}
}

// StartExecution is called at the beginning of plan execution
func (r *ConsoleReporter) StartExecution(plan *planner.Plan) {
	if r.writer == nil {
		return
	}
	if plan.Summary.TotalChanges == 0 {
		fmt.Fprintln(r.writer, "No changes to execute.")
		return
	}

	// In dry-run mode, show a simple header
	if r.dryRun {
		fmt.Fprintln(r.writer, "Validating changes:")
	} else {
		fmt.Fprintln(r.writer, "Applying changes:")
	}
}

// StartChange is called before executing a change
func (r *ConsoleReporter) StartChange(change planner.PlannedChange) {
	if r.writer == nil {
		return
	}
	action := getActionVerb(change.Action)
	resourceName := change.ResourceRef
	if resourceName == "" {
		resourceName = fmt.Sprintf("%s/%s", change.ResourceType, change.ID)
	}
	
	fmt.Fprintf(r.writer, "- %s %s: %s... ", action, change.ResourceType, resourceName)
}

// CompleteChange is called after a change is executed (success or failure)
func (r *ConsoleReporter) CompleteChange(_ planner.PlannedChange, err error) {
	if r.writer == nil {
		return
	}
	if err != nil {
		fmt.Fprintf(r.writer, "✗ Error: %s\n", err.Error())
	} else {
		fmt.Fprintln(r.writer, "✓")
	}
}

// SkipChange is called when a change is skipped
func (r *ConsoleReporter) SkipChange(_ planner.PlannedChange, reason string) {
	if r.writer == nil {
		return
	}
	fmt.Fprintf(r.writer, "⚠ Skipped: %s\n", reason)
}

// FinishExecution is called at the end of plan execution
func (r *ConsoleReporter) FinishExecution(result *ExecutionResult) {
	if r.writer == nil {
		return
	}
	fmt.Fprintln(r.writer, "")
	
	if result.DryRun {
		// For dry-run, show what would happen
		fmt.Fprintln(r.writer, "\nDry run complete.")
		if result.SkippedCount > 0 {
			fmt.Fprintf(r.writer, "%d changes would be applied.\n", result.SkippedCount)
		}
		
		if result.FailureCount > 0 {
			fmt.Fprintln(r.writer, "\nValidation errors:")
			for _, err := range result.Errors {
				fmt.Fprintf(r.writer, "- %s %s: %s\n", err.ResourceType, err.ResourceName, err.Error)
			}
		}
	} else {
		// For actual execution, show results
		fmt.Fprintln(r.writer, "\nComplete.")
		if result.SuccessCount > 0 {
			fmt.Fprintf(r.writer, "Applied %d changes.\n", result.SuccessCount)
		}
		
		if result.FailureCount > 0 && len(result.Errors) > 0 {
			fmt.Fprintln(r.writer, "\nErrors:")
			for _, err := range result.Errors {
				fmt.Fprintf(r.writer, "- %s %s: %s\n", 
					err.ResourceType, err.ResourceName, err.Error)
			}
		}
	}
}

// getActionVerb converts an ActionType to a present-tense verb for display
func getActionVerb(action planner.ActionType) string {
	switch action {
	case planner.ActionCreate:
		return "Creating"
	case planner.ActionUpdate:
		return "Updating"
	case planner.ActionDelete:
		return "Deleting"
	default:
		return string(action) + "ing"
	}
}

// Ensure ConsoleReporter implements ProgressReporter
var _ ProgressReporter = (*ConsoleReporter)(nil)