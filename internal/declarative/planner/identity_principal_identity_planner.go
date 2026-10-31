package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

func (p *Planner) listPrincipalIdentities(ctx context.Context, directoryID, principalID string,
) ([]helpers.PrincipalIdentity, error) {
	key := directoryID + "/" + principalID
	if p.resourceCache != nil {
		if values, ok := p.resourceCache.identityPrincipalIdentities[key]; ok {
			return values, nil
		}
	}
	values, err := p.client.ListIdentityPrincipalIdentities(ctx, directoryID, principalID)
	if err == nil && p.resourceCache != nil {
		p.resourceCache.identityPrincipalIdentities[key] = values
	}
	return values, err
}

func (p *identityDirectoryPlanner) planIdentities(ctx context.Context, config *Config, plan *Plan,
	directories map[string]state.IdentityDirectory,
) error {
	rs := p.planner.resources
	if rs.Identity == nil {
		return nil
	}
	type parentScope struct {
		ref, id, directoryRef, directoryID string
		refs                               map[string]bool
		scoped                             bool
	}
	var parents []*parentScope
	byID := make(map[string]*parentScope)
	for _, principal := range rs.Identity.Principals {
		d, found := rs.GetResourceByRef(resources.NormalizeResourceRef(principal.Directory))
		if !found {
			return fmt.Errorf("identity principal %q has no declared directory", principal.Ref)
		}
		directory := d.(*resources.IdentityDirectoryResource)
		directoryID := directory.GetKonnectID()
		if current, found := directories[directory.Name]; found && !directory.IsExternal() {
			directoryID = current.ID
		}
		id := principal.GetKonnectID()
		if !principal.IsExternal() {
			id = ""
		}
		if !principal.IsExternal() && directoryID != "" {
			all, err := p.planner.listIdentityPrincipals(ctx, directoryID)
			if err != nil {
				return err
			}
			matches := 0
			for _, current := range all {
				if current.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool &&
					current.ManagedBy[resources.ChildNamespaceKey] == resources.GetNamespace(principal.Kongctl) &&
					current.GetName() == principal.Ref {
					id = current.ID
					matches++
				}
			}
			if matches > 1 {
				return fmt.Errorf("multiple principals match identity parent %q", principal.Ref)
			}
		}
		key := directoryID + "/" + id
		if id == "" {
			key = principal.Ref
		}
		parent := byID[key]
		if parent == nil {
			parent = &parentScope{
				ref: principal.Ref, id: id, directoryRef: directory.Ref, directoryID: directoryID,
				refs: make(map[string]bool),
			}
			parents = append(parents, parent)
			byID[key] = parent
		}
		parent.refs[principal.Ref] = true
		if rs.SyncScope != nil {
			byPrincipal := rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityPrincipalIdentity]
			_, scoped := byPrincipal[principal.Ref][config.Namespace]
			parent.scoped = parent.scoped || scoped
		}
	}
	for _, parent := range parents {
		var desired []managedRoot[resources.IdentityPrincipalIdentityResource]
		desiredRefs := make(map[string]bool)
		externalIDs := make(map[string]bool)
		for _, identity := range rs.Identity.Identities {
			if identity.IsExternal() {
				if parent.refs[resources.NormalizeResourceRef(identity.Principal)] {
					externalIDs[identity.GetKonnectID()] = true
				}
				continue
			}
			if parent.refs[resources.NormalizeResourceRef(identity.Principal)] &&
				resources.GetNamespace(identity.Kongctl) == config.Namespace {
				if desiredRefs[identity.Ref] {
					return fmt.Errorf("duplicate identity ref %q through principal aliases", identity.Ref)
				}
				desiredRefs[identity.Ref] = true
				desired = append(desired, managedRoot[resources.IdentityPrincipalIdentityResource]{
					resource: identity, name: identity.Ref, protected: identity.Protected(),
				})
			}
		}
		if len(desired) == 0 && (plan.Metadata.Mode != PlanModeSync || !parent.scoped) {
			continue
		}
		var all []helpers.PrincipalIdentity
		if parent.id != "" {
			var err error
			all, err = p.planner.listPrincipalIdentities(ctx, parent.directoryID, parent.id)
			if err != nil {
				return err
			}
		}
		var current []managedRoot[state.IdentityPrincipalIdentity]
		seen := make(map[string]bool)
		for _, identity := range all {
			if externalIDs[identity.ID] {
				if identity.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool &&
					identity.ManagedBy[resources.ChildNamespaceKey] == config.Namespace &&
					desiredRefs[identity.ManagedBy[resources.ManagedByRefKey]] {
					return fmt.Errorf("identity %q is declared as both managed and external",
						identity.ManagedBy[resources.ManagedByRefKey])
				}
				continue
			}
			if identity.ManagedBy[resources.ManagedByToolKey] != resources.ManagedByTool ||
				identity.ManagedBy[resources.ChildNamespaceKey] != config.Namespace {
				continue
			}
			value := state.NormalizeIdentityPrincipalIdentity(identity)
			if resources.ValidateRef(value.GetName()) != nil {
				continue
			}
			if err := resources.ValidateChildOwnership("principal identity", identity.ManagedBy, config.Namespace); err != nil {
				return err
			}
			if seen[value.GetName()] {
				return fmt.Errorf(
					"multiple identities match principal %q, namespace %q, ref %q",
					parent.ref,
					config.Namespace,
					value.GetName(),
				)
			}
			seen[value.GetName()] = true
			current = append(current, managedRoot[state.IdentityPrincipalIdentity]{
				resource: value, name: value.GetName(),
				protected: resources.IsChildProtected(identity.ManagedBy),
			})
		}
		for _, d := range desired {
			for _, c := range current {
				if c.name == d.name && c.resource.Type != d.resource.Type && plan.Metadata.Mode != PlanModeDelete {
					return fmt.Errorf("identity %q type cannot be changed; declare a separate identity", d.name)
				}
			}
		}
		childPlan := plan
		if plan.Metadata.Mode == PlanModeSync && !parent.scoped {
			applyPlan := *plan
			applyPlan.Metadata.Mode = PlanModeApply
			childPlan = &applyPlan
		}
		change := func(action ActionType, ref, id string, fields map[string]any,
			changed map[string]FieldChange,
		) PlannedChange {
			return PlannedChange{
				ID:           p.NextChangeID(action, ResourceTypeIdentityPrincipalIdentity, ref),
				ResourceType: ResourceTypeIdentityPrincipalIdentity, ResourceRef: ref, ResourceID: id, Action: action,
				Namespace: config.Namespace, Fields: fields, ChangedFields: changed,
				Parent: &ParentInfo{Ref: parent.ref, ID: parent.id},
				References: map[string]ReferenceInfo{
					FieldDirectoryID: {Ref: parent.directoryRef, ID: parent.directoryID},
				},
			}
		}
		err := reconcileManagedRoots(p.BasePlanner, ResourceTypeIdentityPrincipalIdentity, desired, current,
			managedRootOperations[resources.IdentityPrincipalIdentityResource, state.IdentityPrincipalIdentity]{
				diff: diffPrincipalIdentity,
				create: func(d resources.IdentityPrincipalIdentityResource, plan *Plan) string {
					c := change(ActionCreate, d.Ref, "", principalIdentityFields(d), nil)
					c.Protection = d.Protected()
					plan.AddChange(c)
					return c.ID
				},
				update: func(c state.IdentityPrincipalIdentity, d resources.IdentityPrincipalIdentityResource,
					fields map[string]any,
					changed map[string]FieldChange, plan *Plan,
				) {
					v := change(ActionUpdate, d.Ref, c.ID, fields, changed)
					v.Protection = d.Protected()
					plan.AddChange(v)
				},
				changeProtection: func(c state.IdentityPrincipalIdentity, d resources.IdentityPrincipalIdentityResource,
					old, next bool,
					_ map[string]any, changed map[string]FieldChange, plan *Plan,
				) {
					fields := principalIdentityFields(d)
					changed = maps.Clone(changed)
					changed[FieldManagedBy] = FieldChange{Old: c.ManagedBy, New: fields[FieldManagedBy]}
					v := change(ActionUpdate, d.Ref, c.ID, fields, changed)
					v.Protection = ProtectionChange{Old: old, New: next}
					plan.AddChange(v)
				},
				remove: func(c state.IdentityPrincipalIdentity, plan *Plan) {
					v := change(ActionDelete, c.GetName(), c.ID, nil, nil)
					v.Protection = false
					plan.AddChange(v)
				},
			}, childPlan)
		if err != nil {
			return err
		}
		if childPlan != plan {
			plan.Changes = childPlan.Changes
			plan.Warnings = childPlan.Warnings
		}
	}
	return nil
}

