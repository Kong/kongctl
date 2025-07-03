package executor

import (
	"context"
	"fmt"
	"os"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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
	var portal kkComps.CreatePortal
	
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
	
	if defaultAPIVisibility, ok := change.Fields["default_api_visibility"].(string); ok {
		visibility := kkComps.DefaultAPIVisibility(defaultAPIVisibility)
		portal.DefaultAPIVisibility = &visibility
	}
	
	if defaultPageVisibility, ok := change.Fields["default_page_visibility"].(string); ok {
		visibility := kkComps.DefaultPageVisibility(defaultPageVisibility)
		portal.DefaultPageVisibility = &visibility
	}
	
	if defaultAppAuthStrategyID, ok := change.Fields["default_application_auth_strategy_id"].(string); ok {
		portal.DefaultApplicationAuthStrategyID = &defaultAppAuthStrategyID
	}
	
	// Handle labels - preserve user labels
	// The state client will add management labels (managed, last-updated, protected)
	portalLabels := make(map[string]*string)
	
	// First, copy user-defined labels from the change
	if labelsField, ok := change.Fields["labels"].(map[string]interface{}); ok {
		debugLog("Found user labels in fields: %+v", labelsField)
		for k, v := range labelsField {
			if strVal, ok := v.(string); ok {
				// Only copy user labels (non-KONGCTL labels)
				if !labels.IsKongctlLabel(k) {
					portalLabels[k] = &strVal
					debugLog("Adding user label: %s=%s", k, strVal)
				}
			}
		}
	}
	
	// Add protection label based on change.Protection field
	protectionValue := labels.FalseValue
	if prot, ok := change.Protection.(bool); ok && prot {
		protectionValue = labels.TrueValue
		debugLog("Setting protection label to true")
	} else {
		debugLog("Setting protection label to false")
	}
	portalLabels[labels.ProtectedKey] = &protectionValue
	
	// Set labels on portal
	if len(portalLabels) > 0 {
		portal.Labels = portalLabels
		debugLog("Portal will have %d labels (including protection)", len(portalLabels))
	}
	
	// Create the portal
	debugLog("Final portal before creation: Name=%s, Labels=%+v", portal.Name, portal.Labels)
	resp, err := e.client.CreatePortal(ctx, portal)
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
	// Protection changes are always allowed (to unprotect a resource)
	isProtected := portal.NormalizedLabels[labels.ProtectedKey] == "true"
	
	// Check if this is a protection change
	isProtectionChange := false
	var protectionChange planner.ProtectionChange
	
	// Handle both direct struct and map from JSON deserialization
	switch p := change.Protection.(type) {
	case planner.ProtectionChange:
		isProtectionChange = true
		protectionChange = p
	case map[string]interface{}:
		// From JSON deserialization
		if oldVal, hasOld := p["old"].(bool); hasOld {
			if newVal, hasNew := p["new"].(bool); hasNew {
				isProtectionChange = true
				protectionChange = planner.ProtectionChange{
					Old: oldVal,
					New: newVal,
				}
			}
		}
	}
	
	if isProtected && !isProtectionChange {
		// Regular update to a protected resource is not allowed
		return "", fmt.Errorf("resource is protected and cannot be updated")
	}
	
	// Build sparse update request - only include fields that changed
	var updatePortal kkComps.UpdatePortal
	
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
		case "default_api_visibility":
			if visibility, ok := value.(string); ok {
				vis := kkComps.UpdatePortalDefaultAPIVisibility(visibility)
				updatePortal.DefaultAPIVisibility = &vis
			}
		case "default_page_visibility":
			if visibility, ok := value.(string); ok {
				vis := kkComps.UpdatePortalDefaultPageVisibility(visibility)
				updatePortal.DefaultPageVisibility = &vis
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
	
	// Apply any user label updates from the change (excluding KONGCTL labels)
	if labelsField, ok := change.Fields["labels"].(map[string]interface{}); ok {
		for k, v := range labelsField {
			if strVal, ok := v.(string); ok {
				if !labels.IsKongctlLabel(k) {
					userLabels[k] = strVal
				}
			}
		}
	}
	
	// Handle protection based on change.Protection field
	if isProtectionChange {
		// Protection is changing
		if protectionChange.New {
			userLabels[labels.ProtectedKey] = labels.TrueValue
		} else {
			userLabels[labels.ProtectedKey] = labels.FalseValue
		}
	} else {
		// Not a protection change - preserve current protection status
		if isProtected {
			userLabels[labels.ProtectedKey] = labels.TrueValue
		} else {
			userLabels[labels.ProtectedKey] = labels.FalseValue
		}
	}
	
	// Add management labels (will preserve the protection label we just set)
	allLabels := labels.AddManagedLabels(userLabels)
	updatePortal.Labels = labels.DenormalizeLabels(allLabels)
	
	// Update the portal
	resp, err := e.client.UpdatePortal(ctx, change.ResourceID, updatePortal)
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