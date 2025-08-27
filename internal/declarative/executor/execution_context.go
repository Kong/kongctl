package executor

import "github.com/kong/kongctl/internal/declarative/planner"

// ExecutionContext carries execution state that was previously stored in context.
// This struct eliminates the need for context.WithValue and unsafe type assertions
// by making dependencies explicit in function signatures.
type ExecutionContext struct {
	// Namespace is the kongctl namespace for resource labeling
	Namespace string

	// Protection contains protection-related metadata for resource labeling
	Protection any

	// PlannedChange contains the full planned change being executed,
	// including references and field changes
	PlannedChange *planner.PlannedChange
}

// NewExecutionContext creates a new ExecutionContext from a PlannedChange
func NewExecutionContext(change *planner.PlannedChange) *ExecutionContext {
	return &ExecutionContext{
		Namespace:     change.Namespace,
		Protection:    change.Protection,
		PlannedChange: change,
	}
}
