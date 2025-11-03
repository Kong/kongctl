package planner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
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
func (p *portalPlannerImpl) PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error {
	// Get namespace from planner context
	namespace := plannerCtx.Namespace
	desired := p.GetDesiredPortals(namespace)

	// Skip if no portals to plan and not in sync mode
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	// Fetch current managed portals from the specific namespace
	namespaceFilter := []string{namespace}
	currentPortals, err := p.GetClient().ListManagedPortals(ctx, namespaceFilter)
	if err != nil {
		// If portal client is not configured, skip portal planning
		if err.Error() == "Portal client not configured" {
			return nil
		}
		return fmt.Errorf("failed to list current portals in namespace %s: %w", namespace, err)
	}

	// Index current portals by name
	currentByName := make(map[string]state.Portal)
	for _, portal := range currentPortals {
		currentByName[portal.GetName()] = portal
	}

	// Collect protection validation errors
	protectionErrors := &ProtectionErrorCollector{}

	// Compare each desired portal
	for _, desiredPortal := range desired {
		// External portals are not managed by kongctl and exist in Konnect already.
		// We still plan their child resources based on the resolved Konnect ID when available.
		if desiredPortal.IsExternal() {
			// If we have a resolved Konnect ID, plan full child diffs (including deletes in sync mode)
			if portalID := desiredPortal.GetKonnectID(); portalID != "" {
				// Build a minimal current portal for child planning
				current := state.Portal{
					ListPortalsResponsePortal: kkComps.ListPortalsResponsePortal{ID: portalID, Name: desiredPortal.Name},
					NormalizedLabels:          map[string]string{},
				}
				p.planner.logger.Debug("Planning children for external portal",
					slog.String("ref", desiredPortal.GetRef()),
					slog.String("name", desiredPortal.Name),
					slog.String("id", portalID),
				)
				if err := p.planPortalChildResourceChanges(ctx, plannerCtx, current, desiredPortal, plan); err != nil {
					return err
				}
			} else {
				// ID not resolved – plan creates for children, executor will resolve portal at runtime
				p.planner.logger.Debug(
					"External portal without resolved ID; planning child creates only",
					slog.String("ref", desiredPortal.GetRef()),
					slog.String("name", desiredPortal.Name),
				)
				p.planPortalChildResourcesCreate(ctx, plannerCtx, desiredPortal, "", plan)
				// Add plan warning to clarify limitations (wrapped for lll)
				msg := fmt.Sprintf(
					"external portal %q has no resolved ID; "+
						"deletes/diffs of children may be incomplete",
					desiredPortal.GetRef(),
				)
				plan.AddWarning("", msg)
			}
			continue
		}

		current, exists := currentByName[desiredPortal.Name]

		if !exists {
			// CREATE action
			portalChangeID := p.planPortalCreate(desiredPortal, plan)
			// Plan child resources after portal creation
			p.planPortalChildResourcesCreate(ctx, plannerCtx, desiredPortal, portalChangeID, plan)
		} else {
			// Check if update needed
			isProtected := labels.IsProtectedResource(current.NormalizedLabels)

			// Get protection status from desired configuration
			shouldProtect := false
			if desiredPortal.Kongctl != nil && desiredPortal.Kongctl.Protected != nil && *desiredPortal.Kongctl.Protected {
				shouldProtect = true
			}

			// Handle protection changes
			if isProtected != shouldProtect {
				// When changing protection status, include any other field updates too
				needsUpdate, updateFields := p.shouldUpdatePortal(current, desiredPortal)

				// Create protection change object
				protectionChange := &ProtectionChange{
					Old: isProtected,
					New: shouldProtect,
				}

				// Validate protection change
				err := p.ValidateProtectionWithChange("portal", desiredPortal.Name, isProtected, ActionUpdate,
					protectionChange, needsUpdate)
				protectionErrors.Add(err)
				if err == nil {
					p.planPortalProtectionChangeWithFields(current, desiredPortal, isProtected, shouldProtect, updateFields, plan)
				}
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
			if err := p.planPortalChildResourceChanges(ctx, plannerCtx, current, desiredPortal, plan); err != nil {
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

	// Note: Portal child resources are already planned when processing each portal above
	// No need to plan them again here

	return nil
}

// extractPortalFields extracts fields from a portal resource for planner operations
func extractPortalFields(resource any) map[string]any {
	fields := make(map[string]any)

	portal, ok := resource.(resources.PortalResource)
	if !ok {
		return fields
	}

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

	// Copy user-defined labels only (protection label will be added during execution)
	if len(portal.Labels) > 0 {
		labelsMap := make(map[string]any)
		for k, v := range portal.Labels {
			if v != nil {
				labelsMap[k] = *v
			}
		}
		fields["labels"] = labelsMap
	}

	return fields
}

// planPortalCreate creates a CREATE change for a portal
func (p *portalPlannerImpl) planPortalCreate(portal resources.PortalResource, plan *Plan) string {
	generic := p.GetGenericPlanner()
	if generic == nil {
		// During tests, generic planner might not be initialized
		// Fall back to inline implementation
		changeID := p.NextChangeID(ActionCreate, "portal", portal.GetRef())
		change := PlannedChange{
			ID:           changeID,
			ResourceType: "portal",
			ResourceRef:  portal.GetRef(),
			Action:       ActionCreate,
			Fields:       extractPortalFields(portal),
			DependsOn:    []string{},
			Namespace:    DefaultNamespace,
		}
		if portal.Kongctl != nil && portal.Kongctl.Protected != nil {
			change.Protection = *portal.Kongctl.Protected
		}
		if portal.Kongctl != nil && portal.Kongctl.Namespace != nil {
			change.Namespace = *portal.Kongctl.Namespace
		}

		// Check for auth strategy reference
		p.addAuthStrategyReference(&change, portal)

		plan.AddChange(change)
		return changeID
	}

	// Extract protection status
	var protection any
	if portal.Kongctl != nil && portal.Kongctl.Protected != nil {
		protection = *portal.Kongctl.Protected
	}

	// Extract namespace
	namespace := DefaultNamespace
	if portal.Kongctl != nil && portal.Kongctl.Namespace != nil {
		namespace = *portal.Kongctl.Namespace
	}

	config := CreateConfig{
		ResourceType:   "portal",
		ResourceName:   portal.Name,
		ResourceRef:    portal.GetRef(),
		RequiredFields: []string{"name"},
		FieldExtractor: func(_ any) map[string]any {
			return extractPortalFields(portal)
		},
		Namespace: namespace,
		DependsOn: []string{},
	}

	change, err := generic.PlanCreate(context.Background(), config)
	if err != nil {
		// This shouldn't happen with valid configuration
		p.planner.logger.Error("Failed to plan portal create", "error", err.Error())
		return ""
	}

	// Set protection after creation
	change.Protection = protection

	// Check for auth strategy reference
	p.addAuthStrategyReference(&change, portal)

	plan.AddChange(change)
	return change.ID
}

// addAuthStrategyReference checks for and adds auth strategy reference to the change
func (p *portalPlannerImpl) addAuthStrategyReference(change *PlannedChange, portal resources.PortalResource) {
	if portal.DefaultApplicationAuthStrategyID == nil {
		return
	}

	authStrategyValue := *portal.DefaultApplicationAuthStrategyID

	// Check if this is a reference placeholder
	if strings.HasPrefix(authStrategyValue, tags.RefPlaceholderPrefix) {
		// Parse the placeholder to extract the ref
		parsedRef, field, ok := tags.ParseRefPlaceholder(authStrategyValue)
		if !ok {
			p.planner.logger.Warn("Invalid reference placeholder format",
				"field", "default_application_auth_strategy_id",
				"value", authStrategyValue)
			return
		}

		// Initialize References map if needed
		if change.References == nil {
			change.References = make(map[string]ReferenceInfo)
		}

		// Add the reference with lookup fields for resolution
		change.References["default_application_auth_strategy_id"] = ReferenceInfo{
			Ref: authStrategyValue, // Keep full placeholder for later parsing
			ID:  "",                // Will be resolved during execution
			LookupFields: map[string]string{
				"name": parsedRef, // Use ref as name for lookup
			},
		}

		p.planner.logger.Debug("Added auth strategy reference to portal",
			"portal_ref", portal.GetRef(),
			"auth_strategy_ref", parsedRef,
			"field_requested", field)
	}
}

// shouldUpdatePortal checks if portal needs update based on configured fields only
func (p *portalPlannerImpl) shouldUpdatePortal(
	current state.Portal,
	desired resources.PortalResource,
) (bool, map[string]any) {
	updates := make(map[string]any)

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
		desiredValue := *desired.DefaultApplicationAuthStrategyID

		// Skip comparison if desired value is a reference placeholder
		// The executor will resolve it and we trust it matches what's in Konnect
		if !strings.HasPrefix(desiredValue, tags.RefPlaceholderPrefix) {
			currentAuthID := p.GetString(current.DefaultApplicationAuthStrategyID)
			if currentAuthID != desiredValue {
				updates["default_application_auth_strategy_id"] = desiredValue
			}
		}
	}

	if desired.AuthenticationEnabled != nil {
		if curr := current.GetAuthenticationEnabled(); curr == nil || *curr != *desired.AuthenticationEnabled {
			updates["authentication_enabled"] = *desired.AuthenticationEnabled
		}
	}

	if desired.RbacEnabled != nil {
		if curr := current.GetRbacEnabled(); curr == nil || *curr != *desired.RbacEnabled {
			updates["rbac_enabled"] = *desired.RbacEnabled
		}
	}

	if desired.AutoApproveDevelopers != nil {
		if curr := current.GetAutoApproveDevelopers(); curr == nil || *curr != *desired.AutoApproveDevelopers {
			updates["auto_approve_developers"] = *desired.AutoApproveDevelopers
		}
	}

	if desired.AutoApproveApplications != nil {
		if curr := current.GetAutoApproveApplications(); curr == nil || *curr != *desired.AutoApproveApplications {
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
			labelsMap := make(map[string]any)
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
	updateFields map[string]any,
	plan *Plan,
) {
	// Always include name for identification
	updateFields["name"] = current.Name

	// Pass current labels so executor can properly handle removals
	if _, hasLabels := updateFields["labels"]; hasLabels {
		updateFields[FieldCurrentLabels] = current.NormalizedLabels
	}

	// Extract namespace
	namespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		namespace = *desired.Kongctl.Namespace
	}

	config := UpdateConfig{
		ResourceType:   "portal",
		ResourceName:   desired.Name,
		ResourceRef:    desired.GetRef(),
		ResourceID:     current.ID,
		CurrentFields:  nil, // Not needed for direct update
		DesiredFields:  updateFields,
		RequiredFields: []string{"name"},
		Namespace:      namespace,
	}

	generic := p.GetGenericPlanner()
	if generic == nil {
		// During tests, generic planner might not be initialized
		// Fall back to inline implementation
		fields := make(map[string]any)
		fields["name"] = current.Name
		for field, newValue := range updateFields {
			fields[field] = newValue
		}
		if _, hasLabels := updateFields["labels"]; hasLabels {
			fields[FieldCurrentLabels] = current.NormalizedLabels
		}

		changeID := p.NextChangeID(ActionUpdate, "portal", desired.GetRef())
		change := PlannedChange{
			ID:           changeID,
			ResourceType: "portal",
			ResourceRef:  desired.GetRef(),
			ResourceID:   current.ID,
			Action:       ActionUpdate,
			Fields:       fields,
			DependsOn:    []string{},
			Namespace:    DefaultNamespace,
		}
		if labels.IsProtectedResource(current.NormalizedLabels) {
			change.Protection = true
		}
		if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
			change.Namespace = *desired.Kongctl.Namespace
		}
		plan.AddChange(change)
		return
	}

	change, err := generic.PlanUpdate(context.Background(), config)
	if err != nil {
		// This shouldn't happen with valid configuration
		p.planner.logger.Error("Failed to plan portal update", "error", err.Error())
		return
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
	updateFields map[string]any,
	plan *Plan,
) {
	// Extract namespace
	namespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		namespace = *desired.Kongctl.Namespace
	}

	// Use generic protection change planner
	config := ProtectionChangeConfig{
		ResourceType: "portal",
		ResourceName: desired.Name,
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		OldProtected: wasProtected,
		NewProtected: shouldProtect,
		Namespace:    namespace,
	}

	generic := p.GetGenericPlanner()
	var change PlannedChange
	if generic != nil {
		change = generic.PlanProtectionChange(context.Background(), config)
	} else {
		// Fallback for tests
		changeID := p.NextChangeID(ActionUpdate, "portal", desired.GetRef())
		change = PlannedChange{
			ID:           changeID,
			ResourceType: "portal",
			ResourceRef:  desired.GetRef(),
			ResourceID:   current.ID,
			Action:       ActionUpdate,
			Protection: ProtectionChange{
				Old: wasProtected,
				New: shouldProtect,
			},
			Namespace: namespace,
		}
	}

	// Always include name field for identification
	fields := make(map[string]any)
	fields["name"] = current.Name

	// Include any field updates if unprotecting
	if wasProtected && !shouldProtect && len(updateFields) > 0 {
		for field, newValue := range updateFields {
			fields[field] = newValue
		}
	}

	change.Fields = fields
	plan.AddChange(change)
}

// planPortalDelete creates a DELETE change for a portal
func (p *portalPlannerImpl) planPortalDelete(portal state.Portal, plan *Plan) {
	// Extract namespace from labels (for existing resources being deleted)
	namespace := DefaultNamespace
	if ns, ok := portal.NormalizedLabels[labels.NamespaceKey]; ok {
		namespace = ns
	}

	generic := p.GetGenericPlanner()
	var change PlannedChange

	if generic != nil {
		config := DeleteConfig{
			ResourceType: "portal",
			ResourceName: portal.Name,
			ResourceRef:  portal.Name,
			ResourceID:   portal.ID,
			Namespace:    namespace,
		}
		change = generic.PlanDelete(context.Background(), config)
	} else {
		// Fallback for tests
		changeID := p.NextChangeID(ActionDelete, "portal", portal.Name)
		change = PlannedChange{
			ID:           changeID,
			ResourceType: "portal",
			ResourceRef:  portal.Name,
			ResourceID:   portal.ID,
			Action:       ActionDelete,
			Namespace:    namespace,
		}
	}

	// Add the name field for backward compatibility
	change.Fields = map[string]any{"name": portal.Name}

	plan.AddChange(change)
}

// planPortalChildResourcesCreate plans creation of child resources for a new portal
func (p *portalPlannerImpl) planPortalChildResourcesCreate(
	ctx context.Context, plannerCtx *Config, desired resources.PortalResource, _ string, plan *Plan,
) {
	// Portal ID is not yet known, will be resolved at execution time
	// But we still need to plan child resources that depend on this portal

	// Get the main planner instance to access child resource planning methods
	planner := p.planner

	// Extract parent namespace for child resources
	parentNamespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		parentNamespace = *desired.Kongctl.Namespace
	}

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
	if err := planner.planPortalPagesChanges(ctx, parentNamespace, "", desired.Ref, pages, plan); err != nil {
		// Log error but don't fail - portal creation should still proceed
		planner.logger.Debug("Failed to plan portal pages for new portal",
			"portal", desired.Ref,
			"error", err.Error())
	}

	// Plan snippets
	snippets := make([]resources.PortalSnippetResource, 0)
	for _, snippet := range planner.desiredPortalSnippets {
		if snippet.Portal == desired.Ref {
			snippets = append(snippets, snippet)
		}
	}
	if err := planner.planPortalSnippetsChanges(ctx, parentNamespace, "", desired.Ref, snippets, plan); err != nil {
		planner.logger.Debug("Failed to plan portal snippets for new portal",
			"portal", desired.Ref,
			"error", err.Error())
	}

	// Plan customization
	customizations := make([]resources.PortalCustomizationResource, 0)
	for _, customization := range planner.desiredPortalCustomizations {
		if customization.Portal == desired.Ref {
			customizations = append(customizations, customization)
		}
	}
	if err := planner.planPortalCustomizationsChanges(ctx, plannerCtx, parentNamespace, customizations, plan); err != nil {
		planner.logger.Debug("Failed to plan portal customizations for new portal",
			"portal", desired.Ref,
			"error", err.Error())
	}

	// Plan custom domain
	domains := make([]resources.PortalCustomDomainResource, 0)
	for _, domain := range planner.desiredPortalCustomDomains {
		if domain.Portal == desired.Ref {
			domains = append(domains, domain)
		}
	}
	if err := planner.planPortalCustomDomainsChanges(ctx, parentNamespace, "", desired.Ref, domains, plan); err != nil {
		planner.logger.Debug("Failed to plan portal custom domains for new portal",
			"portal", desired.Ref,
			"error", err.Error())
	}
}

// planPortalChildResourceChanges plans changes for child resources of an existing portal
func (p *portalPlannerImpl) planPortalChildResourceChanges(
	ctx context.Context, plannerCtx *Config, current state.Portal, desired resources.PortalResource, plan *Plan,
) error {
	// Get the main planner instance to access child resource planning methods
	planner := p.planner

	// Extract parent namespace for child resources
	parentNamespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		parentNamespace = *desired.Kongctl.Namespace
	}

	// Plan pages - pass empty array if no pages defined
	pages := make([]resources.PortalPageResource, 0)
	// Note: Pages have already been extracted to root level by loader
	// We need to find pages that belong to this portal
	for _, page := range planner.desiredPortalPages {
		if page.Portal == desired.Ref {
			pages = append(pages, page)
		}
	}
	if err := planner.planPortalPagesChanges(ctx, parentNamespace, current.ID, desired.Ref, pages, plan); err != nil {
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
	if err := planner.planPortalSnippetsChanges(
		ctx, parentNamespace, current.ID, desired.Ref, snippets, plan,
	); err != nil {
		return fmt.Errorf("failed to plan portal snippet changes: %w", err)
	}

	// Plan customization (singleton resource)
	customizations := make([]resources.PortalCustomizationResource, 0)
	for _, customization := range planner.desiredPortalCustomizations {
		if customization.Portal == desired.Ref {
			customizations = append(customizations, customization)
		}
	}
	if err := planner.planPortalCustomizationsChanges(ctx, plannerCtx, parentNamespace, customizations, plan); err != nil {
		return fmt.Errorf("failed to plan portal customization changes: %w", err)
	}

	// Plan custom domain (singleton resource)
	domains := make([]resources.PortalCustomDomainResource, 0)
	for _, domain := range planner.desiredPortalCustomDomains {
		if domain.Portal == desired.Ref {
			domains = append(domains, domain)
		}
	}
	if err := planner.planPortalCustomDomainsChanges(
		ctx,
		parentNamespace,
		current.ID,
		desired.Ref,
		domains,
		plan,
	); err != nil {
		return fmt.Errorf("failed to plan portal custom domain changes: %w", err)
	}

	return nil
}
