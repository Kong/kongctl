package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// portalPlannerImpl implements planning logic for portal resources
type portalPlannerImpl struct {
	*BasePlanner
}

// NewPortalPlanner creates a new portal planner
func NewPortalPlanner(base *BasePlanner) PortalPlanner {
	return &portalPlannerImpl{
		BasePlanner: base,
	}
}

// PlanChanges generates changes for portal resources
func (p *portalPlannerImpl) PlanChanges(ctx context.Context, plan *Plan) error {
	desired := p.GetDesiredPortals()
	
	// Skip if no portals to plan and not in sync mode
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	// Fetch current managed portals
	currentPortals, err := p.GetClient().ListManagedPortals(ctx)
	if err != nil {
		// If portal client is not configured, skip portal planning
		if err.Error() == "Portal client not configured" {
			return nil
		}
		return fmt.Errorf("failed to list current portals: %w", err)
	}

	// Index current portals by name
	currentByName := make(map[string]state.Portal)
	for _, portal := range currentPortals {
		currentByName[portal.Name] = portal
	}

	// Collect protection validation errors
	protectionErrors := &ProtectionErrorCollector{}

	// Compare each desired portal
	for _, desiredPortal := range desired {
		current, exists := currentByName[desiredPortal.Name]

		if !exists {
			// CREATE action
			p.planPortalCreate(desiredPortal, plan)
		} else {
			// Check if update needed
			isProtected := labels.IsProtectedResource(current.NormalizedLabels)

			// Get protection status from desired configuration
			shouldProtect := false
			if desiredPortal.Kongctl != nil && desiredPortal.Kongctl.Protected {
				shouldProtect = true
			}

			// Handle protection changes
			if isProtected != shouldProtect {
				// When changing protection status, include any other field updates too
				_, updateFields := p.shouldUpdatePortal(current, desiredPortal)
				p.planPortalProtectionChangeWithFields(current, desiredPortal, isProtected, shouldProtect, updateFields, plan)
			} else {
				// Check if update needed based on configuration
				needsUpdate, updateFields := p.shouldUpdatePortal(current, desiredPortal)
				if needsUpdate {
					// Regular update - check protection
					err := p.ValidateProtection("portal", desiredPortal.Name, isProtected, ActionUpdate)
					protectionErrors.Add(err)
					if err == nil {
						p.planPortalUpdateWithFields(current, desiredPortal, updateFields, plan)
					}
				}
			}
		}
	}

	// Check for managed resources to delete (sync mode only)
	if plan.Metadata.Mode == PlanModeSync {
		// Build set of desired portal names
		desiredNames := make(map[string]bool)
		for _, portal := range desired {
			desiredNames[portal.Name] = true
		}

		// Find managed portals not in desired state
		for name, current := range currentByName {
			if !desiredNames[name] {
				// Validate protection before adding DELETE
				isProtected := labels.IsProtectedResource(current.NormalizedLabels)
				err := p.ValidateProtection("portal", name, isProtected, ActionDelete)
				protectionErrors.Add(err)
				if err == nil {
					p.planPortalDelete(current, plan)
				}
			}
		}
	}

	// Fail fast if any protected resources would be modified
	if protectionErrors.HasErrors() {
		return protectionErrors.Error()
	}

	return nil
}

// planPortalCreate creates a CREATE change for a portal
func (p *portalPlannerImpl) planPortalCreate(portal resources.PortalResource, plan *Plan) {
	fields := make(map[string]interface{})
	fields["name"] = portal.Name
	if portal.DisplayName != nil {
		fields["display_name"] = *portal.DisplayName
	}
	if portal.Description != nil {
		fields["description"] = *portal.Description
	}
	if portal.AuthenticationEnabled != nil {
		fields["authentication_enabled"] = *portal.AuthenticationEnabled
	}
	if portal.RbacEnabled != nil {
		fields["rbac_enabled"] = *portal.RbacEnabled
	}
	if portal.DefaultAPIVisibility != nil {
		fields["default_api_visibility"] = string(*portal.DefaultAPIVisibility)
	}
	if portal.DefaultPageVisibility != nil {
		fields["default_page_visibility"] = string(*portal.DefaultPageVisibility)
	}
	if portal.DefaultApplicationAuthStrategyID != nil {
		fields["default_application_auth_strategy_id"] = *portal.DefaultApplicationAuthStrategyID
	}
	if portal.AutoApproveDevelopers != nil {
		fields["auto_approve_developers"] = *portal.AutoApproveDevelopers
	}
	if portal.AutoApproveApplications != nil {
		fields["auto_approve_applications"] = *portal.AutoApproveApplications
	}

	change := PlannedChange{
		ID:           p.NextChangeID(ActionCreate, portal.GetRef()),
		ResourceType: "portal",
		ResourceRef:  portal.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    []string{},
	}

	// Always set protection status explicitly
	if portal.Kongctl != nil && portal.Kongctl.Protected {
		change.Protection = true
	} else {
		change.Protection = false
	}

	// Copy user-defined labels only (protection label will be added during execution)
	if len(portal.Labels) > 0 {
		labelsMap := make(map[string]interface{})
		for k, v := range portal.Labels {
			if v != nil {
				labelsMap[k] = *v
			}
		}
		fields["labels"] = labelsMap
	}

	plan.AddChange(change)
}

