package planner

import (
	"context"
	"fmt"
	"log/slog"

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
			portalChangeID := p.planPortalCreate(desiredPortal, plan)
			// Plan child resources after portal creation
			p.planPortalChildResourcesCreate(desiredPortal, portalChangeID, plan)
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

			// Plan child resource changes for existing portal
			if err := p.planPortalChildResourceChanges(ctx, current, desiredPortal, plan); err != nil {
				return err
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
func (p *portalPlannerImpl) planPortalCreate(portal resources.PortalResource, plan *Plan) string {
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
		ID:           p.NextChangeID(ActionCreate, "portal", portal.GetRef()),
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
	return change.ID
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
		// Compare only user labels to determine if update is needed
		// Convert portal's pointer map to string map for comparison
		desiredLabels := make(map[string]string)
		for k, v := range desired.Labels {
			if v != nil {
				desiredLabels[k] = *v
			}
		}
		
		if labels.CompareUserLabels(current.NormalizedLabels, desiredLabels) {
			// User labels differ, include all labels in update
			labelsMap := make(map[string]interface{})
			for k, v := range desired.Labels {
				if v != nil {
					labelsMap[k] = *v
				}
			}
			updates["labels"] = labelsMap
		}
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

	// Always include name for identification
	fields["name"] = current.Name

	// Store the fields that need updating
	for field, newValue := range updateFields {
		fields[field] = newValue
	}
	
	// Pass current labels so executor can properly handle removals
	if _, hasLabels := updateFields["labels"]; hasLabels {
		fields[FieldCurrentLabels] = current.NormalizedLabels
	}

	change := PlannedChange{
		ID:           p.NextChangeID(ActionUpdate, "portal", desired.GetRef()),
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
		ID:           p.NextChangeID(ActionUpdate, "portal", desired.GetRef()),
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
		ID:           p.NextChangeID(ActionDelete, "portal", portal.Name),
		ResourceType: "portal",
		ResourceRef:  portal.Name,
		ResourceID:   portal.ID,
		Action:       ActionDelete,
		Fields:       map[string]interface{}{"name": portal.Name},
		DependsOn:    []string{},
	}

	plan.AddChange(change)
}

// planPortalChildResourcesCreate plans creation of child resources for a new portal
func (p *portalPlannerImpl) planPortalChildResourcesCreate(desired resources.PortalResource, _ string, plan *Plan) {
	// Portal ID is not yet known, will be resolved at execution time
	// But we still need to plan child resources that depend on this portal
	
	// Get the main planner instance to access child resource planning methods
	planner := p.planner
	
	// For new portals, we can't list existing resources, but we still need to plan
	// creation of child resources. Pass empty portal ID - the executor will resolve it.
	
	// Plan pages
	pages := make([]resources.PortalPageResource, 0)
	for _, page := range planner.desiredPortalPages {
		if page.Portal == desired.Ref {
			pages = append(pages, page)
		}
	}
	// Note: Passing empty portalID for new portal
	if err := planner.planPortalPagesChanges(context.Background(), "", desired.Ref, pages, plan); err != nil {
		// Log error but don't fail - portal creation should still proceed
		planner.logger.Debug("Failed to plan portal pages for new portal", 
			slog.String("portal", desired.Ref),
			slog.String("error", err.Error()))
	}
	
	// Plan snippets
	snippets := make([]resources.PortalSnippetResource, 0)
	for _, snippet := range planner.desiredPortalSnippets {
		if snippet.Portal == desired.Ref {
			snippets = append(snippets, snippet)
		}
	}
	if err := planner.planPortalSnippetsChanges(context.Background(), "", desired.Ref, snippets, plan); err != nil {
		planner.logger.Debug("Failed to plan portal snippets for new portal",
			slog.String("portal", desired.Ref),
			slog.String("error", err.Error()))
	}
	
	// Plan customization
	customizations := make([]resources.PortalCustomizationResource, 0)
	for _, customization := range planner.desiredPortalCustomizations {
		if customization.Portal == desired.Ref {
			customizations = append(customizations, customization)
		}
	}
	if err := planner.planPortalCustomizationsChanges(context.Background(), customizations, plan); err != nil {
		planner.logger.Debug("Failed to plan portal customizations for new portal",
			slog.String("portal", desired.Ref),
			slog.String("error", err.Error()))
	}
	
	// Plan custom domain
	domains := make([]resources.PortalCustomDomainResource, 0)
	for _, domain := range planner.desiredPortalCustomDomains {
		if domain.Portal == desired.Ref {
			domains = append(domains, domain)
		}
	}
	if err := planner.planPortalCustomDomainsChanges(context.Background(), domains, plan); err != nil {
		planner.logger.Debug("Failed to plan portal custom domains for new portal",
			slog.String("portal", desired.Ref),
			slog.String("error", err.Error()))
	}
}

// planPortalChildResourceChanges plans changes for child resources of an existing portal
func (p *portalPlannerImpl) planPortalChildResourceChanges(
	ctx context.Context, current state.Portal, desired resources.PortalResource, plan *Plan,
) error {
	// Get the main planner instance to access child resource planning methods
	planner := p.planner
	
	// Plan pages - pass empty array if no pages defined
	pages := make([]resources.PortalPageResource, 0)
	// Note: Pages have already been extracted to root level by loader
	// We need to find pages that belong to this portal
	for _, page := range planner.desiredPortalPages {
		if page.Portal == desired.Ref {
			pages = append(pages, page)
		}
	}
	if err := planner.planPortalPagesChanges(ctx, current.ID, desired.Ref, pages, plan); err != nil {
		return fmt.Errorf("failed to plan portal page changes: %w", err)
	}
	
	// Plan snippets - pass empty array if no snippets defined
	snippets := make([]resources.PortalSnippetResource, 0)
	// Note: Snippets have already been extracted to root level by loader
	// We need to find snippets that belong to this portal
	for _, snippet := range planner.desiredPortalSnippets {
		if snippet.Portal == desired.Ref {
			snippets = append(snippets, snippet)
		}
	}
	if err := planner.planPortalSnippetsChanges(ctx, current.ID, desired.Ref, snippets, plan); err != nil {
		return fmt.Errorf("failed to plan portal snippet changes: %w", err)
	}
	
	// Plan customization (singleton resource)
	customizations := make([]resources.PortalCustomizationResource, 0)
	for _, customization := range planner.desiredPortalCustomizations {
		if customization.Portal == desired.Ref {
			customizations = append(customizations, customization)
		}
	}
	if err := planner.planPortalCustomizationsChanges(ctx, customizations, plan); err != nil {
		return fmt.Errorf("failed to plan portal customization changes: %w", err)
	}
	
	// Plan custom domain (singleton resource)
	domains := make([]resources.PortalCustomDomainResource, 0)
	for _, domain := range planner.desiredPortalCustomDomains {
		if domain.Portal == desired.Ref {
			domains = append(domains, domain)
		}
	}
	if err := planner.planPortalCustomDomainsChanges(ctx, domains, plan); err != nil {
		return fmt.Errorf("failed to plan portal custom domain changes: %w", err)
	}
	
	return nil
}