package planner

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
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

func (p *portalPlannerImpl) PlannerComponent() string {
	return string(resources.ResourceTypePortal)
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

	// Delete mode: plan DELETE for desired resources that exist, warn if not found
	if plan.Metadata.Mode == PlanModeDelete {
		return p.planPortalDeletes(ctx, plannerCtx, desired, plan)
	}

	var currentPortals []state.Portal
	if namespace != resources.NamespaceExternal {
		namespaceFilter := []string{namespace}
		var err error
		currentPortals, err = p.planner.listManagedPortals(ctx, namespaceFilter)
		if err != nil {
			// If portal client is not configured, skip portal planning
			if err.Error() == "Portal client not configured" {
				return nil
			}
			return fmt.Errorf("failed to list current portals in namespace %s: %w", namespace, err)
		}
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
			// If we have a resolved Konnect ID, plan full child diffs.
			if portalID := desiredPortal.GetKonnectID(); portalID != "" {
				// Build a minimal current portal for child planning
				current := state.Portal{
					ListPortalsResponsePortal: kkComps.ListPortalsResponsePortal{
						ID:   portalID,
						Name: desiredPortal.Name,
					},
					NormalizedLabels: map[string]string{},
				}
				p.planner.logger.Debug(
					"Planning children for external portal",
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
				if err := p.planPortalChildResourcesCreate(ctx, plannerCtx, desiredPortal, "", plan); err != nil {
					return err
				}
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
			if err := p.planPortalChildResourcesCreate(ctx, plannerCtx, desiredPortal, portalChangeID, plan); err != nil {
				return err
			}
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
				needsUpdate, updateFields, changedFields := p.shouldUpdatePortal(current, desiredPortal)

				// Create protection change object
				protectionChange := &ProtectionChange{
					Old: isProtected,
					New: shouldProtect,
				}

				// Validate protection change
				err := p.ValidateProtectionWithChange(ResourceTypePortal, desiredPortal.Name, isProtected, ActionUpdate,
					protectionChange, needsUpdate)
				protectionErrors.Add(err)
				if err == nil {
					p.planPortalProtectionChangeWithFields(
						current,
						desiredPortal,
						isProtected,
						shouldProtect,
						updateFields,
						changedFields,
						plan,
					)
				}
			} else {
				// Check if update needed based on configuration
				needsUpdate, updateFields, changedFields := p.shouldUpdatePortal(current, desiredPortal)
				if needsUpdate {
					// Regular update - check protection
					err := p.ValidateProtection(ResourceTypePortal, desiredPortal.Name, isProtected, ActionUpdate)
					protectionErrors.Add(err)
					if err == nil {
						p.planPortalUpdateWithFields(current, desiredPortal, updateFields, changedFields, plan)
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
				err := p.ValidateProtection(ResourceTypePortal, name, isProtected, ActionDelete)
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

// planPortalDeletes handles delete mode by planning DELETE for desired portals that exist in Konnect.
func (p *portalPlannerImpl) planPortalDeletes(
	ctx context.Context, plannerCtx *Config, desired []resources.PortalResource, plan *Plan,
) error {
	namespace := plannerCtx.Namespace

	var currentPortals []state.Portal
	if namespace != resources.NamespaceExternal {
		namespaceFilter := []string{namespace}
		var err error
		currentPortals, err = p.GetClient().ListManagedPortals(ctx, namespaceFilter)
		if err != nil {
			if err.Error() == "Portal client not configured" {
				return nil
			}
			return fmt.Errorf("failed to list current portals in namespace %s: %w", namespace, err)
		}
	}

	currentByName := make(map[string]state.Portal)
	for _, portal := range currentPortals {
		currentByName[portal.GetName()] = portal
	}

	protectionErrors := &ProtectionErrorCollector{}

	for _, desiredPortal := range desired {
		// External portals are not deleted, but their explicitly declared pages are.
		if desiredPortal.IsExternal() {
			portalID := desiredPortal.GetKonnectID()
			if portalID == "" {
				return fmt.Errorf("external portal %q has no resolved ID", desiredPortal.GetRef())
			}
			pages := make([]resources.PortalPageResource, 0)
			for _, page := range p.planner.desiredPortalPages {
				if page.Portal == desiredPortal.Ref {
					pages = append(pages, page)
				}
			}
			if len(pages) > 0 {
				parentNamespace := DefaultNamespace
				if desiredPortal.Kongctl != nil && desiredPortal.Kongctl.Namespace != nil {
					parentNamespace = *desiredPortal.Kongctl.Namespace
				}
				if err := p.planner.planExternalPortalPageDeletes(
					ctx,
					parentNamespace,
					portalID,
					desiredPortal.Ref,
					pages,
					plan,
				); err != nil {
					return fmt.Errorf("failed to plan external portal page deletes: %w", err)
				}
			}
			continue
		}

		current, exists := currentByName[desiredPortal.Name]
		if !exists {
			plan.AddWarning("", fmt.Sprintf(
				"portal %q not found in Konnect, skipping delete", desiredPortal.Name,
			))
			continue
		}

		isProtected := labels.IsProtectedResource(current.NormalizedLabels)
		err := p.ValidateProtection(ResourceTypePortal, desiredPortal.Name, isProtected, ActionDelete)
		protectionErrors.Add(err)
		if err == nil {
			p.planPortalDelete(current, plan)
		}
	}

	if protectionErrors.HasErrors() {
		return protectionErrors.Error()
	}

	return nil
}

// extractPortalFields extracts fields from a portal resource for planner operations
func extractPortalFields(resource any) map[string]any {
	fields := make(map[string]any)

	portal, ok := resource.(resources.PortalResource)
	if !ok {
		return fields
	}

	fields[FieldName] = portal.Name
	if portal.DisplayName != nil {
		fields[FieldDisplayName] = *portal.DisplayName
	}
	if portal.Description != nil {
		fields[FieldDescription] = *portal.Description
	}
	if portal.AuthenticationEnabled != nil {
		fields[FieldAuthenticationEnabled] = *portal.AuthenticationEnabled
	}
	if portal.RbacEnabled != nil {
		fields[FieldRBACEnabled] = *portal.RbacEnabled
	}
	if portal.SiprEnabled != nil {
		fields[FieldSIPREnabled] = *portal.SiprEnabled
	}
	if portal.DefaultAPIVisibility != nil {
		fields[FieldDefaultAPIVisibility] = string(*portal.DefaultAPIVisibility)
	}
	if portal.DefaultPageVisibility != nil {
		fields[FieldDefaultPageVisibility] = string(*portal.DefaultPageVisibility)
	}
	if portal.DefaultApplicationAuthStrategyID != nil {
		fields[FieldDefaultApplicationStrategyID] = *portal.DefaultApplicationAuthStrategyID
	}
	if portal.AutoApproveDevelopers != nil {
		fields[FieldAutoApproveDevelopers] = *portal.AutoApproveDevelopers
	}
	if portal.AutoApproveApplications != nil {
		fields[FieldAutoApproveApplications] = *portal.AutoApproveApplications
	}

	// Copy user-defined labels only (protection label will be added during execution)
	if len(portal.Labels) > 0 {
		labelsMap := make(map[string]any)
		for k, v := range portal.Labels {
			if v != nil {
				labelsMap[k] = *v
			}
		}
		fields[FieldLabels] = labelsMap
	}

	return fields
}

// planPortalCreate creates a CREATE change for a portal
func (p *portalPlannerImpl) planPortalCreate(portal resources.PortalResource, plan *Plan) string {
	generic := p.GetGenericPlanner()
	if generic == nil {
		// During tests, generic planner might not be initialized
		// Fall back to inline implementation
		changeID := p.NextChangeID(ActionCreate, ResourceTypePortal, portal.GetRef())
		change := PlannedChange{
			ID:           changeID,
			ResourceType: ResourceTypePortal,
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
		ResourceType:   ResourceTypePortal,
		ResourceName:   portal.Name,
		ResourceRef:    portal.GetRef(),
		RequiredFields: []string{FieldName},
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
		change.References[FieldDefaultApplicationStrategyID] = ReferenceInfo{
			Ref: authStrategyValue, // Keep full placeholder for later parsing
			ID:  "",                // Will be resolved during execution
			LookupFields: map[string]string{
				FieldName: parsedRef, // Use ref as name for lookup
			},
		}

		p.planner.logger.Debug("Added auth strategy reference to portal",
			"portal_ref", portal.GetRef(),
			"auth_strategy_ref", parsedRef,
			"field_requested", field)
	}
}

func planOptionalBoolChange(
	desired *bool,
	current *bool,
	fieldName string,
	updates map[string]any,
	changedFields map[string]FieldChange,
) {
	if desired == nil {
		return
	}
	if current == nil || *current != *desired {
		updates[fieldName] = *desired
		var oldValue any
		if current != nil {
			oldValue = *current
		}
		changedFields[fieldName] = FieldChange{Old: oldValue, New: *desired}
	}
}

// shouldUpdatePortal checks if portal needs update based on configured fields only
func (p *portalPlannerImpl) shouldUpdatePortal(
	current state.Portal,
	desired resources.PortalResource,
) (bool, map[string]any, map[string]FieldChange) {
	updates := make(map[string]any)
	changedFields := make(map[string]FieldChange)

	// Only compare fields present in desired configuration
	if desired.DisplayName != nil {
		if current.DisplayName != *desired.DisplayName {
			updates[FieldDisplayName] = *desired.DisplayName
			changedFields[FieldDisplayName] = FieldChange{
				Old: current.DisplayName,
				New: *desired.DisplayName,
			}
		}
	}
	if desired.Description != nil {
		currentDesc := p.GetString(current.Description)
		if currentDesc != *desired.Description {
			updates[FieldDescription] = *desired.Description
			changedFields[FieldDescription] = FieldChange{
				Old: currentDesc,
				New: *desired.Description,
			}
		}
	}

	if desired.DefaultApplicationAuthStrategyID != nil {
		desiredValue := *desired.DefaultApplicationAuthStrategyID

		// Skip comparison if desired value is a reference placeholder
		// The executor will resolve it and we trust it matches what's in Konnect
		if !strings.HasPrefix(desiredValue, tags.RefPlaceholderPrefix) {
			currentAuthID := p.GetString(current.DefaultApplicationAuthStrategyID)
			if currentAuthID != desiredValue {
				updates[FieldDefaultApplicationStrategyID] = desiredValue
				changedFields[FieldDefaultApplicationStrategyID] = FieldChange{
					Old: currentAuthID,
					New: desiredValue,
				}
			}
		}
	}

	planOptionalBoolChange(
		desired.AuthenticationEnabled,
		current.GetAuthenticationEnabled(),
		FieldAuthenticationEnabled,
		updates,
		changedFields,
	)
	planOptionalBoolChange(desired.RbacEnabled, current.GetRbacEnabled(), FieldRBACEnabled, updates, changedFields)
	planOptionalBoolChange(desired.SiprEnabled, current.GetSiprEnabled(), FieldSIPREnabled, updates, changedFields)
	planOptionalBoolChange(
		desired.AutoApproveDevelopers,
		current.GetAutoApproveDevelopers(),
		FieldAutoApproveDevelopers,
		updates,
		changedFields,
	)
	planOptionalBoolChange(
		desired.AutoApproveApplications,
		current.GetAutoApproveApplications(),
		FieldAutoApproveApplications,
		updates,
		changedFields,
	)

	if desired.DefaultAPIVisibility != nil {
		currentVisibility := string(current.DefaultAPIVisibility)
		desiredVisibility := string(*desired.DefaultAPIVisibility)
		if currentVisibility != desiredVisibility {
			updates[FieldDefaultAPIVisibility] = desiredVisibility
			changedFields[FieldDefaultAPIVisibility] = FieldChange{
				Old: currentVisibility,
				New: desiredVisibility,
			}
		}
	}

	if desired.DefaultPageVisibility != nil {
		currentVisibility := string(current.DefaultPageVisibility)
		desiredVisibility := string(*desired.DefaultPageVisibility)
		if currentVisibility != desiredVisibility {
			updates[FieldDefaultPageVisibility] = desiredVisibility
			changedFields[FieldDefaultPageVisibility] = FieldChange{
				Old: currentVisibility,
				New: desiredVisibility,
			}
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
			updates[FieldLabels] = labelsMap
			changedFields[FieldLabels] = FieldChange{
				Old: labels.GetUserLabels(current.NormalizedLabels),
				New: desiredLabels,
			}
		}
	}

	return len(updates) > 0, updates, changedFields
}

// planPortalUpdateWithFields creates an UPDATE change with specific fields
func (p *portalPlannerImpl) planPortalUpdateWithFields(
	current state.Portal,
	desired resources.PortalResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	// Always include name for identification
	updateFields[FieldName] = current.Name

	// Pass current labels so executor can properly handle removals
	if _, hasLabels := updateFields[FieldLabels]; hasLabels {
		updateFields[FieldCurrentLabels] = current.NormalizedLabels
	}

	// Extract namespace
	namespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		namespace = *desired.Kongctl.Namespace
	}

	config := UpdateConfig{
		ResourceType:   ResourceTypePortal,
		ResourceName:   desired.Name,
		ResourceRef:    desired.GetRef(),
		ResourceID:     current.ID,
		CurrentFields:  nil, // Not needed for direct update
		DesiredFields:  updateFields,
		ChangedFields:  changedFields,
		RequiredFields: []string{FieldName},
		Namespace:      namespace,
	}

	generic := p.GetGenericPlanner()
	if generic == nil {
		// During tests, generic planner might not be initialized
		// Fall back to inline implementation
		fields := make(map[string]any)
		fields[FieldName] = current.Name
		maps.Copy(fields, updateFields)
		if _, hasLabels := updateFields[FieldLabels]; hasLabels {
			fields[FieldCurrentLabels] = current.NormalizedLabels
		}

		changeID := p.NextChangeID(ActionUpdate, ResourceTypePortal, desired.GetRef())
		change := PlannedChange{
			ID:            changeID,
			ResourceType:  ResourceTypePortal,
			ResourceRef:   desired.GetRef(),
			ResourceID:    current.ID,
			Action:        ActionUpdate,
			Fields:        fields,
			ChangedFields: changedFields,
			DependsOn:     []string{},
			Namespace:     DefaultNamespace,
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
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	// Extract namespace
	namespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		namespace = *desired.Kongctl.Namespace
	}

	// Use generic protection change planner
	config := ProtectionChangeConfig{
		ResourceType: ResourceTypePortal,
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
		changeID := p.NextChangeID(ActionUpdate, ResourceTypePortal, desired.GetRef())
		change = PlannedChange{
			ID:           changeID,
			ResourceType: ResourceTypePortal,
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
	fields[FieldName] = current.Name

	// Include any field updates if unprotecting
	if wasProtected && !shouldProtect && len(updateFields) > 0 {
		maps.Copy(fields, updateFields)
	}

	change.Fields = fields
	if len(changedFields) > 0 {
		change.ChangedFields = changedFields
	}
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
			ResourceType: ResourceTypePortal,
			ResourceName: portal.Name,
			ResourceRef:  portal.Name,
			ResourceID:   portal.ID,
			Namespace:    namespace,
		}
		change = generic.PlanDelete(context.Background(), config)
	} else {
		// Fallback for tests
		changeID := p.NextChangeID(ActionDelete, ResourceTypePortal, portal.Name)
		change = PlannedChange{
			ID:           changeID,
			ResourceType: ResourceTypePortal,
			ResourceRef:  portal.Name,
			ResourceID:   portal.ID,
			Action:       ActionDelete,
			Namespace:    namespace,
		}
	}

	// Add the name field for backward compatibility
	change.Fields = map[string]any{FieldName: portal.Name}

	plan.AddChange(change)
}

// planPortalChildResourcesCreate plans children without observing the parent.
func (p *portalPlannerImpl) planPortalChildResourcesCreate(
	ctx context.Context, plannerCtx *Config, desired resources.PortalResource, _ string, plan *Plan,
) error {
	return p.planPortalChildren(ctx, plannerCtx, desired, "", true, plan)
}

func (p *portalPlannerImpl) planPortalChildResourceChanges(
	ctx context.Context, plannerCtx *Config, current state.Portal, desired resources.PortalResource, plan *Plan,
) error {
	return p.planPortalChildren(ctx, plannerCtx, desired, current.ID, false, plan)
}
