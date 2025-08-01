package executor

import (
	"context"
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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
func (a *APIVersionAdapter) MapCreateFields(_ context.Context, fields map[string]interface{},
	create *kkComps.CreateAPIVersionRequest) error {
	// Version field
	if version, ok := fields["version"].(string); ok {
		create.Version = &version
	}
	
	// Spec field (optional)
	if spec, ok := fields["spec"].(map[string]interface{}); ok {
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
func (a *APIVersionAdapter) Delete(ctx context.Context, id string) error {
	// Get API ID from context
	apiID, err := a.getAPIID(ctx)
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
func (a *APIVersionAdapter) getAPIID(ctx context.Context) (string, error) {
	// Get the planned change from context
	change, ok := ctx.Value(contextKeyPlannedChange).(planner.PlannedChange)
	if !ok {
		return "", fmt.Errorf("planned change not found in context")
	}

	// Get API ID from references
	if apiRef, ok := change.References["api_id"]; ok {
		if apiRef.ID != "" {
			return apiRef.ID, nil
		}
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