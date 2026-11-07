package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

func (p *Planner) listIdentityGroups(ctx context.Context, directoryID string) ([]state.IdentityGroup, error) {
	if p.resourceCache != nil {
		if values, ok := p.resourceCache.identityGroups[directoryID]; ok {
			return values, nil
		}
	}
	values, err := p.client.ListIdentityGroups(ctx, directoryID)
	if err == nil && p.resourceCache != nil {
		p.resourceCache.identityGroups[directoryID] = values
	}
	return values, err
}

func (p *identityDirectoryPlanner) planGroups(ctx context.Context, config *Config, plan *Plan,
	byName map[string]state.IdentityDirectory,
) error {
	rs := p.planner.resources
	if rs.Identity == nil {
		return nil
	}
	type parentScope struct {
		ref, id string
		refs    map[string]bool
		scoped  bool
	}
	var parents []*parentScope
	byID := make(map[string]*parentScope)
	for _, directory := range rs.Identity.Directories {
		id := directory.GetKonnectID()
		if current, ok := byName[directory.Name]; ok && !directory.IsExternal() {
			id = current.ID
		}
		key := id
		if key == "" {
			key = directory.Ref
		}
		parent := byID[key]
		if parent == nil {
			parent = &parentScope{ref: directory.Ref, id: id, refs: make(map[string]bool)}
			parents = append(parents, parent)
			byID[key] = parent
		}
		parent.refs[directory.Ref] = true
		if rs.SyncScope != nil {
			_, scoped := rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityGroup][directory.Ref][config.Namespace]
			parent.scoped = parent.scoped || scoped
		}
	}
	for _, parent := range parents {
		var desired []managedRoot[resources.IdentityGroupResource]
		externalIDs, names := make(map[string]bool), make(map[string]bool)
		declaredNames := make(map[string]bool)
		for _, group := range rs.Identity.Groups {
			if !parent.refs[resources.NormalizeResourceRef(group.Directory)] {
				continue
			}
			if err := group.Validate(); err != nil {
				return err
			}
			if group.IsExternal() {
				externalIDs[group.GetKonnectID()] = true
				continue
			}
			if declaredNames[group.Name] {
				return fmt.Errorf("multiple declarations manage group %q in the same directory", group.Name)
			}
			declaredNames[group.Name] = true
			if resources.GetNamespace(group.Kongctl) != config.Namespace {
				continue
			}
			if names[group.Name] {
				return fmt.Errorf("multiple declarations manage group %q in the same directory", group.Name)
			}
			names[group.Name] = true
			desired = append(
				desired,
				managedRoot[resources.IdentityGroupResource]{
					resource:  group,
					name:      group.Name,
					protected: group.Protected(),
				},
			)
		}
		if len(desired) == 0 && (plan.Metadata.Mode != PlanModeSync || !parent.scoped) {
			continue
		}
		var all []state.IdentityGroup
		if parent.id != "" {
			var err error
			all, err = p.planner.listIdentityGroups(ctx, parent.id)
			if err != nil {
				return err
			}
		}
		seen := make(map[string]bool)
		var current []managedRoot[state.IdentityGroup]
		for _, group := range all {
			if seen[group.Name] {
				return fmt.Errorf("multiple groups have name %q in directory %q", group.Name, parent.ref)
			}
			seen[group.Name] = true
			if names[group.Name] {
				if externalIDs[group.ID] {
					return fmt.Errorf("group %q is declared as both managed and external", group.Name)
				}
				if group.ManagedBy[resources.ManagedByToolKey] != resources.ManagedByTool {
					return fmt.Errorf(
						"group %q is not owned by kongctl; explicitly adopt it before managing it",
						group.Name,
					)
				}
				if err := resources.ValidateChildOwnership("group "+group.Name, group.ManagedBy, config.Namespace); err != nil {
					return err
				}
				if err := helpers.ValidateIdentityGroupManagement(group.IdentityGroup); err != nil {
					return err
				}
			}
			if externalIDs[group.ID] || group.ManagedBy[resources.ManagedByToolKey] != resources.ManagedByTool ||
				group.ManagedBy[resources.ChildNamespaceKey] != config.Namespace {
				continue
			}
			// Unsupported/importer-owned kinds never enter ordinary-group pruning.
			if err := helpers.ValidateIdentityGroupManagement(group.IdentityGroup); err != nil {
				continue
			}
			if err := resources.ValidateChildOwnership("group "+group.Name, group.ManagedBy, config.Namespace); err != nil {
				return err
			}
			current = append(current, managedRoot[state.IdentityGroup]{
				resource: group, name: group.Name,
				protected: resources.IsChildProtected(group.ManagedBy),
			})
		}
		start := len(plan.Changes)
		childPlan := plan
		if plan.Metadata.Mode == PlanModeSync && !parent.scoped {
			clone := *plan
			clone.Metadata.Mode = PlanModeApply
			childPlan = &clone
		}
		change := func(action ActionType, ref, id string, fields map[string]any,
			changed map[string]FieldChange,
		) PlannedChange {
			return PlannedChange{
				ID: p.NextChangeID(action, ResourceTypeIdentityGroup, ref), ResourceType: ResourceTypeIdentityGroup,
				ResourceRef: ref, ResourceID: id, Action: action, Fields: fields, ChangedFields: changed,
				Namespace: config.Namespace,
			}
		}
		update := func(c state.IdentityGroup, d resources.IdentityGroupResource, fields map[string]any,
			changed map[string]FieldChange, protection any, plan *Plan,
		) {
			fields, changed = maps.Clone(fields), maps.Clone(changed)
			fields[FieldName] = c.Name
			fields[FieldManagedBy] = resources.BuildChildManagedBy(config.Namespace, d.Protected(), "")
			if _, ok := protection.(ProtectionChange); ok {
				changed[FieldManagedBy] = FieldChange{Old: c.ManagedBy, New: fields[FieldManagedBy]}
			}
			value := change(ActionUpdate, d.Ref, c.ID, fields, changed)
			value.Protection = protection
			plan.AddChange(value)
		}
		err := reconcileManagedRoots(p.BasePlanner, ResourceTypeIdentityGroup, desired, current,
			managedRootOperations[resources.IdentityGroupResource, state.IdentityGroup]{
				diff: diffIdentityGroup,
				create: func(d resources.IdentityGroupResource, plan *Plan) string {
					value := change(ActionCreate, d.Ref, "", identityGroupFields(d), nil)
					value.Protection = d.Protected()
					plan.AddChange(value)
					return value.ID
				},
				update: func(c state.IdentityGroup, d resources.IdentityGroupResource,
					fields map[string]any, changed map[string]FieldChange, plan *Plan,
				) {
					update(c, d, fields, changed, d.Protected(), plan)
				},
				changeProtection: func(c state.IdentityGroup, d resources.IdentityGroupResource, old, next bool,
					fields map[string]any, changed map[string]FieldChange, plan *Plan,
				) {
					update(c, d, fields, changed, ProtectionChange{Old: old, New: next}, plan)
				},
				remove: func(c state.IdentityGroup, plan *Plan) {
					value := change(ActionDelete, c.Name, c.ID, map[string]any{FieldName: c.Name}, nil)
					value.Protection = false
					plan.AddChange(value)
				},
			}, childPlan)
		if err != nil {
			return err
		}
		if childPlan != plan {
			plan.Changes, plan.Warnings = childPlan.Changes, childPlan.Warnings
		}
		for i := start; i < len(plan.Changes); i++ {
			value := &plan.Changes[i]
			value.Parent = &ParentInfo{Ref: parent.ref, ID: parent.id}
			for _, earlier := range plan.Changes[:start] {
				if earlier.ResourceType == ResourceTypeIdentityDirectory && earlier.Action == ActionCreate &&
					parent.refs[earlier.ResourceRef] {
					value.DependsOn = append(value.DependsOn, earlier.ID)
				}
			}
		}
	}
	return nil
}