// shouldUpdatePortal checks if portal needs update based on configured fields only
func (p *portalPlannerImpl) shouldUpdatePortal(
	current state.Portal,
	desired resources.PortalResource,
) (bool, map[string]interface{}) {
	updates := make(map[string]interface{})

	// Only compare fields present in desired configuration
	if desired.DisplayName != nil {
		if current.DisplayName != *desired.DisplayName {
			updates["display_name"] = *desired.DisplayName
		}
	}
	if desired.Description != nil {
		currentDesc := p.GetString(current.Description)
		if currentDesc != *desired.Description {
			updates["description"] = *desired.Description
		}
	}
	
	if desired.DefaultApplicationAuthStrategyID != nil {
		currentAuthID := p.GetString(current.DefaultApplicationAuthStrategyID)
		if currentAuthID != *desired.DefaultApplicationAuthStrategyID {
			updates["default_application_auth_strategy_id"] = *desired.DefaultApplicationAuthStrategyID
		}
	}
	
	if desired.AuthenticationEnabled != nil {
		if current.AuthenticationEnabled != *desired.AuthenticationEnabled {
			updates["authentication_enabled"] = *desired.AuthenticationEnabled
		}
	}
	
	if desired.RbacEnabled != nil {
		if current.RbacEnabled != *desired.RbacEnabled {
			updates["rbac_enabled"] = *desired.RbacEnabled
		}
	}
	
	if desired.AutoApproveDevelopers != nil {
		if current.AutoApproveDevelopers != *desired.AutoApproveDevelopers {
			updates["auto_approve_developers"] = *desired.AutoApproveDevelopers
		}
	}
	
	if desired.AutoApproveApplications != nil {
		if current.AutoApproveApplications != *desired.AutoApproveApplications {
			updates["auto_approve_applications"] = *desired.AutoApproveApplications
		}
	}
	
	if desired.DefaultAPIVisibility != nil {
		currentVisibility := string(current.DefaultAPIVisibility)
		desiredVisibility := string(*desired.DefaultAPIVisibility)
		if currentVisibility != desiredVisibility {
			updates["default_api_visibility"] = desiredVisibility
		}
	}
	
	if desired.DefaultPageVisibility != nil {
		currentVisibility := string(current.DefaultPageVisibility)
		desiredVisibility := string(*desired.DefaultPageVisibility)
		if currentVisibility != desiredVisibility {
			updates["default_page_visibility"] = desiredVisibility
		}
	}

	// Check if labels are defined in the desired state
	// If labels are defined (even if empty), we need to send them to ensure proper replacement
	if desired.Labels != nil {
		// Always include labels when they're defined in desired state
		// This ensures labels not in desired state are removed from the API
		labelsMap := make(map[string]interface{})
		for k, v := range desired.Labels {
			if v != nil {
				labelsMap[k] = *v
			}
		}
		updates["labels"] = labelsMap
	}

	return len(updates) > 0, updates
}

// planPortalUpdateWithFields creates an UPDATE change with specific fields
func (p *portalPlannerImpl) planPortalUpdateWithFields(
	current state.Portal,
	desired resources.PortalResource,
	updateFields map[string]interface{},
	plan *Plan,
) {
	fields := make(map[string]interface{})

	// Store the fields that need updating
	for field, newValue := range updateFields {
		fields[field] = newValue
	}

	// Always include name for identification
	fields["name"] = current.Name
	
	// Pass current labels so executor can properly handle removals
	if _, hasLabels := updateFields["labels"]; hasLabels {
		fields[FieldCurrentLabels] = current.NormalizedLabels
	}

	change := PlannedChange{
		ID:           p.NextChangeID(ActionUpdate, desired.GetRef()),
		ResourceType: "portal",
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    []string{},
	}

	// Check if already protected
	if labels.IsProtectedResource(current.NormalizedLabels) {
		change.Protection = true
	}

	plan.AddChange(change)
}

// planPortalProtectionChangeWithFields creates an UPDATE for protection status with optional field updates
func (p *portalPlannerImpl) planPortalProtectionChangeWithFields(
	current state.Portal,
	desired resources.PortalResource,
	wasProtected, shouldProtect bool,
	updateFields map[string]interface{},
	plan *Plan,
) {
	fields := make(map[string]interface{})

	// Include any field updates if unprotecting
	if wasProtected && !shouldProtect && len(updateFields) > 0 {
		for field, newValue := range updateFields {
			fields[field] = newValue
		}
	}

	// Always include name for identification
	fields["name"] = current.Name

	// Don't add protection label here - it will be added during execution
	// based on the Protection field

	change := PlannedChange{
		ID:           p.NextChangeID(ActionUpdate, desired.GetRef()),
		ResourceType: "portal",
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		Protection: ProtectionChange{
			Old: wasProtected,
			New: shouldProtect,
		},
		DependsOn: []string{},
	}

	plan.AddChange(change)
}

// planPortalDelete creates a DELETE change for a portal
func (p *portalPlannerImpl) planPortalDelete(portal state.Portal, plan *Plan) {
	change := PlannedChange{
		ID:           p.NextChangeID(ActionDelete, portal.Name),
		ResourceType: "portal",
		ResourceRef:  portal.Name,
		ResourceID:   portal.ID,
		Action:       ActionDelete,
		Fields:       map[string]interface{}{"name": portal.Name},
		DependsOn:    []string{},
	}

	plan.AddChange(change)
}