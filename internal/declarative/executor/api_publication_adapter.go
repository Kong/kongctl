package executor

import (
	"context"
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// APIPublicationAdapter implements CreateDeleteOperations for API publications
// API publications only support create and delete operations, not updates
type APIPublicationAdapter struct {
	client *state.Client
}

// NewAPIPublicationAdapter creates a new API publication adapter
func NewAPIPublicationAdapter(client *state.Client) *APIPublicationAdapter {
	return &APIPublicationAdapter{client: client}
}

// MapCreateFields maps fields to APIPublication
func (a *APIPublicationAdapter) MapCreateFields(ctx context.Context, fields map[string]interface{},
	create *kkComps.APIPublication) error {
	// Get the planned change from context to access references
	change, _ := ctx.Value(contextKeyPlannedChange).(planner.PlannedChange)

	// Handle auth strategy IDs references
	if authStrategyRefs, ok := change.References["auth_strategy_ids"]; ok {
		// For multiple references, we expect the IDs to be comma-separated
		if authStrategyRefs.ID != "" {
			// The executor should have resolved these to a comma-separated list
			// Split the comma-separated IDs into a slice
			ids := strings.Split(authStrategyRefs.ID, ",")
			create.AuthStrategyIds = ids
		}
		// Auth strategy resolution will be handled by the executor if ID is empty
	} else if authStrategyIDs, ok := fields["auth_strategy_ids"].(string); ok {
		ids := strings.Split(authStrategyIDs, ",")
		create.AuthStrategyIds = ids
	} else if authStrategyIDsList, ok := fields["auth_strategy_ids"].([]string); ok {
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

	// Check that we have auth strategy IDs
	if len(req.AuthStrategyIds) == 0 {
		return "", fmt.Errorf("auth_strategy_ids is required for API publication")
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
func (a *APIPublicationAdapter) Delete(ctx context.Context, id string) error {
	// Get API ID from context
	apiID, err := a.getAPIID(ctx)
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
	return []string{"portal_id", "auth_strategy_ids"}
}

// getPortalID extracts the portal ID from the context
func (a *APIPublicationAdapter) getPortalID(ctx context.Context) (string, error) {
	// Get the planned change from context
	change, ok := ctx.Value(contextKeyPlannedChange).(planner.PlannedChange)
	if !ok {
		return "", fmt.Errorf("planned change not found in context")
	}

	// Get portal ID from references
	if portalRef, ok := change.References["portal_id"]; ok {
		if portalRef.ID != "" {
			return portalRef.ID, nil
		}
	}
	
	// Check fields as fallback
	if portalID, ok := change.Fields["portal_id"].(string); ok {
		return portalID, nil
	}

	return "", fmt.Errorf("portal ID is required for publication operations")
}

// getAPIID extracts the API ID from the context
func (a *APIPublicationAdapter) getAPIID(ctx context.Context) (string, error) {
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