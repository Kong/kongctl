package executor

import (
	"context"
	"fmt"
	"log/slog"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/state"
)

// APIPublicationAdapter implements CreateDeleteOperations for API publications
// API publications only support create and delete operations, not updates
type APIPublicationAdapter struct {
	client *state.Client
	logger *slog.Logger
}

// NewAPIPublicationAdapter creates a new API publication adapter
func NewAPIPublicationAdapter(client *state.Client) *APIPublicationAdapter {
	return &APIPublicationAdapter{
		client: client,
		logger: slog.Default(),
	}
}

// MapCreateFields maps fields to APIPublication
func (a *APIPublicationAdapter) MapCreateFields(
	_ context.Context, execCtx *ExecutionContext, fields map[string]any,
	create *kkComps.APIPublication,
) error {
	// Get the planned change from execution context to access references
	change := *execCtx.PlannedChange

	// Handle auth strategy IDs references
	if authStrategyRefs, ok := change.References["auth_strategy_ids"]; ok && authStrategyRefs.IsArray {
		// Handle array references - use ResolvedIDs if available
		if len(authStrategyRefs.ResolvedIDs) > 0 {
			create.AuthStrategyIds = authStrategyRefs.ResolvedIDs
		}
	} else if authStrategyIDs, ok := fields["auth_strategy_ids"].([]any); ok {
		// Fallback: Convert interface array to string array
		ids := make([]string, 0, len(authStrategyIDs))
		for _, id := range authStrategyIDs {
			if strID, ok := id.(string); ok {
				ids = append(ids, strID)
			}
		}
		create.AuthStrategyIds = ids
	} else if authStrategyIDsList, ok := fields["auth_strategy_ids"].([]string); ok {
		// Direct array assignment
		create.AuthStrategyIds = authStrategyIDsList
	}

	// Optional fields
	if autoApprove, ok := fields["auto_approve_registrations"].(bool); ok {
		create.AutoApproveRegistrations = &autoApprove
	}

	if visibilityStr, ok := fields["visibility"].(string); ok {
		visibility := kkComps.APIPublicationVisibility(visibilityStr)
		create.Visibility = &visibility
	}

	return nil
}

// Create creates a new API publication
func (a *APIPublicationAdapter) Create(ctx context.Context, req kkComps.APIPublication,
	_ string, execCtx *ExecutionContext,
) (string, error) {
	// Get API ID from execution context
	apiID, err := a.getAPIIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}

	// Get portal ID from execution context
	portalID, err := a.getPortalIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}

	_, err = a.client.CreateAPIPublication(ctx, apiID, portalID, req)
	if err != nil {
		return "", err
	}
	// API publications don't have their own ID - they're identified by the api_id
	return apiID, nil
}

// Delete deletes an API publication
func (a *APIPublicationAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	// Get API ID from execution context
	apiID, err := a.getAPIIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}

	return a.client.DeleteAPIPublication(ctx, apiID, id)
}

// GetByName gets an API publication by name
func (a *APIPublicationAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	// API publications don't have names and are singleton resources per API
	// The planner handles this by checking existence
	return nil, nil
}

// GetByID gets an API publication by ID
func (a *APIPublicationAdapter) GetByID(_ context.Context, _ string, _ *ExecutionContext) (ResourceInfo, error) {
	// API publications don't have a direct "get by ID" method - they're singleton resources per API
	// For now, return nil to indicate lookup by ID is not available
	return nil, nil
}

// ResourceType returns the resource type name
func (a *APIPublicationAdapter) ResourceType() string {
	return "api_publication"
}

// RequiredFields returns the required fields for creation
func (a *APIPublicationAdapter) RequiredFields() []string {
	return []string{"portal_id"}
}

// MapUpdateFields maps fields for update operations (not supported for API publications)
func (a *APIPublicationAdapter) MapUpdateFields(
	_ context.Context, _ *ExecutionContext, _ map[string]any,
	_ *kkComps.APIPublication, _ map[string]string,
) error {
	return fmt.Errorf("API publications do not support update operations")
}

// Update is not supported for API publications
func (a *APIPublicationAdapter) Update(
	_ context.Context, _ string, _ kkComps.APIPublication, _ string, _ *ExecutionContext,
) (string, error) {
	return "", fmt.Errorf("API publications do not support update operations")
}

// SupportsUpdate returns false as API publications don't support updates
func (a *APIPublicationAdapter) SupportsUpdate() bool {
	return false
}

// getPortalIDFromExecutionContext extracts the portal ID from ExecutionContext parameter
func (a *APIPublicationAdapter) getPortalIDFromExecutionContext(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for publication operations")
	}

	change := *execCtx.PlannedChange
	if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID != "" {
		return portalRef.ID, nil
	}
	// Check fields as fallback
	if portalID, ok := change.Fields["portal_id"].(string); ok {
		return portalID, nil
	}

	return "", fmt.Errorf("portal ID is required for publication operations")
}

// getAPIIDFromExecutionContext extracts the API ID from ExecutionContext parameter
func (a *APIPublicationAdapter) getAPIIDFromExecutionContext(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for publication operations")
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

	// Priority 3: Check Fields (special case for api_publication delete)
	if apiID, ok := change.Fields["api_id"].(string); ok && apiID != "" {
		return apiID, nil
	}

	return "", fmt.Errorf("API ID is required for publication operations")
}

// APIPublicationResourceInfo implements ResourceInfo for API publications
type APIPublicationResourceInfo struct {
	publication *state.APIPublication
}

func (a *APIPublicationResourceInfo) GetID() string {
	return a.publication.ID
}

func (a *APIPublicationResourceInfo) GetName() string {
	// API publications don't have names, use ID instead
	return a.publication.ID
}

func (a *APIPublicationResourceInfo) GetLabels() map[string]string {
	// API publications don't support labels in the SDK
	return make(map[string]string)
}

func (a *APIPublicationResourceInfo) GetNormalizedLabels() map[string]string {
	return make(map[string]string)
}
