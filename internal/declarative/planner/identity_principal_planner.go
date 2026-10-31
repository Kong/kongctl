package planner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) listIdentityPrincipals(ctx context.Context, directoryID string) ([]state.IdentityPrincipal, error) {
	if p.resourceCache != nil {
		if values, ok := p.resourceCache.identityPrincipals[directoryID]; ok {
			return values, nil
		}
	}
	values, err := p.client.ListIdentityPrincipals(ctx, directoryID)
	if err == nil && p.resourceCache != nil {
		p.resourceCache.identityPrincipals[directoryID] = values
	}
	return values, err
}

func (p *identityDirectoryPlanner) planPrincipals(ctx context.Context, config *Config, plan *Plan,
	byName map[string]state.IdentityDirectory,
) error {
	rs := p.planner.resources
	if rs.Identity == nil {
		return nil
	}
	type parentScope struct {
		ref, id string
		refs    map[string]bool
		desired []resources.IdentityPrincipalResource
		scoped  bool
	}
	var parents []*parentScope
	byID := make(map[string]*parentScope)
	for _, directory := range rs.Identity.Directories {
		id := directory.GetKonnectID()
		if current, exists := byName[directory.Name]; exists && !directory.IsExternal() {
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
		if rs.SyncScope != nil &&
			rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityPrincipal][directory.Ref] != nil {
			_, inScope := rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityPrincipal][directory.Ref][config.Namespace]
			parent.scoped = parent.scoped || inScope
		}
	}
	for _, parent := range parents {
		for _, principal := range rs.Identity.Principals {
			if parent.refs[resources.NormalizeResourceRef(principal.Directory)] &&
				resources.GetNamespace(principal.Kongctl) == config.Namespace {
				parent.desired = append(parent.desired, principal)
			}
		}
		if len(parent.desired) == 0 && (plan.Metadata.Mode != PlanModeSync || !parent.scoped) {
			continue
		}
		var all []state.IdentityPrincipal
		if parent.id != "" {
			var err error
			all, err = p.planner.listIdentityPrincipals(ctx, parent.id)
			if err != nil {
				return err
			}
		}
		currentByRef := make(map[string]state.IdentityPrincipal)
		for _, value := range all {
			if value.ManagedBy[resources.ManagedByToolKey] != resources.ManagedByTool ||
				value.ManagedBy[resources.ChildNamespaceKey] != config.Namespace {
				continue
			}
			ref := value.ManagedBy[resources.ManagedByRefKey]
			// Visibility alone never enrolls preexisting, imported or JIT resources.
			if err := resources.ValidateRef(ref); err != nil {
				continue
			}
			if err := resources.ValidateChildOwnership("principal "+ref, value.ManagedBy, config.Namespace); err != nil {
				return err
			}
			if _, duplicate := currentByRef[ref]; duplicate {
				return fmt.Errorf(
					"multiple principals match directory %q, namespace %q, ref %q",
					parent.ref,
					config.Namespace,
					ref,
				)
			}
			currentByRef[ref] = value
		}
		var desired []managedRoot[resources.IdentityPrincipalResource]
		for _, value := range parent.desired {
			if current, found := currentByRef[value.Ref]; found {
				// Q9 compatibility restriction; see contributor/kong-identity-compatibility.md.
				if plan.Metadata.Mode != PlanModeDelete && value.TypeDeclared() &&
					!principalTypeEqual(current.Type, value.Type) {
					return fmt.Errorf("principal %q type is create-only pending API question Q9", value.Ref)
				}
			}
			desired = append(
				desired,
				managedRoot[resources.IdentityPrincipalResource]{
					resource:  value,
					name:      value.Ref,
					protected: value.Protected(),
				},
			)
		}
		var current []managedRoot[state.IdentityPrincipal]
		for _, value := range all {
			if matched, ok := currentByRef[value.GetName()]; ok && matched.ID == value.ID {
				current = append(
					current,
					managedRoot[state.IdentityPrincipal]{
						resource:  value,
						name:      value.GetName(),
						protected: resources.IsChildProtected(value.ManagedBy),
					},
				)
			}
		}
		start := len(plan.Changes)
		childPlan := plan
		if plan.Metadata.Mode == PlanModeSync && !parent.scoped {
			// Programmatic callers without captured scope may update declared children,
			// but omission never authorizes pruning.
			copyPlan := *plan
			copyPlan.Metadata.Mode = PlanModeApply
			childPlan = &copyPlan
		}
		err := reconcileManagedRoots(p.BasePlanner, ResourceTypeIdentityPrincipal, desired, current,
			managedRootOperations[resources.IdentityPrincipalResource, state.IdentityPrincipal]{
				diff: diffIdentityPrincipal,
				create: func(d resources.IdentityPrincipalResource, plan *Plan) string {
					change := p.principalChange(
						ActionCreate,
						d.Ref,
						"",
						identityPrincipalFields(d),
						nil,
						config.Namespace,
					)
					change.Protection = d.Protected()
					plan.AddChange(change)
					return change.ID
				},
				update: func(c state.IdentityPrincipal, d resources.IdentityPrincipalResource,
					fields map[string]any, changed map[string]FieldChange, plan *Plan,
				) {
					change := p.principalChange(ActionUpdate, d.Ref, c.ID, fields, changed, config.Namespace)
					change.Protection = d.Protected()
					plan.AddChange(change)
				},
				changeProtection: func(c state.IdentityPrincipal, d resources.IdentityPrincipalResource, old, next bool,
					fields map[string]any, changed map[string]FieldChange, plan *Plan,
				) {
					fields = maps.Clone(fields)
					changed = maps.Clone(changed)
					fields[FieldManagedBy] = resources.BuildPrincipalManagedBy(config.Namespace, d.Ref, next, "")
					changed[FieldManagedBy] = FieldChange{Old: c.ManagedBy, New: fields[FieldManagedBy]}
					change := p.principalChange(ActionUpdate, d.Ref, c.ID, fields, changed, config.Namespace)
					change.Protection = ProtectionChange{Old: old, New: next}
					plan.AddChange(change)
				},
				remove: func(c state.IdentityPrincipal, plan *Plan) {
					change := p.principalChange(ActionDelete, c.GetName(), c.ID, nil, nil, config.Namespace)
					change.Protection = false
					plan.AddChange(change)
				},
			}, childPlan)
		if err != nil {
			return err
		}
		if childPlan != plan {
			plan.Changes = childPlan.Changes
			plan.Warnings = childPlan.Warnings
		}
		for i := start; i < len(plan.Changes); i++ {
			change := &plan.Changes[i]
			change.Parent = &ParentInfo{Ref: parent.ref, ID: parent.id}
			for j := range plan.Changes[:start] {
				directoryChange := &plan.Changes[j]
				if directoryChange.ResourceType != ResourceTypeIdentityDirectory {
					continue
				}
				if directoryChange.Action == ActionCreate && parent.refs[directoryChange.ResourceRef] {
					change.DependsOn = append(change.DependsOn, directoryChange.ID)
				}
				if directoryChange.Action == ActionDelete && directoryChange.ResourceID == parent.id &&
					change.Action == ActionDelete {
					directoryChange.DependsOn = append(directoryChange.DependsOn, change.ID)
				}
			}
		}
	}
	return nil
}

