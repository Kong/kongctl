package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
)

// createAPIImplementation creates a new API implementation
// Deprecated: Use APIImplementationAdapter with BaseExecutor instead
//
//nolint:unused // kept for backward compatibility, will be removed in Phase 2 cleanup
func (e *Executor) createAPIImplementation(_ context.Context, _ planner.PlannedChange) (string, error) {
	// API implementation creation is not yet supported by the SDK
	return "", fmt.Errorf("API implementation creation not yet supported by SDK")
}

// deleteAPIImplementation deletes an API implementation
// Deprecated: Use APIImplementationAdapter with BaseExecutor instead
//
//nolint:unused // kept for backward compatibility, will be removed in Phase 2 cleanup
func (e *Executor) deleteAPIImplementation(_ context.Context, _ planner.PlannedChange) error {
	// API implementation deletion is not yet supported by the SDK
	return fmt.Errorf("API implementation deletion not yet supported by SDK")
}

// Note: API implementations don't support update operations in the SDK
