package planner

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
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
		currentTeams, err = t.planner.listManagedOrganizationTeams(ctx, namespaceFilter)
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

	// Handle delete mode - plan DELETE for desired resources that exist in Konnect
	if plan.Metadata.Mode == PlanModeDelete {
		for _, desiredTeam := range desired {
			// External teams are not managed by kongctl - skip
			if desiredTeam.IsExternal() {
				continue
			}

			current, exists := currentByName[desiredTeam.Name]
			if !exists {
				plan.AddWarning("", fmt.Sprintf(
					"organization_team %q not found in Konnect, skipping delete", desiredTeam.Name))
				continue
			}

			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			err := t.ValidateProtection(ResourceTypeOrganizationTeam, desiredTeam.Name, isProtected, ActionDelete)
			protectionErrors.Add(err)
			if err == nil {
				if err := t.planOrganizationTeamRoleDeletesForDesired(ctx, namespace, desiredTeam, current, plan); err != nil {
					return err
				}
				t.planOrganizationTeamDelete(current, plan)
			}
		}

		if protectionErrors.HasErrors() {
			return protectionErrors.Error()
		}
		return nil
	}

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
			needsUpdate, updateFields, changedFields := t.shouldUpdateOrganizationTeam(current, desiredTeam)

			// Create protection change object
			protectionChange := &ProtectionChange{
				Old: currentProtected,
				New: desiredProtected,
			}

			// Validate protection change
			err := t.ValidateProtectionWithChange(ResourceTypeOrganizationTeam, desiredTeam.Name, currentProtected, ActionUpdate,
				protectionChange, needsUpdate)
			protectionErrors.Add(err)
			if err == nil {
				t.planOrganizationTeamProtectionChangeWithFields(
					current,
					desiredTeam,
					currentProtected,
					desiredProtected,
					updateFields,
					changedFields,
					plan,
				)
			}
		} else {
			// Check if update needed based on configuration
			needsUpdate, updateFields, changedFields := t.shouldUpdateOrganizationTeam(current, desiredTeam)
			if needsUpdate {
				// Regular update - check protection
				err := t.ValidateProtection(ResourceTypeOrganizationTeam, desiredTeam.Name, currentProtected, ActionUpdate)
				protectionErrors.Add(err)
				if err == nil {
					t.planOrganizationTeamUpdateWithFields(current, desiredTeam, updateFields, changedFields, plan)
				}
			}
		}
	}

	if err := t.planOrganizationTeamRoleChanges(ctx, namespace, desired, currentByName, plan); err != nil {
		return err
	}
	if err := t.planOrganizationUserAssignmentChanges(ctx, namespace, desired, currentByName, plan); err != nil {
		return err
	}
	if err := t.planOrganizationSystemAccountAssignmentChanges(ctx, namespace, desired, currentByName, plan); err != nil {
		return err
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
				err := t.ValidateProtection(ResourceTypeOrganizationTeam, name, isProtected, ActionDelete)
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

func (t *OrganizationTeamPlannerImpl) planOrganizationTeamRoleDeletesForDesired(
	ctx context.Context,
	namespace string,
	team resources.OrganizationTeamResource,
	current state.OrganizationTeam,
	plan *Plan,
) error {
	teamID := util.GetString(current.ID)
	if teamID == "" {
		return nil
	}

	var desiredRoles []resources.OrganizationTeamRoleResource
	for _, role := range t.GetDesiredOrganizationTeamRoles(namespace) {
		if role.Team == team.Ref {
			desiredRoles = append(desiredRoles, role)
		}
	}
	if len(desiredRoles) == 0 {
		return nil
	}

	currentRoles, err := t.planner.client.ListOrganizationTeamRoles(ctx, teamID)
	if err != nil {
		if state.IsAPIClientError(err) {
			return nil
		}
		return fmt.Errorf("failed to list organization team roles for team %s: %w", teamID, err)
	}

	currentByKey := make(map[string]state.OrganizationTeamRole)
	for _, role := range currentRoles {
		key := buildOrganizationTeamRoleKey(role.RoleName, role.EntityID, role.EntityTypeName, role.EntityRegion)
		currentByKey[key] = role
	}
	for _, desiredRole := range desiredRoles {
		key := buildOrganizationTeamRoleKey(
			desiredRole.RoleName,
			t.resolveOrganizationTeamRoleEntityID(desiredRole.EntityID),
			desiredRole.EntityTypeName,
			desiredRole.EntityRegion,
		)
		if currentRole, ok := currentByKey[key]; ok {
			t.planOrganizationTeamRoleDelete(namespace, team.Ref, team.Name, teamID, currentRole, plan)
		}
	}

	return nil
}

func (t *OrganizationTeamPlannerImpl) planOrganizationTeamRoleChanges(
	ctx context.Context,
	namespace string,
	desiredTeams []resources.OrganizationTeamResource,
	currentByName map[string]state.OrganizationTeam,
	plan *Plan,
) error {
	desiredRoles := t.GetDesiredOrganizationTeamRoles(namespace)
	if len(desiredRoles) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	rolesByTeam := make(map[string][]resources.OrganizationTeamRoleResource)
	for _, role := range desiredRoles {
		rolesByTeam[role.Team] = append(rolesByTeam[role.Team], role)
	}

	teamByRef := make(map[string]resources.OrganizationTeamResource)
	for _, team := range desiredTeams {
		teamByRef[team.Ref] = team
		if plan.Metadata.Mode == PlanModeSync && !team.IsExternal() {
			if _, ok := rolesByTeam[team.Ref]; !ok {
				rolesByTeam[team.Ref] = []resources.OrganizationTeamRoleResource{}
			}
		}
	}

	teamsDeleted := make(map[string]bool)
	for _, change := range plan.Changes {
		if change.ResourceType == ResourceTypeOrganizationTeam && change.Action == ActionDelete {
			teamsDeleted[change.ResourceRef] = true
		}
	}

	for teamRef, roles := range rolesByTeam {
		if teamsDeleted[teamRef] {
			continue
		}

		team, ok := teamByRef[teamRef]
		if !ok {
			continue
		}

		teamName := team.Name
		if team.IsExternal() && team.External.Selector != nil {
			if selectorName := team.External.Selector.MatchFields[FieldName]; selectorName != "" {
				teamName = selectorName
			}
		}

		teamID := ""
		if current, exists := currentByName[teamName]; exists {
			teamID = util.GetString(current.ID)
		}
		if teamID == "" && team.GetKonnectID() != "" {
			teamID = team.GetKonnectID()
		}
		if teamID == "" && team.IsExternal() && team.External.ID != "" {
			teamID = team.External.ID
		}

		existingRoles := make(map[string]state.OrganizationTeamRole)
		if teamID != "" {
			currentRoles, err := t.planner.client.ListOrganizationTeamRoles(ctx, teamID)
			if err != nil {
				if state.IsAPIClientError(err) {
					return nil
				}
				return fmt.Errorf("failed to list organization team roles for team %s: %w", teamID, err)
			}
			for _, role := range currentRoles {
				key := buildOrganizationTeamRoleKey(role.RoleName, role.EntityID, role.EntityTypeName, role.EntityRegion)
				existingRoles[key] = role
			}
		}

		desiredKeys := make(map[string]bool)
		for _, role := range roles {
			key := buildOrganizationTeamRoleKey(
				role.RoleName,
				t.resolveOrganizationTeamRoleEntityID(role.EntityID),
				role.EntityTypeName,
				role.EntityRegion,
			)
			if desiredKeys[key] {
				return fmt.Errorf("duplicate organization team role assignment %q for team %q", key, teamRef)
			}
			desiredKeys[key] = true

			if _, exists := existingRoles[key]; exists {
				continue
			}

			t.planOrganizationTeamRoleCreate(namespace, teamRef, teamName, teamID, role, plan)
		}

		if plan.Metadata.Mode == PlanModeSync && teamID != "" && !team.IsExternal() {
			for key, existingRole := range existingRoles {
				if !desiredKeys[key] {
					t.planOrganizationTeamRoleDelete(namespace, teamRef, teamName, teamID, existingRole, plan)
				}
			}
		}
	}

	return nil
}

func (t *OrganizationTeamPlannerImpl) planOrganizationTeamRoleCreate(
	namespace string,
	teamRef string,
	teamName string,
	teamID string,
	role resources.OrganizationTeamRoleResource,
	plan *Plan,
) {
	dependencies := []string{}
	if teamChangeID := findChangeID(plan, ResourceTypeOrganizationTeam, teamRef); teamChangeID != "" {
		dependencies = append(dependencies, teamChangeID)
	}
	if tags.IsRefPlaceholder(role.EntityID) {
		if apiRef, _, ok := tags.ParseRefPlaceholder(role.EntityID); ok {
			if apiChangeID := findChangeID(plan, string(resources.ResourceTypeAPI), apiRef); apiChangeID != "" {
				dependencies = append(dependencies, apiChangeID)
			}
		}
	}

	refs := map[string]ReferenceInfo{
		FieldTeamID: {
			Ref: role.Team,
			LookupFields: map[string]string{
				FieldName: teamName,
			},
		},
	}
	if teamID != "" {
		refs[FieldTeamID] = ReferenceInfo{
			Ref: role.Team,
			ID:  teamID,
			LookupFields: map[string]string{
				FieldName: teamName,
			},
		}
	}
	if tags.IsRefPlaceholder(role.EntityID) {
		refs[FieldEntityID] = ReferenceInfo{Ref: role.EntityID}
	}

	change := PlannedChange{
		ID:           t.planner.nextChangeID(ActionCreate, ResourceTypeOrganizationTeamRole, role.GetRef()),
		ResourceType: ResourceTypeOrganizationTeamRole,
		ResourceRef:  role.GetRef(),
		Action:       ActionCreate,
		Fields: map[string]any{
			FieldRoleName:       role.RoleName,
			FieldEntityID:       role.EntityID,
			FieldEntityTypeName: role.EntityTypeName,
			FieldEntityRegion:   role.EntityRegion,
		},
		DependsOn:  dependencies,
		Namespace:  namespace,
		References: refs,
	}
	plan.AddChange(change)
}

func (t *OrganizationTeamPlannerImpl) planOrganizationTeamRoleDelete(
	namespace string,
	teamRef string,
	teamName string,
	teamID string,
	role state.OrganizationTeamRole,
	plan *Plan,
) {
	roleRef := buildOrganizationTeamRoleDeleteRef(teamRef, role)
	change := PlannedChange{
		ID:           t.planner.nextChangeID(ActionDelete, ResourceTypeOrganizationTeamRole, roleRef),
		ResourceType: ResourceTypeOrganizationTeamRole,
		ResourceRef:  roleRef,
		ResourceID:   role.ID,
		Action:       ActionDelete,
		Fields: map[string]any{
			FieldRoleName:       role.RoleName,
			FieldEntityID:       role.EntityID,
			FieldEntityTypeName: role.EntityTypeName,
			FieldEntityRegion:   role.EntityRegion,
		},
		Namespace: namespace,
		References: map[string]ReferenceInfo{
			FieldTeamID: {
				Ref: teamRef,
				ID:  teamID,
				LookupFields: map[string]string{
					FieldName: teamName,
				},
			},
		},
		Parent: &ParentInfo{
			Ref: teamRef,
			ID:  teamID,
		},
	}
	plan.AddChange(change)
}

func (t *OrganizationTeamPlannerImpl) resolveOrganizationTeamRoleEntityID(entityID string) string {
	if t.planner.resources == nil || !tags.IsRefPlaceholder(entityID) {
		return entityID
	}

	ref, field, ok := tags.ParseRefPlaceholder(entityID)
	if !ok || (field != "" && field != FieldID && field != "ID") {
		return entityID
	}

	if api := t.planner.resources.GetAPIByRef(ref); api != nil && api.GetKonnectID() != "" {
		return api.GetKonnectID()
	}

	return entityID
}

func buildOrganizationTeamRoleKey(roleName, entityID, entityTypeName, entityRegion string) string {
	return fmt.Sprintf("%s|%s|%s|%s", roleName, entityID, entityTypeName, strings.ToLower(entityRegion))
}

func buildOrganizationTeamRoleDeleteRef(teamRef string, role state.OrganizationTeamRole) string {
	roleKey := buildOrganizationTeamRoleKey(role.RoleName, role.EntityID, role.EntityTypeName, role.EntityRegion)
	if teamRef == "" {
		return roleKey
	}
	return fmt.Sprintf("%s|%s", teamRef, roleKey)
}

// extractTeamFields extracts fields from a organization_team resource for planner operations
func extractTeamFields(resource any) map[string]any {
	fields := make(map[string]any)
	team, ok := resource.(resources.OrganizationTeamResource)
	if !ok {
		return fields
	}

	fields[FieldName] = team.Name
	if team.Description != nil {
		fields[FieldDescription] = util.GetString(team.Description)
	}
	// Copy user-defined labels only (protection label will be added during execution)
	if len(team.GetLabels()) > 0 {
		fields[FieldLabels] = team.GetLabels()
	}

	return fields
}

// planTeamCreate creates a CREATE change for a organization_team
func (t *OrganizationTeamPlannerImpl) planTeamCreate(team resources.OrganizationTeamResource, plan *Plan) string {
	generic := t.GetGenericPlanner()
	if generic == nil {
		// During tests, generic planner might not be initialized
		// Fall back to inline implementation
		changeID := t.NextChangeID(ActionCreate, ResourceTypeOrganizationTeam, team.GetRef())
		change := PlannedChange{
			ID:           changeID,
			ResourceType: ResourceTypeOrganizationTeam,
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
		ResourceType:   ResourceTypeOrganizationTeam,
		ResourceName:   team.Name,
		ResourceRef:    team.GetRef(),
		RequiredFields: []string{FieldName},
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
) (bool, map[string]any, map[string]FieldChange) {
	updates := make(map[string]any)
	changedFields := make(map[string]FieldChange)

	if desired.Description != nil {
		currentDesc := t.GetString(current.Description)
		if currentDesc != *desired.Description {
			updates[FieldDescription] = *desired.Description
			changedFields[FieldDescription] = FieldChange{
				Old: currentDesc,
				New: *desired.Description,
			}
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
			updates[FieldLabels] = labelsMap
			changedFields[FieldLabels] = FieldChange{
				Old: labels.GetUserLabels(current.NormalizedLabels),
				New: labels.GetUserLabels(desired.Labels),
			}
		}
	}

	return len(updates) > 0, updates, changedFields
}

// planOrganizationTeamUpdateWithFields creates an UPDATE change with specific fields
func (t *OrganizationTeamPlannerImpl) planOrganizationTeamUpdateWithFields(
	current state.OrganizationTeam,
	desired resources.OrganizationTeamResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	// Always include name for identification
	updateFields[FieldName] = util.GetString(current.Name)

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
		ResourceType:   ResourceTypeOrganizationTeam,
		ResourceName:   desired.Name,
		ResourceRef:    desired.GetRef(),
		ResourceID:     util.GetString(current.ID),
		CurrentFields:  nil, // Not needed for direct update
		DesiredFields:  updateFields,
		ChangedFields:  changedFields,
		RequiredFields: []string{FieldName},
		Namespace:      namespace,
	}

	generic := t.GetGenericPlanner()
	if generic == nil {
		// During tests, generic planner might not be initialized
		// Fall back to inline implementation
		fields := make(map[string]any)
		fields[FieldName] = util.GetString(current.Name)
		maps.Copy(fields, updateFields)
		if _, hasLabels := updateFields[FieldLabels]; hasLabels {
			fields[FieldCurrentLabels] = current.NormalizedLabels
		}

		changeID := t.NextChangeID(ActionUpdate, ResourceTypeOrganizationTeam, desired.GetRef())
		change := PlannedChange{
			ID:            changeID,
			ResourceType:  ResourceTypeOrganizationTeam,
			ResourceRef:   desired.GetRef(),
			ResourceID:    util.GetString(current.ID),
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
		ResourceType: ResourceTypeOrganizationTeam,
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
		changeID := t.NextChangeID(ActionUpdate, ResourceTypeOrganizationTeam, desired.GetRef())
		change = PlannedChange{
			ID:           changeID,
			ResourceType: ResourceTypeOrganizationTeam,
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
	fields[FieldName] = util.GetString(current.Name)

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
			ResourceType: ResourceTypeOrganizationTeam,
			ResourceName: util.GetString(team.Name),
			ResourceRef:  util.GetString(team.Name),
			ResourceID:   util.GetString(team.ID),
			Namespace:    namespace,
		}
		change = generic.PlanDelete(context.Background(), config)
	} else {
		// Fallback for tests
		changeID := t.NextChangeID(ActionDelete, ResourceTypeOrganizationTeam, util.GetString(team.Name))
		change = PlannedChange{
			ID:           changeID,
			ResourceType: ResourceTypeOrganizationTeam,
			ResourceRef:  util.GetString(team.Name),
			ResourceID:   util.GetString(team.ID),
			Action:       ActionDelete,
			Namespace:    namespace,
		}
	}

	// Add the name field for backward compatibility
	change.Fields = map[string]any{FieldName: util.GetString(team.Name)}

	plan.AddChange(change)
}
