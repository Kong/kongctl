package executor

import (
	"context"
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/planner"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// createAPIPublication creates a new API publication
// Deprecated: Use APIPublicationAdapter with BaseCreateDeleteExecutor instead
//
//nolint:unused // kept for backward compatibility, will be removed in Phase 2 cleanup
func (e *Executor) createAPIPublication(ctx context.Context, change planner.PlannedChange) (string, error) {
	if e.client == nil {
		return "", fmt.Errorf("client not configured")
	}

	// Get parent API ID
	parentAPIID, err := e.getParentAPIID(ctx, change)
	if err != nil {
		return "", err
	}

	// Get portal ID - check resolved reference first, then field value
	var portalID string
	
	// First check if we have a resolved reference
	if change.References != nil {
		if ref, exists := change.References["portal_id"]; exists && ref.ID != "" && ref.ID != "[unknown]" {
			portalID = ref.ID
		}
	}
	
	// If no resolved reference, check field value
	if portalID == "" {
		fieldValue, ok := change.Fields["portal_id"].(string)
		if !ok {
			return "", fmt.Errorf("portal_id is required for API publication")
		}
		
		// If it's a UUID, use it directly
		if isUUID(fieldValue) {
			portalID = fieldValue
		} else {
			// It's a reference that needs runtime resolution
			// Use the reference info from the change if available
			refInfo := planner.ReferenceInfo{Ref: fieldValue}
			if ref, exists := change.References["portal_id"]; exists {
				refInfo = ref
			}
			resolvedID, err := e.resolvePortalRef(ctx, refInfo)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference %q: %w", fieldValue, err)
			}
			portalID = resolvedID
		}
	}

	// Build publication object
	publication := kkComps.APIPublication{}

	// Map fields to SDK request
	if authStrategyIDs, ok := change.Fields["auth_strategy_ids"].([]interface{}); ok {
		ids := make([]string, 0, len(authStrategyIDs))
		for _, id := range authStrategyIDs {
			if strID, ok := id.(string); ok {
				// Check if this is a UUID or a reference
				if isUUID(strID) {
					ids = append(ids, strID)
				} else {
					// It's a reference, resolve it
					resolvedID, err := e.resolveAuthStrategyRef(ctx, strID)
					if err != nil {
						return "", fmt.Errorf("failed to resolve auth strategy reference %q: %w", strID, err)
					}
					ids = append(ids, resolvedID)
				}
			}
		}
		publication.AuthStrategyIds = ids
	}
	// Also handle []string type (from planner)
	if authStrategyIDs, ok := change.Fields["auth_strategy_ids"].([]string); ok {
		ids := make([]string, 0, len(authStrategyIDs))
		for _, strID := range authStrategyIDs {
			// Check if this is a UUID or a reference
			if isUUID(strID) {
				ids = append(ids, strID)
			} else {
				// It's a reference, resolve it
				resolvedID, err := e.resolveAuthStrategyRef(ctx, strID)
				if err != nil {
					return "", fmt.Errorf("failed to resolve auth strategy reference %q: %w", strID, err)
				}
				ids = append(ids, resolvedID)
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
// Deprecated: Use APIPublicationAdapter with BaseCreateDeleteExecutor instead
//
//nolint:unused // kept for backward compatibility, will be removed in Phase 2 cleanup
func (e *Executor) deleteAPIPublication(ctx context.Context, change planner.PlannedChange) error {
	if e.client == nil {
		return fmt.Errorf("client not configured")
	}

	// Get parent API ID
	parentAPIID, err := e.getParentAPIID(ctx, change)
	if err != nil {
		return err
	}

	// Extract portal ID from fields or ResourceID
	var portalID string
	
	// First try to get portal ID from fields (new format)
	if pid, ok := change.Fields["portal_id"].(string); ok {
		portalID = pid
	} else {
		// Fallback to ResourceID for backward compatibility
		// ResourceID might be in format "api_id:portal_id" or just "portal_id"
		if idx := strings.LastIndex(change.ResourceID, ":"); idx != -1 {
			portalID = change.ResourceID[idx+1:]
		} else {
			portalID = change.ResourceID
		}
	}

	if err := e.client.DeleteAPIPublication(ctx, parentAPIID, portalID); err != nil {
		return fmt.Errorf("failed to delete API publication: %w", err)
	}

	return nil
}

// Note: API publications don't support update operations in the SDK

// isUUID checks if a string is a UUID format
//
//nolint:unused // kept for backward compatibility, will be removed in Phase 2 cleanup
func isUUID(s string) bool {
	// Simple check - UUID format: 8-4-4-4-12 characters
	return len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}