func (p *identityDirectoryPlanner) principalChange(action ActionType, ref, id string, fields map[string]any,
	changed map[string]FieldChange, namespace string,
) PlannedChange {
	return PlannedChange{
		ID: p.NextChangeID(action, ResourceTypeIdentityPrincipal, ref), ResourceType: ResourceTypeIdentityPrincipal,
		ResourceRef: ref, ResourceID: id, Action: action, Fields: fields, ChangedFields: changed, Namespace: namespace,
	}
}

func principalTypeEqual(current *kkComps.KongPrincipalType, desired *string) bool {
	if current == nil {
		return desired == nil
	}
	return desired != nil && string(*current) == *desired
}

func identityPrincipalFields(d resources.IdentityPrincipalResource) map[string]any {
	fields := map[string]any{
		FieldManagedBy: resources.BuildPrincipalManagedBy(resources.GetNamespace(d.Kongctl), d.Ref, d.Protected(), ""),
	}
	if d.DisplayName != nil {
		fields[FieldDisplayName] = *d.DisplayName
	}
	if d.Description != nil {
		fields[FieldDescription] = *d.Description
	}
	if d.Type != nil {
		fields[FieldType] = *d.Type
	}
	if d.Metadata != nil {
		fields[FieldMetadata] = d.Metadata
	}
	if d.Labels != nil {
		fields[FieldLabels] = d.Labels
	}
	return fields
}

