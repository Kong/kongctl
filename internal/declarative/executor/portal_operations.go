package executor

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
)

// createPortal handles CREATE operations for portals
func (e *Executor) createPortal(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Debug logging
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == "true"
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG [portal_operations]: "+format+"\n", args...)
		}
	}
	
	debugLog("Creating portal with fields: %+v", change.Fields)
	
	// Extract portal fields
	var portal kkInternalComps.CreatePortal
	
	// Required fields
	if name, ok := change.Fields["name"].(string); ok {
		portal.Name = name
	} else {
		return "", fmt.Errorf("portal name is required")
	}
	
	// Optional fields
	if desc, ok := change.Fields["description"].(string); ok {
		portal.Description = &desc
	}
	
	if displayName, ok := change.Fields["display_name"].(string); ok {
		portal.DisplayName = &displayName
	}
	
	if authEnabled, ok := change.Fields["authentication_enabled"].(bool); ok {
		portal.AuthenticationEnabled = &authEnabled
	}
	
	if rbacEnabled, ok := change.Fields["rbac_enabled"].(bool); ok {
		portal.RbacEnabled = &rbacEnabled
	}
	
	if autoApproveDevelopers, ok := change.Fields["auto_approve_developers"].(bool); ok {
		portal.AutoApproveDevelopers = &autoApproveDevelopers
	}
	
	if autoApproveApplications, ok := change.Fields["auto_approve_applications"].(bool); ok {
		portal.AutoApproveApplications = &autoApproveApplications
	}
	
	// Handle labels - preserve user labels only
	// The state client will add management labels
	if labelsField, ok := change.Fields["labels"].(map[string]interface{}); ok {
		debugLog("Found labels in fields: %+v", labelsField)
		userLabels := make(map[string]*string)
		for k, v := range labelsField {
			if strVal, ok := v.(string); ok && !labels.IsKongctlLabel(k) {
				userLabels[k] = &strVal
				debugLog("Adding user label: %s=%s", k, strVal)
			}
		}
		if len(userLabels) > 0 {
			portal.Labels = userLabels
			debugLog("Portal will have %d user labels", len(userLabels))
		} else {
			debugLog("No user labels to set on portal")
		}
	} else {
		debugLog("No labels field found in change")
	}
	
	// Create the portal
	debugLog("Final portal before creation: Name=%s, Labels=%+v", portal.Name, portal.Labels)
	resp, err := e.client.CreatePortal(ctx, portal, change.ConfigHash)
	if err != nil {
		return "", err
	}
	
	return resp.ID, nil
}

// updatePortal handles UPDATE operations for portals
func (e *Executor) updatePortal(ctx context.Context, change planner.PlannedChange) (string, error) {
	// First, validate protection status at execution time
	portal, err := e.client.GetPortalByName(ctx, getResourceName(change.Fields))
	if err != nil {
		return "", fmt.Errorf("failed to fetch portal for protection check: %w", err)
	}
	if portal == nil {
		return "", fmt.Errorf("portal no longer exists")
	}
	
	// Check if portal is protected
	isProtected := portal.NormalizedLabels[labels.ProtectedKey] == "true"
	if isProtected {
		return "", fmt.Errorf("resource is protected and cannot be updated")
	}
	
	// Build sparse update request - only include fields that changed
	var updatePortal kkInternalComps.UpdatePortal
	
	// Name is always required for updates
	if name, ok := change.Fields["name"].(string); ok {
		updatePortal.Name = &name
	} else {
		return "", fmt.Errorf("portal name is required")
	}
	
	// Only include fields that are in the change.Fields map (excluding "name")
	// These represent actual changes detected by the planner
	for field, value := range change.Fields {
		switch field {
		case "description":
			if desc, ok := value.(string); ok {
				updatePortal.Description = &desc
			}
		case "display_name":
			if displayName, ok := value.(string); ok {
				updatePortal.DisplayName = &displayName
			}
		case "authentication_enabled":
			if authEnabled, ok := value.(bool); ok {
				updatePortal.AuthenticationEnabled = &authEnabled
			}
		case "rbac_enabled":
			if rbacEnabled, ok := value.(bool); ok {
				updatePortal.RbacEnabled = &rbacEnabled
			}
		case "auto_approve_developers":
			if autoApproveDevelopers, ok := value.(bool); ok {
				updatePortal.AutoApproveDevelopers = &autoApproveDevelopers
			}
		case "auto_approve_applications":
			if autoApproveApplications, ok := value.(bool); ok {
				updatePortal.AutoApproveApplications = &autoApproveApplications
			}
		case "default_application_auth_strategy_id":
			if authID, ok := value.(string); ok {
				updatePortal.DefaultApplicationAuthStrategyID = &authID
			}
		// Skip "name" as it's already handled above
		// Skip "labels" as they're handled separately below
		}
	}
	
	// Handle labels - preserve existing user labels from the portal
	userLabels := make(map[string]string)
	for k, v := range portal.Labels {
		if !labels.IsKongctlLabel(k) {
			userLabels[k] = v
		}
	}
	
	// Apply any label updates from the change
	if labelsField, ok := change.Fields["labels"].(map[string]interface{}); ok {
		for k, v := range labelsField {
			if strVal, ok := v.(string); ok && !labels.IsKongctlLabel(k) {
				userLabels[k] = strVal
			}
		}
	}
	
	// Update management labels with new hash and timestamp
	allLabels := labels.AddManagedLabels(userLabels, change.ConfigHash)
	allLabels[labels.LastUpdatedKey] = time.Now().UTC().Format("20060102-150405Z")
	updatePortal.Labels = labels.DenormalizeLabels(allLabels)
	
	// Update the portal
	resp, err := e.client.UpdatePortal(ctx, change.ResourceID, updatePortal, change.ConfigHash)
	if err != nil {
		return "", err
	}
	
	return resp.ID, nil
}

// deletePortal handles DELETE operations for portals
func (e *Executor) deletePortal(ctx context.Context, change planner.PlannedChange) error {
	// First, validate protection status at execution time
	portal, err := e.client.GetPortalByName(ctx, getResourceName(change.Fields))
	if err != nil {
		return fmt.Errorf("failed to fetch portal for protection check: %w", err)
	}
	if portal == nil {
		// Portal already deleted, consider this success
		return nil
	}
	
	// Check if portal is protected
	isProtected := portal.NormalizedLabels[labels.ProtectedKey] == "true"
	if isProtected {
		return fmt.Errorf("resource is protected and cannot be deleted")
	}
	
	// Verify it's a managed resource
	if !labels.IsManagedResource(portal.NormalizedLabels) {
		return fmt.Errorf("cannot delete portal: not a KONGCTL-managed resource")
	}
	
	// Delete the portal with force=true to cascade delete child resources
	err = e.client.DeletePortal(ctx, change.ResourceID, true)
	if err != nil {
		return fmt.Errorf("failed to delete portal: %w", err)
	}
	
	return nil
}