func principalIdentityFields(d resources.IdentityPrincipalIdentityResource) map[string]any {
	fields := d.PublicFields()
	fields[FieldManagedBy] = resources.BuildPrincipalManagedBy(
		resources.GetNamespace(d.Kongctl),
		d.Ref,
		d.Protected(),
		"",
	)
	return fields
}

func diffPrincipalIdentity(c state.IdentityPrincipalIdentity, d resources.IdentityPrincipalIdentityResource,
) (bool, map[string]any, map[string]FieldChange) {
	data, _ := json.Marshal(c.PrincipalIdentity)
	var observed map[string]any
	_ = json.Unmarshal(data, &observed)
	changed := make(map[string]FieldChange)
	for field, next := range d.PublicFields() {
		old := observed[field]
		if field == FieldLabels && old == nil {
			old = map[string]string{}
		}
		before, _ := json.Marshal(old)
		after, _ := json.Marshal(next)
		if string(before) != string(after) {
			changed[field] = FieldChange{Old: old, New: next}
		}
	}
	fields := principalIdentityFields(d)
	comparison := maps.Clone(fields[FieldManagedBy].(map[string]string))
	comparison[resources.ChildProtectedKey] = c.ManagedBy[resources.ChildProtectedKey]
	if (!resources.IsChildProtected(c.ManagedBy) || d.Protected()) && !managedByValuesEqual(c.ManagedBy, comparison) {
		changed[FieldManagedBy] = FieldChange{Old: c.ManagedBy, New: fields[FieldManagedBy]}
	}
	return len(changed) > 0, fields, changed
}