func identityGroupFields(g resources.IdentityGroupResource) map[string]any {
	fields := map[string]any{
		FieldName:      g.Name,
		FieldManagedBy: resources.BuildChildManagedBy(resources.GetNamespace(g.Kongctl), g.Protected(), ""),
	}
	if g.Description != nil {
		fields[FieldDescription] = *g.Description
	}
	if g.Type != nil {
		fields[FieldType] = *g.Type
	}
	if g.Metadata != nil {
		fields[FieldMetadata] = g.Metadata
	}
	if g.Labels != nil {
		fields[FieldLabels] = g.Labels
	}
	return fields
}

func diffIdentityGroup(
	c state.IdentityGroup,
	g resources.IdentityGroupResource,
) (bool, map[string]any, map[string]FieldChange) {
	fields, changed := make(map[string]any), make(map[string]FieldChange)
	compare := func(key string, old, next any) {
		before, _ := json.Marshal(old)
		after, _ := json.Marshal(next)
		if string(before) != string(after) {
			fields[key] = next
			changed[key] = FieldChange{Old: old, New: next}
		}
	}
	if g.Description != nil {
		compare(FieldDescription, c.Description, *g.Description)
	}
	if g.Metadata != nil {
		old := c.Metadata
		if old == nil {
			old = make(map[string]kkComps.KongIdentityMetadata)
		}
		compare(FieldMetadata, old, g.Metadata)
	}
	if g.Labels != nil {
		old := c.Labels
		if old == nil {
			old = make(map[string]string)
		}
		compare(FieldLabels, old, g.Labels)
	}
	metadata := resources.BuildChildManagedBy(resources.GetNamespace(g.Kongctl), g.Protected(), "")
	comparison := maps.Clone(metadata)
	comparison[resources.ChildProtectedKey] = c.ManagedBy[resources.ChildProtectedKey]
	if (!resources.IsChildProtected(c.ManagedBy) || g.Protected()) && !managedByValuesEqual(c.ManagedBy, comparison) {
		fields[FieldManagedBy] = metadata
		changed[FieldManagedBy] = FieldChange{Old: c.ManagedBy, New: metadata}
	}
	return len(fields) != 0, fields, changed
}
