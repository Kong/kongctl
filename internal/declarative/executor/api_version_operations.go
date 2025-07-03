package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// createAPIVersion creates a new API version
func (e *Executor) createAPIVersion(ctx context.Context, change planner.PlannedChange) (string, error) {
	if e.client == nil {
		return "", fmt.Errorf("client not configured")
	}

	// Get parent API ID
	if change.Parent == nil {
		return "", fmt.Errorf("parent API reference required for API version creation")
	}

	// Get the parent API by ref
	parentAPI, err := e.client.GetAPIByName(ctx, change.Parent.Ref)
	if err != nil {
		return "", fmt.Errorf("failed to get parent API: %w", err)
	}
	if parentAPI == nil {
		return "", fmt.Errorf("parent API not found: %s", change.Parent.Ref)
	}

	// Build request
	req := kkComps.CreateAPIVersionRequest{}

	// Map fields to SDK request
	if version, ok := change.Fields["version"].(string); ok {
		req.Version = &version
	}
	// The SDK only supports Version and Spec fields
	if spec, ok := change.Fields["spec"].(map[string]interface{}); ok {
		if content, ok := spec["content"].(string); ok {
			req.Spec = &kkComps.CreateAPIVersionRequestSpec{
				Content: &content,
			}
		}
	}

	// Create the version
	resp, err := e.client.CreateAPIVersion(ctx, parentAPI.ID, req)
	if err != nil {
		return "", fmt.Errorf("failed to create API version: %w", err)
	}

	return resp.ID, nil
}

// Note: API versions don't support update or delete operations in the SDK