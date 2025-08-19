package executor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// APIPublicationAdapter implements CreateDeleteOperations for API publications
// API publications only support create and delete operations, not updates
type APIPublicationAdapter struct {
	client  *state.Client
	logger  *slog.Logger
	execCtx *ExecutionContext // Store execution context for helper methods
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
	create *kkComps.APIPublication) error {
	// Store execution context for use in helper methods
	a.execCtx = execCtx
	
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
	_ string) (string, error) {
	// Get API ID from context
	apiID, err := a.getAPIID(ctx)
	if err != nil {
		return "", err
	}

	// Get portal ID from context
	portalID, err := a.getPortalID(ctx)
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

// ResourceType returns the resource type name
func (a *APIPublicationAdapter) ResourceType() string {
	return "api_publication"
}

// RequiredFields returns the required fields for creation
func (a *APIPublicationAdapter) RequiredFields() []string {
	return []string{"portal_id"}
}

// getPortalID extracts the portal ID from the execution context
func (a *APIPublicationAdapter) getPortalID(_ context.Context) (string, error) {
	// Use stored context (for Create operations)
	if a.execCtx != nil && a.execCtx.PlannedChange != nil {
		change := *a.execCtx.PlannedChange
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID != "" {
			return portalRef.ID, nil
		}
		// Check fields as fallback
		if portalID, ok := change.Fields["portal_id"].(string); ok {
			return portalID, nil
		}
	}

	return "", fmt.Errorf("portal ID is required for publication operations")
}

// getAPIID extracts the API ID from the execution context
func (a *APIPublicationAdapter) getAPIID(_ context.Context) (string, error) {
	// Use stored context (for Create operations)
	if a.execCtx != nil && a.execCtx.PlannedChange != nil {
		change := *a.execCtx.PlannedChange
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID != "" {
			return apiRef.ID, nil
		}
	}

	return "", fmt.Errorf("API ID is required for publication operations")
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