package executor

import (
	"github.com/kong/kongctl/internal/declarative/planner"
)

// ExecutionResult represents the outcome of executing a plan
type ExecutionResult struct {
	// Counts
	SuccessCount int `json:"success_count"`
	FailureCount int `json:"failure_count"`
	SkippedCount int `json:"skipped_count"`
	
	// Errors encountered during execution
	Errors []ExecutionError `json:"errors,omitempty"`
	
	// Indicates if this was a dry-run execution
	DryRun bool `json:"dry_run"`
	
	// Changes that were successfully applied (empty in dry-run)
	ChangesApplied []AppliedChange `json:"changes_applied,omitempty"`
	
	// Validation results for dry-run mode
	ValidationResults []ValidationResult `json:"validation_results,omitempty"`
}

// ExecutionError represents an error that occurred during execution
type ExecutionError struct {
	ChangeID     string `json:"change_id"`
	ResourceType string `json:"resource_type"`
	ResourceName string `json:"resource_name"`
	ResourceRef  string `json:"resource_ref"`
	Action       string `json:"action"`
	Error        string `json:"error"`
}

// AppliedChange represents a successfully applied change
type AppliedChange struct {
	ChangeID     string `json:"change_id"`
	ResourceType string `json:"resource_type"`
	ResourceName string `json:"resource_name"`
	ResourceRef  string `json:"resource_ref"`
	Action       string `json:"action"`
	ResourceID   string `json:"resource_id,omitempty"` // ID of created/updated resource
}

// ValidationResult represents the validation outcome for a change in dry-run mode
type ValidationResult struct {
	ChangeID     string `json:"change_id"`
	ResourceType string `json:"resource_type"`
	ResourceName string `json:"resource_name"`
	ResourceRef  string `json:"resource_ref"`
	Action       string `json:"action"`
	Status       string `json:"status"` // "would_succeed", "would_fail", "skipped"
	Validation   string `json:"validation,omitempty"` // "passed", "failed", reason
	Message      string `json:"message,omitempty"`
}

// ProgressReporter provides real-time feedback during plan execution
type ProgressReporter interface {
	// StartExecution is called at the beginning of plan execution
	StartExecution(plan *planner.Plan)
	
	// StartChange is called before executing a change
	StartChange(change planner.PlannedChange)
	
	// CompleteChange is called after a change is executed (success or failure)
	CompleteChange(change planner.PlannedChange, err error)
	
	// SkipChange is called when a change is skipped (e.g., in dry-run mode)
	SkipChange(change planner.PlannedChange, reason string)
	
	// FinishExecution is called at the end of plan execution
	FinishExecution(result *ExecutionResult)
}

// Message returns a user-friendly summary of the execution result
func (r *ExecutionResult) Message() string {
	if r.DryRun {
		if r.FailureCount > 0 {
			return "Dry-run complete with errors. No changes were made."
		}
		return "Dry-run complete. No changes were made."
	}
	
	if r.FailureCount > 0 {
		return "Execution completed with errors."
	}
	return "Execution completed successfully."
}

// HasErrors returns true if any errors occurred during execution
func (r *ExecutionResult) HasErrors() bool {
	return r.FailureCount > 0 || len(r.Errors) > 0
}

// TotalChanges returns the total number of changes processed
func (r *ExecutionResult) TotalChanges() int {
	return r.SuccessCount + r.FailureCount + r.SkippedCount
}