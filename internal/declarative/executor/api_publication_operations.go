package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// createAPIPublication creates a new API publication
func (e *Executor) createAPIPublication(ctx context.Context, change planner.PlannedChange) (string, error) {
	if e.client == nil {
		return "", fmt.Errorf("client not configured")
	}

	// Get parent API ID
	parentAPIID, err := e.getParentAPIID(ctx, change)
	if err != nil {
		return "", err
	}

	// Get portal ID from fields
	portalID, ok := change.Fields["portal_id"].(string)
	if !ok {
		return "", fmt.Errorf("portal_id is required for API publication")
	}

	// Build publication object
	publication := kkComps.APIPublication{}

	// Map fields to SDK request
	if authStrategyIDs, ok := change.Fields["auth_strategy_ids"].([]interface{}); ok {
		ids := make([]string, len(authStrategyIDs))
		for i, id := range authStrategyIDs {
			if strID, ok := id.(string); ok {
				ids[i] = strID
			}
		}
		publication.AuthStrategyIds = ids
	}
	if autoApprove, ok := change.Fields["auto_approve_registrations"].(bool); ok {
		publication.AutoApproveRegistrations = &autoApprove
	}
	if visibility, ok := change.Fields["visibility"].(string); ok {
		vis := kkComps.APIPublicationVisibility(visibility)
		publication.Visibility = &vis
	}

	// Create the publication
	_, err = e.client.CreateAPIPublication(ctx, parentAPIID, portalID, publication)
	if err != nil {
		return "", fmt.Errorf("failed to create API publication: %w", err)
	}

	// Publications don't have their own ID - they're identified by API+Portal combination
	return portalID, nil
}

// deleteAPIPublication deletes an API publication
func (e *Executor) deleteAPIPublication(ctx context.Context, change planner.PlannedChange) error {
	if e.client == nil {
		return fmt.Errorf("client not configured")
	}

	// Get parent API ID
	if change.Parent == nil {
		return fmt.Errorf("parent API reference required for API publication deletion")
	}

	// Get the parent API by ref
	parentAPI, err := e.client.GetAPIByName(ctx, change.Parent.Ref)
	if err != nil {
		return fmt.Errorf("failed to get parent API: %w", err)
	}
	if parentAPI == nil {
		return fmt.Errorf("parent API not found: %s", change.Parent.Ref)
	}

	// For delete, we need the portal ID, not publication ID
	// The ResourceID should contain the portal ID for publications
	if err := e.client.DeleteAPIPublication(ctx, parentAPI.ID, change.ResourceID); err != nil {
		return fmt.Errorf("failed to delete API publication: %w", err)
	}

	return nil
}

// Note: API publications don't support update operations in the SDK