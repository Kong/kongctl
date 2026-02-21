package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/util"
)

// OrganizationTeamPlannerImpl implements planning logic for organization teams
type OrganizationTeamPlannerImpl struct {
	*BasePlanner
}

// NewOrganizationTeamPlanner creates a new organization team planner
func NewOrganizationTeamPlanner(base *BasePlanner) OrganizationTeamPlanner {
	return &OrganizationTeamPlannerImpl{
		BasePlanner: base,
	}
}

func (t *OrganizationTeamPlannerImpl) PlannerComponent() string {
	return string(resources.ResourceTypeOrganizationTeam)
}

// PlanChanges generates changes for organization_team resources
func (t *OrganizationTeamPlannerImpl) PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error {
	namespace := plannerCtx.Namespace
	desired := t.GetDesiredOrganizationTeams(namespace)

	// Skip if no teams to plan and not in sync mode
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	var currentTeams []state.OrganizationTeam
	if namespace != resources.NamespaceExternal {
		namespaceFilter := []string{namespace}
		var err error
		currentTeams, err = t.GetClient().ListManagedOrganizationTeams(ctx, namespaceFilter)
		if err != nil {
			// If team client is not configured, skip team planning
			if err.Error() == "organization team API client not configured" {
				return nil
			}
			return fmt.Errorf("failed to list current organization teams in namespace %s: %w", namespace, err)
		}
	}

	// Index current teams by name
	currentByName := make(map[string]state.OrganizationTeam)
	for _, team := range currentTeams {
		if team.Name == nil || util.GetString(team.Name) == "" {
			continue
		}
		currentByName[util.GetString(team.Name)] = team
	}

	// Collect protection validation errors
	protectionErrors := &ProtectionErrorCollector{}

	// Compare each desired team
	for _, desiredTeam := range desired {
		// External teams are not managed by kongctl and exist in Konnect already.
		// We still plan their child resources based on the resolved Konnect ID when available.
		if desiredTeam.IsExternal() {
			t.planner.logger.Debug(
				"Skipping external organization team",
				"ref",
				desiredTeam.GetRef(),
				"name",
				desiredTeam.Name,
			)
			continue
		}

		current, exists := currentByName[desiredTeam.Name]
		if !exists {
			t.planTeamCreate(desiredTeam, plan)
			continue
		}

		// Check if update needed
		// Get protection status from desired configuration
		currentProtected := labels.IsProtectedResource(current.NormalizedLabels)
		desiredProtected := false
		if desiredTeam.Kongctl != nil && desiredTeam.Kongctl.Protected != nil && *desiredTeam.Kongctl.Protected {
			desiredProtected = true
		}

		// Handle protection changes
		if currentProtected != desiredProtected {
			// When changing protection status, include any other field updates too
			needsUpdate, updateFields := t.shouldUpdateOrganizationTeam(current, desiredTeam)

			// Create protection change object
			protectionChange := &ProtectionChange{
				Old: currentProtected,
				New: desiredProtected,
			}

			// Validate protection change
			err := t.ValidateProtectionWithChange("organization_team", desiredTeam.Name, currentProtected, ActionUpdate,
				protectionChange, needsUpdate)
			protectionErrors.Add(err)
			if err == nil {
				t.planOrganizationTeamProtectionChangeWithFields(current, desiredTeam, currentProtected,
					desiredProtected, updateFields, plan)
			}
		} else {
			// Check if update needed based on configuration
			needsUpdate, updateFields := t.shouldUpdateOrganizationTeam(current, desiredTeam)
			if needsUpdate {
				// Regular update - check protection
				err := t.ValidateProtection("organization_team", desiredTeam.Name, currentProtected, ActionUpdate)
				protectionErrors.Add(err)
				if err == nil {
					t.planOrganizationTeamUpdateWithFields(current, desiredTeam, updateFields, plan)
				}
			}
		}
	}

	// Check for managed resources to delete (sync mode only)
	if plan.Metadata.Mode == PlanModeSync {
		// Build set of desired team names
		desiredNames := make(map[string]bool)
		for _, team := range desired {
			desiredNames[team.Name] = true
		}

		// Find managed teams not in desired state
		for name, current := range currentByName {
			if !desiredNames[name] {
				// Validate protection before adding DELETE
				isProtected := labels.IsProtectedResource(current.NormalizedLabels)
				err := t.ValidateProtection("organization_team", name, isProtected, ActionDelete)
				protectionErrors.Add(err)
				if err == nil {
					t.planOrganizationTeamDelete(current, plan)
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

// extractTeamFields extracts fields from a organization_team resource for planner operations
func extractTeamFields(resource any) map[string]any {
	fields := make(map[string]any)
	team, ok := resource.(resources.OrganizationTeamResource)
	if !ok {
		return fields
	}

	fields["name"] = team.Name
	if team.Description != nil {
		fields["description"] = util.GetString(team.Description)
	}
	// Copy user-defined labels only (protection label will be added during execution)
	if len(team.GetLabels()) > 0 {
		fields["labels"] = team.GetLabels()
	}

	return fields
}

// planTeamCreate creates a CREATE change for a organization_team
func (t *OrganizationTeamPlannerImpl) planTeamCreate(team resources.OrganizationTeamResource, plan *Plan) string {
	generic := t.GetGenericPlanner()
	if generic == nil {
		// During tests, generic planner might not be initialized
		// Fall back to inline implementation
		changeID := t.NextChangeID(ActionCreate, "organization_team", team.GetRef())
		change := PlannedChange{
			ID:           changeID,
			ResourceType: "organization_team",
			ResourceRef:  team.GetRef(),
			Action:       ActionCreate,
			Fields:       extractTeamFields(team),
			DependsOn:    []string{},
			Namespace:    DefaultNamespace,
		}
		if team.Kongctl != nil && team.Kongctl.Protected != nil {
			change.Protection = *team.Kongctl.Protected
		}
		if team.Kongctl != nil && team.Kongctl.Namespace != nil {
			change.Namespace = *team.Kongctl.Namespace
		}

		plan.AddChange(change)
		return changeID
	}

	// Extract protection status
	var protection any
	if team.Kongctl != nil && team.Kongctl.Protected != nil {
		protection = *team.Kongctl.Protected
	}

	// Extract namespace
	namespace := DefaultNamespace
	if team.Kongctl != nil && team.Kongctl.Namespace != nil {
		namespace = *team.Kongctl.Namespace
	}

	config := CreateConfig{
		ResourceType:   "organization_team",
		ResourceName:   team.Name,
		ResourceRef:    team.GetRef(),
		RequiredFields: []string{"name"},
		FieldExtractor: func(_ any) map[string]any {
			return extractTeamFields(team)
		},
		Namespace: namespace,
		DependsOn: []string{},
	}

	change, err := generic.PlanCreate(context.Background(), config)
	if err != nil {
		// This shouldn't happen with valid configuration
		t.planner.logger.Error("Failed to plan organization_team create", "error", err.Error())
		return ""
	}

	// Set protection after creation
	change.Protection = protection

	plan.AddChange(change)
	return change.ID
}

// shouldUpdateOrganizationTeam checks if organization_team needs update based on configured fields only
func (t *OrganizationTeamPlannerImpl) shouldUpdateOrganizationTeam(
	current state.OrganizationTeam,
	desired resources.OrganizationTeamResource,
) (bool, map[string]any) {
	updates := make(map[string]any)

	if desired.Description != nil {
		currentDesc := t.GetString(current.Description)
		if currentDesc != *desired.Description {
			updates["description"] = *desired.Description
		}
	}

	// Check if labels are defined in the desired state
	// If labels are defined (even if empty), we need to send them to ensure proper replacement
	if desired.Labels != nil {
		if labels.CompareUserLabels(current.NormalizedLabels, desired.Labels) {
			// User labels differ, include all labels in update
			labelsMap := make(map[string]any)
			for k, v := range desired.Labels {
				labelsMap[k] = v
			}
			updates["labels"] = labelsMap
		}
	}

	return len(updates) > 0, updates
}

// planOrganizationTeamUpdateWithFields creates an UPDATE change with specific fields
func (t *OrganizationTeamPlannerImpl) planOrganizationTeamUpdateWithFields(
	current state.OrganizationTeam,
	desired resources.OrganizationTeamResource,
	updateFields map[string]any,
	plan *Plan,
) {
	// Always include name for identification
	updateFields["name"] = util.GetString(current.Name)

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
		ResourceType:   "organization_team",
		ResourceName:   desired.Name,
		ResourceRef:    desired.GetRef(),
		ResourceID:     util.GetString(current.ID),
		CurrentFields:  nil, // Not needed for direct update
		DesiredFields:  updateFields,
		RequiredFields: []string{"name"},
		Namespace:      namespace,
	}

	generic := t.GetGenericPlanner()
	if generic == nil {
		// During tests, generic planner might not be initialized
		// Fall back to inline implementation
		fields := make(map[string]any)
		fields["name"] = util.GetString(current.Name)
		for field, newValue := range updateFields {
			fields[field] = newValue
		}
		if _, hasLabels := updateFields["labels"]; hasLabels {
			fields[FieldCurrentLabels] = current.NormalizedLabels
		}

		changeID := t.NextChangeID(ActionUpdate, "organization_team", desired.GetRef())
		change := PlannedChange{
			ID:           changeID,
			ResourceType: "organization_team",
			ResourceRef:  desired.GetRef(),
			ResourceID:   util.GetString(current.ID),
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
		t.planner.logger.Error("Failed to plan organization_team update", "error", err.Error())
		return
	}

	// Check if already protected
	if labels.IsProtectedResource(current.NormalizedLabels) {
		change.Protection = true
	}

	plan.AddChange(change)
}

// planOrganizationTeamProtectionChangeWithFields creates an UPDATE for protection status with optional field updates
func (t *OrganizationTeamPlannerImpl) planOrganizationTeamProtectionChangeWithFields(
	current state.OrganizationTeam,
	desired resources.OrganizationTeamResource,
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
		ResourceType: "organization_team",
		ResourceName: desired.Name,
		ResourceRef:  desired.GetRef(),
		ResourceID:   util.GetString(current.ID),
		OldProtected: wasProtected,
		NewProtected: shouldProtect,
		Namespace:    namespace,
	}

	generic := t.GetGenericPlanner()
	var change PlannedChange
	if generic != nil {
		change = generic.PlanProtectionChange(context.Background(), config)
	} else {
		// Fallback for tests
		changeID := t.NextChangeID(ActionUpdate, "organization_team", desired.GetRef())
		change = PlannedChange{
			ID:           changeID,
			ResourceType: "organization_team",
			ResourceRef:  desired.GetRef(),
			ResourceID:   util.GetString(current.ID),
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
	fields["name"] = util.GetString(current.Name)

	// Include any field updates if unprotecting
	if wasProtected && !shouldProtect && len(updateFields) > 0 {
		for field, newValue := range updateFields {
			fields[field] = newValue
		}
	}

	change.Fields = fields
	plan.AddChange(change)
}

// planOrganizationTeamDelete creates a DELETE change for an organization_team
func (t *OrganizationTeamPlannerImpl) planOrganizationTeamDelete(team state.OrganizationTeam, plan *Plan) {
	// Extract namespace from labels (for existing resources being deleted)
	namespace := DefaultNamespace
	if ns, ok := team.NormalizedLabels[labels.NamespaceKey]; ok {
		namespace = ns
	}

	generic := t.GetGenericPlanner()
	var change PlannedChange

	if generic != nil {
		config := DeleteConfig{
			ResourceType: "organization_team",
			ResourceName: util.GetString(team.Name),
			ResourceRef:  util.GetString(team.Name),
			ResourceID:   util.GetString(team.ID),
			Namespace:    namespace,
		}
		change = generic.PlanDelete(context.Background(), config)
	} else {
		// Fallback for tests
		changeID := t.NextChangeID(ActionDelete, "organization_team", util.GetString(team.Name))
		change = PlannedChange{
			ID:           changeID,
			ResourceType: "organization_team",
			ResourceRef:  util.GetString(team.Name),
			ResourceID:   util.GetString(team.ID),
			Action:       ActionDelete,
			Namespace:    namespace,
		}
	}

	// Add the name field for backward compatibility
	change.Fields = map[string]any{"name": util.GetString(team.Name)}

	plan.AddChange(change)
}
