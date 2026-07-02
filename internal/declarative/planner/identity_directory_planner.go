package planner

import (
	"context"
	"fmt"
	"slices"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

type identityDirectoryPlannerImpl struct {
	*BasePlanner
}

func NewIdentityDirectoryPlanner(base *BasePlanner) IdentityDirectoryPlanner {
	return &identityDirectoryPlannerImpl{BasePlanner: base}
}

func (p *identityDirectoryPlannerImpl) PlannerComponent() string {
	return string(resources.ResourceTypeIdentityDirectory)
}

func (p *identityDirectoryPlannerImpl) PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error {
	namespace := plannerCtx.Namespace
	desired := p.GetDesiredIdentityDirectories(namespace)

	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	currentDirectories, err := p.planner.listManagedIdentityDirectories(ctx, []string{namespace})
	if err != nil {
		if state.IsAPIClientError(err) {
			return nil
		}
		return fmt.Errorf("failed to list current identity directories: %w", err)
	}

	currentByName := make(map[string]state.IdentityDirectory)
	for _, directory := range currentDirectories {
		currentByName[directory.Name] = directory
	}

	protectionErrors := &ProtectionErrorCollector{}

	if plan.Metadata.Mode == PlanModeDelete {
		for _, desiredDirectory := range desired {
			current, exists := currentByName[desiredDirectory.Name]
			if !exists {
				plan.AddWarning(
					"",
					fmt.Sprintf("identity_directory %q not found in Konnect, skipping delete", desiredDirectory.Name),
				)
				continue
			}

			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			err := p.ValidateProtection(
				ResourceTypeIdentityDirectory,
				desiredDirectory.Name,
				isProtected,
				ActionDelete,
			)
			protectionErrors.Add(err)
			if err == nil {
				p.planIdentityDirectoryDelete(current, plan)
			}
		}

		if protectionErrors.HasErrors() {
			return protectionErrors.Error()
		}
		return nil
	}

	for _, desiredDirectory := range desired {
		current, exists := currentByName[desiredDirectory.Name]
		if !exists {
			p.planIdentityDirectoryCreate(desiredDirectory, plan)
			continue
		}

		isProtected := labels.IsProtectedResource(current.NormalizedLabels)
		shouldProtect := false
		if desiredDirectory.Kongctl != nil &&
			desiredDirectory.Kongctl.Protected != nil &&
			*desiredDirectory.Kongctl.Protected {
			shouldProtect = true
		}

		needsUpdate, updateFields, changedFields := p.shouldUpdateIdentityDirectory(current, desiredDirectory)
		if isProtected != shouldProtect {
			protectionChange := &ProtectionChange{Old: isProtected, New: shouldProtect}
			err := p.ValidateProtectionWithChange(
				ResourceTypeIdentityDirectory,
				desiredDirectory.Name,
				isProtected,
				ActionUpdate,
				protectionChange,
				needsUpdate,
			)
			protectionErrors.Add(err)
			if err == nil {
				p.planIdentityDirectoryProtectionChangeWithFields(
					current,
					desiredDirectory,
					isProtected,
					shouldProtect,
					updateFields,
					changedFields,
					plan,
				)
			}
			continue
		}

		if needsUpdate {
			err := p.ValidateProtection(
				ResourceTypeIdentityDirectory,
				desiredDirectory.Name,
				isProtected,
				ActionUpdate,
			)
			protectionErrors.Add(err)
			if err == nil {
				p.planIdentityDirectoryUpdateWithFields(current, desiredDirectory, updateFields, changedFields, plan)
			}
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		desiredNames := make(map[string]bool)
		for _, directory := range desired {
			desiredNames[directory.Name] = true
		}

		for name, current := range currentByName {
			if desiredNames[name] {
				continue
			}

			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			err := p.ValidateProtection(ResourceTypeIdentityDirectory, name, isProtected, ActionDelete)
			protectionErrors.Add(err)
			if err == nil {
				p.planIdentityDirectoryDelete(current, plan)
			}
		}
	}

	if protectionErrors.HasErrors() {
		return protectionErrors.Error()
	}

	return nil
}

func (p *identityDirectoryPlannerImpl) planIdentityDirectoryCreate(
	directory resources.IdentityDirectoryResource,
	plan *Plan,
) {
	fields := identityDirectoryCreateFields(directory)
	fields[FieldName] = directory.Name

	change := PlannedChange{
		ID:           p.NextChangeID(ActionCreate, ResourceTypeIdentityDirectory, directory.GetRef()),
		ResourceType: ResourceTypeIdentityDirectory,
		ResourceRef:  directory.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    []string{},
		Namespace:    resources.GetNamespace(directory.Kongctl),
	}

	if directory.Kongctl != nil && directory.Kongctl.Protected != nil {
		change.Protection = *directory.Kongctl.Protected
	}

	plan.AddChange(change)
}

func (p *identityDirectoryPlannerImpl) shouldUpdateIdentityDirectory(
	current state.IdentityDirectory,
	desired resources.IdentityDirectoryResource,
) (bool, map[string]any, map[string]FieldChange) {
	fields := identityDirectoryReplaceFields(current, desired)
	changedFields := make(map[string]FieldChange)

	if desired.Description != nil && current.Description != *desired.Description {
		changedFields[FieldDescription] = FieldChange{Old: current.Description, New: *desired.Description}
	}

	if desired.AllowedControlPlanes != nil &&
		!slices.Equal(current.AllowedControlPlanes, desired.AllowedControlPlanes) {
		changedFields[FieldAllowedControlPlanes] = FieldChange{
			Old: slices.Clone(current.AllowedControlPlanes),
			New: slices.Clone(desired.AllowedControlPlanes),
		}
	}

	if desired.AllowAllControlPlanes != nil &&
		(current.AllowAllControlPlanes == nil || *current.AllowAllControlPlanes != *desired.AllowAllControlPlanes) {
		changedFields[FieldAllowAllControlPlanes] = FieldChange{
			Old: pointerValue(current.AllowAllControlPlanes),
			New: *desired.AllowAllControlPlanes,
		}
	}

	if desired.TTLSecs != nil && (current.TTLSecs == nil || *current.TTLSecs != *desired.TTLSecs) {
		changedFields[FieldTTLSecs] = FieldChange{
			Old: pointerValue(current.TTLSecs),
			New: *desired.TTLSecs,
		}
	}

	if desired.NegativeTTLSecs != nil &&
		(current.NegativeTTLSecs == nil || *current.NegativeTTLSecs != *desired.NegativeTTLSecs) {
		changedFields[FieldNegativeTTLSecs] = FieldChange{
			Old: pointerValue(current.NegativeTTLSecs),
			New: *desired.NegativeTTLSecs,
		}
	}

	if desired.Labels != nil && labels.CompareUserLabels(current.NormalizedLabels, desired.Labels) {
		changedFields[FieldLabels] = FieldChange{
			Old: labels.GetUserLabels(current.NormalizedLabels),
			New: labels.GetUserLabels(desired.Labels),
		}
	}

	return len(changedFields) > 0, fields, changedFields
}

func identityDirectoryCreateFields(directory resources.IdentityDirectoryResource) map[string]any {
	fields := make(map[string]any)
	if directory.Description != nil {
		fields[FieldDescription] = *directory.Description
	}
	if directory.AllowedControlPlanes != nil {
		fields[FieldAllowedControlPlanes] = stringSliceAsAny(directory.AllowedControlPlanes)
	}
	if directory.AllowAllControlPlanes != nil {
		fields[FieldAllowAllControlPlanes] = *directory.AllowAllControlPlanes
	}
	if directory.TTLSecs != nil {
		fields[FieldTTLSecs] = *directory.TTLSecs
	}
	if directory.NegativeTTLSecs != nil {
		fields[FieldNegativeTTLSecs] = *directory.NegativeTTLSecs
	}
	if directory.Labels != nil {
		fields[FieldLabels] = directory.Labels
	}
	return fields
}

func identityDirectoryReplaceFields(
	current state.IdentityDirectory,
	desired resources.IdentityDirectoryResource,
) map[string]any {
	fields := make(map[string]any)
	fields[FieldName] = current.Name

	description := current.Description
	if desired.Description != nil {
		description = *desired.Description
	}
	fields[FieldDescription] = description

	allowedControlPlanes := current.AllowedControlPlanes
	if desired.AllowedControlPlanes != nil {
		allowedControlPlanes = desired.AllowedControlPlanes
	}
	fields[FieldAllowedControlPlanes] = stringSliceAsAny(allowedControlPlanes)

	if desired.AllowAllControlPlanes != nil {
		fields[FieldAllowAllControlPlanes] = *desired.AllowAllControlPlanes
	} else if current.AllowAllControlPlanes != nil {
		fields[FieldAllowAllControlPlanes] = *current.AllowAllControlPlanes
	}

	if desired.TTLSecs != nil {
		fields[FieldTTLSecs] = *desired.TTLSecs
	} else if current.TTLSecs != nil {
		fields[FieldTTLSecs] = *current.TTLSecs
	}

	if desired.NegativeTTLSecs != nil {
		fields[FieldNegativeTTLSecs] = *desired.NegativeTTLSecs
	} else if current.NegativeTTLSecs != nil {
		fields[FieldNegativeTTLSecs] = *current.NegativeTTLSecs
	}

	if desired.Labels != nil {
		fields[FieldLabels] = desired.Labels
	} else {
		fields[FieldLabels] = labels.GetUserLabels(current.NormalizedLabels)
	}
	fields[FieldCurrentLabels] = current.NormalizedLabels

	return fields
}

func (p *identityDirectoryPlannerImpl) planIdentityDirectoryUpdateWithFields(
	current state.IdentityDirectory,
	desired resources.IdentityDirectoryResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	change := PlannedChange{
		ID:            p.NextChangeID(ActionUpdate, ResourceTypeIdentityDirectory, desired.GetRef()),
		ResourceType:  ResourceTypeIdentityDirectory,
		ResourceRef:   desired.GetRef(),
		ResourceID:    current.ID,
		Action:        ActionUpdate,
		Fields:        updateFields,
		ChangedFields: changedFields,
		DependsOn:     []string{},
		Namespace:     resources.GetNamespace(desired.Kongctl),
		Protection:    labels.IsProtectedResource(current.NormalizedLabels),
	}

	plan.AddChange(change)
}

func (p *identityDirectoryPlannerImpl) planIdentityDirectoryProtectionChangeWithFields(
	current state.IdentityDirectory,
	desired resources.IdentityDirectoryResource,
	wasProtected, shouldProtect bool,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.NextChangeID(ActionUpdate, ResourceTypeIdentityDirectory, desired.GetRef()),
		ResourceType: ResourceTypeIdentityDirectory,
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       updateFields,
		Protection: ProtectionChange{
			Old: wasProtected,
			New: shouldProtect,
		},
		DependsOn: []string{},
		Namespace: resources.GetNamespace(desired.Kongctl),
	}
	if len(changedFields) > 0 {
		change.ChangedFields = changedFields
	}

	plan.AddChange(change)
}

func (p *identityDirectoryPlannerImpl) planIdentityDirectoryDelete(
	directory state.IdentityDirectory,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.NextChangeID(ActionDelete, ResourceTypeIdentityDirectory, directory.Name),
		ResourceType: ResourceTypeIdentityDirectory,
		ResourceRef:  directory.Name,
		ResourceID:   directory.ID,
		Action:       ActionDelete,
		Fields:       map[string]any{FieldName: directory.Name},
		DependsOn:    []string{},
	}
	if ns, ok := directory.NormalizedLabels[labels.NamespaceKey]; ok {
		change.Namespace = ns
	} else {
		change.Namespace = DefaultNamespace
	}

	plan.AddChange(change)
}

func stringSliceAsAny(values []string) []any {
	if values == nil {
		return nil
	}

	result := make([]any, len(values))
	for i, value := range values {
		result[i] = value
	}
	return result
}

func pointerValue[T any](value *T) any {
	if value == nil {
		return nil
	}
	return *value
}
