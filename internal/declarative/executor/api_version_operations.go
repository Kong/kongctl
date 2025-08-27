package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
)

// createAPIVersion creates a new API version
// Deprecated: Use APIVersionAdapter with BaseCreateDeleteExecutor instead
//
//nolint:unused // kept for backward compatibility, will be removed in Phase 2 cleanup
func (e *Executor) createAPIVersion(ctx context.Context, change planner.PlannedChange) (string, error) {
	if e.client == nil {
		return "", fmt.Errorf("client not configured")
	}

	// Get parent API ID
	parentAPIID, err := e.getParentAPIID(ctx, change)
	if err != nil {
		return "", err
	}

	// Build request
	req := kkComps.CreateAPIVersionRequest{}

	// Map fields to SDK request
	if version, ok := change.Fields["version"].(string); ok {
		req.Version = &version
	}
	// The SDK only supports Version and Spec fields
	if spec, ok := change.Fields["spec"].(map[string]any); ok {
		if content, ok := spec["content"].(string); ok {
			req.Spec = &kkComps.CreateAPIVersionRequestSpec{
				Content: &content,
			}
		}
	}

	// Create the version
	resp, err := e.client.CreateAPIVersion(ctx, parentAPIID, req)
	if err != nil {
		return "", fmt.Errorf("failed to create API version: %w", err)
	}

	return resp.ID, nil
}

// deleteAPIVersion deletes an API version
// Deprecated: Use APIVersionAdapter with BaseCreateDeleteExecutor instead
//
//nolint:unused // kept for backward compatibility, will be removed in Phase 2 cleanup
func (e *Executor) deleteAPIVersion(ctx context.Context, change planner.PlannedChange) error {
	if e.client == nil {
		return fmt.Errorf("client not configured")
	}

	// Get parent API ID
	parentAPIID, err := e.getParentAPIID(ctx, change)
	if err != nil {
		return err
	}

	// Delete the version
	if err := e.client.DeleteAPIVersion(ctx, parentAPIID, change.ResourceID); err != nil {
		return fmt.Errorf("failed to delete API version: %w", err)
	}

	return nil
}

// Note: API versions don't support update operations in the SDK
