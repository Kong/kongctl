package executor

import (
	"context"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/state"
)

// APIVersionAdapter implements CreateDeleteOperations for API versions
// API versions only support create and delete operations, not updates
type APIVersionAdapter struct {
	client *state.Client
}

// NewAPIVersionAdapter creates a new API version adapter
func NewAPIVersionAdapter(client *state.Client) *APIVersionAdapter {
	return &APIVersionAdapter{client: client}
}

// MapCreateFields maps fields to CreateAPIVersionRequest
func (a *APIVersionAdapter) MapCreateFields(_ context.Context, _ *ExecutionContext, fields map[string]any,
	create *kkComps.CreateAPIVersionRequest,
) error {
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
	_ string, execCtx *ExecutionContext,
) (string, error) {
	// Get API ID from execution context
	apiID, err := a.getAPIIDFromExecutionContext(execCtx)
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

// GetByID gets an API version by ID
func (a *APIVersionAdapter) GetByID(
	ctx context.Context, versionID string, execCtx *ExecutionContext,
) (ResourceInfo, error) {
	// Get API ID from execution context
	apiID, err := a.getAPIIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get API ID for version lookup: %w", err)
	}

	// Fetch the full API version
	version, err := a.client.FetchAPIVersion(ctx, apiID, versionID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API version: %w", err)
	}

	if version == nil {
		return nil, nil // Not found
	}

	// Convert to ResourceInfo
	return NewAPIVersionResourceInfo(version), nil
}

// ResourceType returns the resource type name
func (a *APIVersionAdapter) ResourceType() string {
	return "api_version"
}

// RequiredFields returns the required fields for creation
func (a *APIVersionAdapter) RequiredFields() []string {
	return []string{} // No required fields according to the SDK model (all are pointers)
}

// MapUpdateFields maps fields for update operations
func (a *APIVersionAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext,
	fields map[string]any, update *kkComps.APIVersion, _ map[string]string,
) error {
	// Map version field if changed
	if version, ok := fields["version"].(string); ok {
		update.Version = &version
	}

	// Map spec field if changed
	if spec, ok := fields["spec"].(map[string]any); ok {
		if content, ok := spec["content"].(string); ok {
			update.Spec = &kkComps.APIVersionSpec{
				Content: &content,
			}
		}
	}

	return nil
}

// Update updates an existing API version
func (a *APIVersionAdapter) Update(ctx context.Context, id string,
	update kkComps.APIVersion, _ string, execCtx *ExecutionContext,
) (string, error) {
	// Get API ID from execution context
	apiID, err := a.getAPIIDFromExecutionContext(execCtx)
	if err != nil {
		return "", fmt.Errorf("failed to get API ID for version update: %w", err)
	}

	// Call client's UpdateAPIVersion
	resp, err := a.client.UpdateAPIVersion(ctx, apiID, id, update)
	if err != nil {
		return "", fmt.Errorf("failed to update API version: %w", err)
	}

	if resp == nil {
		return "", fmt.Errorf("API version update returned no response")
	}

	return resp.ID, nil
}

// SupportsUpdate returns true as API versions now support updates
func (a *APIVersionAdapter) SupportsUpdate() bool {
	return true
}

// getAPIIDFromExecutionContext extracts the API ID from ExecutionContext parameter (used for Delete operations)
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

// NewAPIVersionResourceInfo creates a new APIVersionResourceInfo from an APIVersion
func NewAPIVersionResourceInfo(version *state.APIVersion) *APIVersionResourceInfo {
	return &APIVersionResourceInfo{version: version}
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