// The final cross-namespace plan must prove every existing child is explicitly
// deleted before its principal. Execution still requires a fresh empty list.
func (p *Planner) resolvePrincipalIdentityDeletes(ctx context.Context, plan *Plan) error {
	for i := range plan.Changes {
		parent := &plan.Changes[i]
		if parent.ResourceType != ResourceTypeIdentityPrincipal || parent.Action != ActionDelete {
			continue
		}
		if parent.Parent == nil || parent.Parent.ID == "" {
			return fmt.Errorf("principal deletion requires its directory")
		}
		all, err := p.listPrincipalIdentities(ctx, parent.Parent.ID, parent.ResourceID)
		if err != nil {
			return fmt.Errorf("cannot verify identities before deleting principal %q: %w", parent.ResourceRef, err)
		}
		for _, identity := range all {
			deletion := slices.IndexFunc(plan.Changes, func(c PlannedChange) bool {
				return c.ResourceType == ResourceTypeIdentityPrincipalIdentity && c.Action == ActionDelete &&
					c.ResourceID == identity.ID && c.Parent != nil && c.Parent.ID == parent.ResourceID &&
					c.References[FieldDirectoryID].ID == parent.Parent.ID
			})
			if deletion < 0 {
				return fmt.Errorf(
					"principal %q still has %d identities; remove them explicitly before deleting it",
					parent.ResourceRef,
					len(all),
				)
			}
			parent.DependsOn = append(parent.DependsOn, plan.Changes[deletion].ID)
		}
	}
	for i := range plan.Changes {
		child := &plan.Changes[i]
		if child.ResourceType != ResourceTypeIdentityPrincipalIdentity {
			continue
		}
		for _, other := range plan.Changes {
			if other.ResourceType == ResourceTypeIdentityPrincipal && other.Action == ActionDelete &&
				child.Parent != nil && child.Parent.ID == other.ResourceID && child.Action != ActionDelete {
				return fmt.Errorf("cannot write identity %q while deleting its principal", child.ResourceRef)
			}
			if other.Action == ActionCreate && (other.ResourceType == ResourceTypeIdentityPrincipal &&
				child.Parent != nil && child.Parent.Ref == other.ResourceRef ||
				other.ResourceType == ResourceTypeIdentityDirectory &&
					child.References[FieldDirectoryID].Ref == other.ResourceRef) {
				child.DependsOn = append(child.DependsOn, other.ID)
			}
		}
	}
	return nil
}
