package planner

// Internal field names used for communication between planner and executor
// These fields are prefixed with underscore to indicate they are internal
// and should not be confused with actual resource fields
const (
	// FieldCurrentLabels contains the current labels of a resource
	// Used during updates to determine which labels should be removed
	FieldCurrentLabels = "_current_labels"

	// FieldStrategyType contains the current strategy type for auth strategies
	// Used during updates since strategy type cannot be changed
	FieldStrategyType = "_strategy_type"

	// FieldError contains validation errors that should be reported
	// Used when the planner detects an invalid operation
	FieldError = "_error"
)