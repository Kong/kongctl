package planner

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
)

type identityDirectoryPlanner struct{ *BasePlanner }

func (p *identityDirectoryPlanner) PlannerComponent() string { return ResourceTypeIdentityDirectory }

func (p *identityDirectoryPlanner) PlanChanges(ctx context.Context, config *Config, plan *Plan) error {
	desired := resources.GetResourcesByNamespace[resources.IdentityDirectoryResource](
		p.planner.resources, config.Namespace,
	)
	all, err := p.planner.listIdentityDirectories(ctx)
	if err != nil {
		if state.IsAPIClientError(err) && len(desired) == 0 {
			return nil
		}
		return err
	}
	byName := make(map[string]state.IdentityDirectory, len(all))
	for _, current := range all {
		if _, exists := byName[current.Name]; exists {
			return fmt.Errorf("multiple directories have name %q", current.Name)
		}
		byName[current.Name] = current
	}

	externalIDs := make(map[string]bool)
	for _, item := range p.planner.resources.AllResourcesByType(resources.ResourceTypeIdentityDirectory) {
		if item.(*resources.IdentityDirectoryResource).IsExternal() {
			externalIDs[item.GetKonnectID()] = true
		}
	}
	externalOnly := p.planner.resources.Identity != nil && len(p.planner.resources.Identity.Directories) > 0
	if externalOnly {
		for _, d := range p.planner.resources.Identity.Directories {
			if !d.IsExternal() {
				externalOnly = false
				break
			}
		}
	}
	var currentRoots []managedRoot[state.IdentityDirectory]
	for _, current := range all {
		if externalOnly || !labels.IsManagedResource(current.NormalizedLabels) ||
			current.NormalizedLabels[labels.NamespaceKey] != config.Namespace || externalIDs[current.ID] {
			continue
		}
		currentRoots = append(currentRoots, managedRoot[state.IdentityDirectory]{
			resource: current, name: current.Name, protected: resources.IsChildProtected(current.ManagedBy),
		})
	}
	var desiredRoots []managedRoot[resources.IdentityDirectoryResource]
	for _, directory := range desired {
		if directory.IsExternal() {
			continue
		}
		if current, exists := byName[directory.Name]; exists {
			if current.ManagedBy[resources.ManagedByToolKey] != resources.ManagedByTool {
				return fmt.Errorf(
					"directory %q is not owned by kongctl; explicitly adopt it before managing it",
					directory.Name,
				)
			}
			if err := resources.ValidateChildOwnership(
				"directory "+directory.Name, current.ManagedBy, config.Namespace,
			); err != nil {
				return err
			}
		}
		directory.AllowedControlPlanes = slices.Clone(directory.AllowedControlPlanes)
		for i, value := range directory.AllowedControlPlanes {
			if tags.IsRefPlaceholder(value) {
				resolved, err := p.planner.resources.ResolvePayloadReference(value)
				if err != nil {
					return err
				}
				value, ok := resolved.(string)
				if !ok {
					return fmt.Errorf("control plane reference must resolve to an ID string")
				}
				directory.AllowedControlPlanes[i] = value
			}
		}
		desiredRoots = append(desiredRoots, managedRoot[resources.IdentityDirectoryResource]{
			resource: directory, name: directory.Name, protected: directory.Protected(),
		})
	}
	if err := reconcileManagedRoots(p.BasePlanner, ResourceTypeIdentityDirectory, desiredRoots, currentRoots,
		managedRootOperations[resources.IdentityDirectoryResource, state.IdentityDirectory]{
			diff: p.diff,
			create: func(d resources.IdentityDirectoryResource, plan *Plan) string {
				fields := identityDirectoryFields(d)
				fields[FieldManagedBy] = resources.BuildChildManagedBy(resources.GetNamespace(d.Kongctl), d.Protected(), "")
				change := PlannedChange{
					ID:           p.NextChangeID(ActionCreate, ResourceTypeIdentityDirectory, d.Ref),
					ResourceType: ResourceTypeIdentityDirectory, ResourceRef: d.Ref, Action: ActionCreate,
					Fields: fields, Namespace: resources.GetNamespace(d.Kongctl), Protection: d.Protected(),
				}
				plan.AddChange(change)
				return change.ID
			},
			update: func(current state.IdentityDirectory, desired resources.IdentityDirectoryResource,
				fields map[string]any, changed map[string]FieldChange, plan *Plan,
			) {
				p.update(current, desired, fields, changed, nil, plan)
			},
			changeProtection: func(current state.IdentityDirectory, desired resources.IdentityDirectoryResource,
				old, next bool, fields map[string]any, changed map[string]FieldChange, plan *Plan,
			) {
				p.update(current, desired, fields, changed, ProtectionChange{Old: old, New: next}, plan)
			},
			remove: func(current state.IdentityDirectory, plan *Plan) {
				plan.AddChange(PlannedChange{
					ID:           p.NextChangeID(ActionDelete, ResourceTypeIdentityDirectory, current.Name),
					ResourceType: ResourceTypeIdentityDirectory, ResourceRef: current.Name, ResourceID: current.ID,
					Action: ActionDelete, Fields: map[string]any{FieldName: current.Name},
					Namespace: current.ManagedBy[resources.ChildNamespaceKey], Protection: false,
				})
			},
		}, plan); err != nil {
		return err
	}
	return p.planPrincipals(ctx, config, plan, byName)
}

