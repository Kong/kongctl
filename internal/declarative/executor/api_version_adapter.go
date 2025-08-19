package executor

import (
	"context"
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// APIVersionAdapter implements CreateDeleteOperations for API versions
// API versions only support create and delete operations, not updates
type APIVersionAdapter struct {
	client  *state.Client
	execCtx *ExecutionContext // Store execution context for helper methods
}

// NewAPIVersionAdapter creates a new API version adapter
func NewAPIVersionAdapter(client *state.Client) *APIVersionAdapter {
	return &APIVersionAdapter{client: client}
}

// MapCreateFields maps fields to CreateAPIVersionRequest
func (a *APIVersionAdapter) MapCreateFields(_ context.Context, execCtx *ExecutionContext, fields map[string]any,
	create *kkComps.CreateAPIVersionRequest) error {
	// Store execution context for use in helper methods
	a.execCtx = execCtx
	
	// Version field
	if version, ok := fields["version"].(string); ok {
		create.Version = &version
	}
	
	// Spec field (optional)
	if spec, ok := fields["spec"].(map[string]any); ok {
		if content, ok := spec["content"].(string); ok {
			create.Spec = &kkComps.CreateAPIVersionRequestSpec{
				Content: &content,
			}
		}
	}

	return nil
}

// Create creates a new API version
func (a *APIVersionAdapter) Create(ctx context.Context, req kkComps.CreateAPIVersionRequest,
	_ string) (string, error) {
	// Get API ID from context
	apiID, err := a.getAPIID(ctx)
	if err != nil {
		return "", err
	}

	resp, err := a.client.CreateAPIVersion(ctx, apiID, req)
	if err != nil {
		// Enhance error message for Konnect's single version constraint
		if strings.Contains(err.Error(), "At most one api specification") {
			return "", fmt.Errorf("failed to create API version: Konnect allows only one version per API. "+
				"Consider updating the existing version or creating a separate API. Original error: %w", err)
		}
		return "", fmt.Errorf("failed to create API version: %w", err)
	}
	if resp == nil {
		return "", fmt.Errorf("API version creation returned no response")
	}
	return resp.ID, nil
}

// Delete deletes an API version
func (a *APIVersionAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	// Get API ID from execution context
	apiID, err := a.getAPIIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}

	return a.client.DeleteAPIVersion(ctx, apiID, id)
}

// GetByName gets an API version by name
func (a *APIVersionAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	// API versions are looked up by the planner from the list
	// No direct "get by name" API available
	return nil, nil
}

// ResourceType returns the resource type name
func (a *APIVersionAdapter) ResourceType() string {
	return "api_version"
}

// RequiredFields returns the required fields for creation
func (a *APIVersionAdapter) RequiredFields() []string {
	return []string{} // No required fields according to the SDK model (all are pointers)
}

// getAPIID extracts the API ID from the context
func (a *APIVersionAdapter) getAPIID(_ context.Context) (string, error) {
	// Use stored context (for Create operations)
	if a.execCtx != nil && a.execCtx.PlannedChange != nil {
		change := *a.execCtx.PlannedChange
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID != "" {
			return apiRef.ID, nil
		}
	}
	
	return "", fmt.Errorf("API ID is required for version operations")
}

// getAPIIDFromExecutionContext extracts the API ID from ExecutionContext parameter
func (a *APIVersionAdapter) getAPIIDFromExecutionContext(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for version operations")
	}
	
	change := *execCtx.PlannedChange
	
	// Priority 1: Check References (for Create operations)
	if apiRef, ok := change.References["api_id"]; ok && apiRef.ID != "" {
		return apiRef.ID, nil
	}
	
	// Priority 2: Check Parent field (for Delete operations)
	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}
	
	return "", fmt.Errorf("API ID is required for version operations")
}

// APIVersionResourceInfo implements ResourceInfo for API versions
type APIVersionResourceInfo struct {
	version *state.APIVersion
}

func (a *APIVersionResourceInfo) GetID() string {
	return a.version.ID
}

func (a *APIVersionResourceInfo) GetName() string {
	return a.version.Version
}

func (a *APIVersionResourceInfo) GetLabels() map[string]string {
	// API versions don't support labels in the SDK
	return make(map[string]string)
}

func (a *APIVersionResourceInfo) GetNormalizedLabels() map[string]string {
	return make(map[string]string)
}