func diffIdentityPrincipal(
	c state.IdentityPrincipal,
	d resources.IdentityPrincipalResource,
) (bool, map[string]any, map[string]FieldChange) {
	fields := make(map[string]any)
	changed := make(map[string]FieldChange)
	compare := func(field string, old, next any) {
		before, _ := json.Marshal(old)
		after, _ := json.Marshal(next)
		if string(before) != string(after) {
			fields[field] = next
			changed[field] = FieldChange{Old: old, New: next}
		}
	}
	if d.DisplayName != nil {
		var old any
		if c.DisplayName != nil {
			old = *c.DisplayName
		}
		compare(FieldDisplayName, old, *d.DisplayName)
	}
	if d.Description != nil {
		compare(FieldDescription, c.Description, *d.Description)
	}
	if d.Metadata != nil {
		var old any = c.Metadata
		if c.Metadata == nil {
			old = map[string]any{}
		}
		compare(FieldMetadata, old, d.Metadata)
	}
	if d.Labels != nil {
		old := c.Labels
		if old == nil {
			old = make(map[string]string)
		}
		compare(FieldLabels, old, d.Labels)
	}
	metadata := resources.BuildPrincipalManagedBy(resources.GetNamespace(d.Kongctl), d.Ref, d.Protected(), "")
	comparison := maps.Clone(metadata)
	comparison[resources.ChildProtectedKey] = c.ManagedBy[resources.ChildProtectedKey]
	if (!resources.IsChildProtected(c.ManagedBy) || d.Protected()) && !managedByValuesEqual(c.ManagedBy, comparison) {
		fields[FieldManagedBy] = metadata
		changed[FieldManagedBy] = FieldChange{Old: c.ManagedBy, New: metadata}
	}
	if len(fields) != 0 {
		fields[FieldManagedBy] = metadata
	}
	return len(fields) > 0, fields, changed
}

// Namespace plans are merged before computing parent deletion dependencies.
func adjustIdentityDirectoryDeleteDependencies(changes []PlannedChange) {
	for i := range changes {
		parent := &changes[i]
		if parent.ResourceType != ResourceTypeIdentityDirectory || parent.Action != ActionDelete {
			continue
		}
		for _, child := range changes {
			if child.ResourceType == ResourceTypeIdentityPrincipal && child.Action == ActionDelete && child.Parent != nil &&
				(child.Parent.ID == parent.ResourceID || child.Parent.Ref == parent.ResourceRef) &&
				!contains(parent.DependsOn, child.ID) {
				parent.DependsOn = append(parent.DependsOn, child.ID)
			}
		}
	}
}

// Preserve integer metadata when reloading a principal's saved plan. Other
// resource payloads retain their existing JSON number representation.
func (change *PlannedChange) UnmarshalJSON(data []byte) error {
	type plain PlannedChange
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.ResourceType == ResourceTypeIdentityPrincipal {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		value = plain{}
		if err := decoder.Decode(&value); err != nil {
			return err
		}
	}
	*change = PlannedChange(value)
	return nil
}