func identityDirectoryFields(d resources.IdentityDirectoryResource) map[string]any {
	fields := map[string]any{FieldName: d.Name}
	if d.Description != nil {
		fields[FieldDescription] = *d.Description
	}
	if d.AllowAllControlPlanes != nil {
		fields[FieldAllowAllControlPlanes] = *d.AllowAllControlPlanes
	}
	if d.TTLSecs != nil {
		fields[FieldTTLSecs] = *d.TTLSecs
	}
	if d.NegativeTTLSecs != nil {
		fields[FieldNegativeTTLSecs] = *d.NegativeTTLSecs
	}
	if d.VaultEnabled != nil {
		fields[FieldVaultEnabled] = *d.VaultEnabled
	}
	if d.Labels != nil {
		fields[FieldLabels] = d.Labels
	}
	if d.AllowedControlPlanes != nil {
		values := make([]any, len(d.AllowedControlPlanes))
		for i, value := range d.AllowedControlPlanes {
			values[i] = value
		}
		fields[FieldAllowedControlPlanes] = values
	}
	return fields
}

func (p *identityDirectoryPlanner) diff(current state.IdentityDirectory, desired resources.IdentityDirectoryResource,
) (bool, map[string]any, map[string]FieldChange) {
	fields := make(map[string]any)
	changed := make(map[string]FieldChange)
	compare := func(name string, old, next any) {
		if !reflect.DeepEqual(old, next) {
			fields[name] = next
			changed[name] = FieldChange{Old: old, New: next}
		}
	}
	if desired.Description != nil {
		compare(FieldDescription, current.Description, *desired.Description)
	}
	if desired.AllowAllControlPlanes != nil {
		compare(FieldAllowAllControlPlanes, current.AllowAllControlPlanes, desired.AllowAllControlPlanes)
	}
	if desired.TTLSecs != nil {
		compare(FieldTTLSecs, current.TTLSecs, desired.TTLSecs)
	}
	if desired.NegativeTTLSecs != nil {
		compare(FieldNegativeTTLSecs, current.NegativeTTLSecs, desired.NegativeTTLSecs)
	}
	if desired.VaultEnabled != nil {
		compare(FieldVaultEnabled, current.VaultEnabled, desired.VaultEnabled)
	}
	if desired.Labels != nil && !maps.Equal(current.Labels, desired.Labels) {
		fields[FieldLabels] = desired.Labels
		changed[FieldLabels] = FieldChange{Old: current.Labels, New: desired.Labels}
	}
	if desired.AllowedControlPlanes != nil {
		old, next := slices.Clone(current.AllowedControlPlanes), slices.Clone(desired.AllowedControlPlanes)
		slices.Sort(old)
		slices.Sort(next)
		if !slices.Equal(old, next) {
			fields[FieldAllowedControlPlanes] = identityDirectoryFields(desired)[FieldAllowedControlPlanes]
			changed[FieldAllowedControlPlanes] = FieldChange{
				Old: current.AllowedControlPlanes, New: desired.AllowedControlPlanes,
			}
		}
	}
	metadata := resources.BuildChildManagedBy(resources.GetNamespace(desired.Kongctl), desired.Protected(), "")
	comparison := maps.Clone(metadata)
	comparison[resources.ChildProtectedKey] = current.ManagedBy[resources.ChildProtectedKey]
	// Protection is handled by the shared root reconciler. Canonicalizing outside
	// metadata during a protection-only unprotect follows the owned-child policy.
	unprotect := resources.IsChildProtected(current.ManagedBy) && !desired.Protected()
	if !unprotect && !managedByValuesEqual(current.ManagedBy, comparison) {
		fields[FieldManagedBy] = metadata
		changed[FieldManagedBy] = FieldChange{Old: current.ManagedBy, New: metadata}
	}
	return len(fields) > 0, fields, changed
}

func (p *identityDirectoryPlanner) update(current state.IdentityDirectory, desired resources.IdentityDirectoryResource,
	fields map[string]any, changed map[string]FieldChange, protection any, plan *Plan,
) {
	fields = maps.Clone(fields)
	changed = maps.Clone(changed)
	fields[FieldName] = current.Name
	metadata := resources.BuildChildManagedBy(resources.GetNamespace(desired.Kongctl), desired.Protected(), "")
	fields[FieldManagedBy] = metadata
	if protection != nil {
		changed[FieldManagedBy] = FieldChange{Old: current.ManagedBy, New: metadata}
	} else {
		protection = desired.Protected()
	}
	plan.AddChange(PlannedChange{
		ID:           p.NextChangeID(ActionUpdate, ResourceTypeIdentityDirectory, desired.Ref),
		ResourceType: ResourceTypeIdentityDirectory, ResourceRef: desired.Ref, ResourceID: current.ID,
		Action: ActionUpdate, Fields: fields, ChangedFields: changed,
		Namespace: resources.GetNamespace(desired.Kongctl), Protection: protection,
	})
}
