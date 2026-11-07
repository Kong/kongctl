package planner

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) listIdentityBasicAuths(
	ctx context.Context,
	directoryID, principalID string,
) ([]state.IdentityBasicAuth, error) {
	key := directoryID + "/" + principalID
	if p.resourceCache != nil {
		if values, ok := p.resourceCache.identityBasicAuths[key]; ok {
			return values, nil
		}
	}
	values, err := p.client.ListIdentityBasicAuths(ctx, directoryID, principalID)
	if err == nil && p.resourceCache != nil {
		p.resourceCache.identityBasicAuths[key] = values
	}
	return values, err
}

func (p *identityDirectoryPlanner) planBasicAuths(ctx context.Context, config *Config, plan *Plan,
	directories map[string]state.IdentityDirectory,
) error {
	parents, err := p.principalChildScopes(ctx, config, directories, resources.ResourceTypeIdentityBasicAuth)
	if err != nil {
		return err
	}
	declared := make(map[string]bool)
	for _, parent := range parents {
		var desired []managedRoot[resources.IdentityBasicAuthResource]
		names, externalIDs := make(map[string]bool), make(map[string]bool)
		for _, basic := range p.planner.resources.Identity.BasicAuths {
			if !parent.refs[resources.NormalizeResourceRef(basic.Principal)] {
				continue
			}
			if basic.IsExternal() {
				externalIDs[basic.GetKonnectID()] = true
				continue
			}
			directory := parent.directoryID
			if directory == "" {
				directory = parent.directoryRef
			}
			key := directory + "\x00" + basic.Username
			if declared[key] {
				return fmt.Errorf(
					"multiple declarations manage basic-auth username %q in the same directory",
					basic.Username,
				)
			}
			declared[key] = true
			if resources.GetNamespace(basic.Kongctl) != config.Namespace {
				continue
			}
			names[basic.Username] = true
			desired = append(desired, managedRoot[resources.IdentityBasicAuthResource]{
				resource: basic, name: basic.Username, protected: basic.Protected(),
			})
		}
		if len(desired) == 0 && (plan.Metadata.Mode != PlanModeSync || !parent.scoped) {
			continue
		}
		var all []state.IdentityBasicAuth
		if parent.id != "" {
			all, err = p.planner.listIdentityBasicAuths(ctx, parent.directoryID, parent.id)
			if err != nil {
				return err
			}
		}
		seen := make(map[string]bool)
		var current []managedRoot[state.IdentityBasicAuth]
		for _, basic := range all {
			if seen[basic.Username] {
				return fmt.Errorf("multiple basic-auth credentials match the selected username")
			}
			seen[basic.Username] = true
			if names[basic.Username] {
				if externalIDs[basic.ID] {
					return fmt.Errorf("basic-auth username is declared as both managed and external")
				}
				if err := resources.ValidateChildOwnership("basic-auth", basic.ManagedBy, config.Namespace); err != nil {
					return fmt.Errorf("explicitly adopt the basic-auth credential before managing it: %w", err)
				}
			}
			if externalIDs[basic.ID] || basic.ManagedBy[resources.ManagedByToolKey] != resources.ManagedByTool ||
				basic.ManagedBy[resources.ChildNamespaceKey] != config.Namespace {
				continue
			}
			if err := resources.ValidateChildOwnership("basic-auth", basic.ManagedBy, config.Namespace); err != nil {
				return err
			}
			current = append(current, managedRoot[state.IdentityBasicAuth]{
				resource: basic, name: basic.Username, protected: resources.IsChildProtected(basic.ManagedBy),
			})
		}
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
				ID: p.NextChangeID(
					action,
					ResourceTypeIdentityBasicAuth,
					ref,
				), ResourceType: ResourceTypeIdentityBasicAuth,
				ResourceRef: ref, ResourceID: id, Action: action, Fields: fields, ChangedFields: changed,
				Namespace: config.Namespace, Parent: &ParentInfo{Ref: parent.ref, ID: parent.id},
				References: map[string]ReferenceInfo{
					FieldDirectoryID: {Ref: parent.directoryRef, ID: parent.directoryID},
				},
			}
		}
		update := func(c state.IdentityBasicAuth, d resources.IdentityBasicAuthResource, protection any, target *Plan) {
			fields := identityBasicAuthFields(d)
			value := change(ActionUpdate, d.Ref, c.ID, fields,
				map[string]FieldChange{FieldManagedBy: {Old: c.ManagedBy, New: fields[FieldManagedBy]}})
			value.Protection = protection
			target.AddChange(value)
		}
		err := reconcileManagedRoots(p.BasePlanner, ResourceTypeIdentityBasicAuth, desired, current,
			managedRootOperations[resources.IdentityBasicAuthResource, state.IdentityBasicAuth]{
				diff: diffIdentityBasicAuth,
				create: func(d resources.IdentityBasicAuthResource, target *Plan) string {
					value := change(ActionCreate, d.Ref, "", identityBasicAuthFields(d), nil)
					value.Protection = d.Protected()
					target.AddChange(value)
					return value.ID
				},
				update: func(c state.IdentityBasicAuth, d resources.IdentityBasicAuthResource,
					_ map[string]any, _ map[string]FieldChange, target *Plan,
				) {
					update(c, d, d.Protected(), target)
				},
				changeProtection: func(c state.IdentityBasicAuth, d resources.IdentityBasicAuthResource, old, next bool,
					_ map[string]any, _ map[string]FieldChange, target *Plan,
				) {
					update(c, d, ProtectionChange{Old: old, New: next}, target)
				},
				remove: func(c state.IdentityBasicAuth, target *Plan) {
					value := change(ActionDelete, c.ID, c.ID, map[string]any{FieldUsername: c.Username}, nil)
					value.Protection = false
					target.AddChange(value)
				},
			}, childPlan)
		if err != nil {
			return err
		}
		if childPlan != plan {
			plan.Changes, plan.Warnings = childPlan.Changes, childPlan.Warnings
		}
	}
	return nil
}

func identityBasicAuthFields(b resources.IdentityBasicAuthResource) map[string]any {
	return map[string]any{
		FieldUsername:  b.Username,
		FieldManagedBy: resources.BuildChildManagedBy(resources.GetNamespace(b.Kongctl), b.Protected(), ""),
	}
}

func diffIdentityBasicAuth(c state.IdentityBasicAuth, d resources.IdentityBasicAuthResource,
) (bool, map[string]any, map[string]FieldChange) {
	fields := identityBasicAuthFields(d)
	comparison := maps.Clone(fields[FieldManagedBy].(map[string]string))
	comparison[resources.ChildProtectedKey] = c.ManagedBy[resources.ChildProtectedKey]
	changed := (!resources.IsChildProtected(c.ManagedBy) || d.Protected()) &&
		!managedByValuesEqual(c.ManagedBy, comparison)
	return changed, fields, nil
}

func (p *Planner) resolvePrincipalCredentialDeletes(ctx context.Context, plan *Plan) error {
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.ResourceType == ResourceTypeIdentityBasicAuth {
			if change.Parent == nil {
				return fmt.Errorf("basic-auth operation requires its principal")
			}
			for _, other := range plan.Changes {
				if other.ResourceType == ResourceTypeIdentityPrincipal && other.Action == ActionDelete &&
					change.Parent.ID == other.ResourceID && change.Action != ActionDelete {
					return fmt.Errorf("cannot write basic-auth while deleting its principal")
				}
				if other.Action == ActionCreate && (other.ResourceType == ResourceTypeIdentityPrincipal &&
					change.Parent.Ref == other.ResourceRef || other.ResourceType == ResourceTypeIdentityDirectory &&
					change.References[FieldDirectoryID].Ref == other.ResourceRef) {
					change.DependsOn = append(change.DependsOn, other.ID)
				}
			}
			if change.Action == ActionDelete {
				if err := p.client.VerifyIdentityBasicAuthEmpty(ctx, change.References[FieldDirectoryID].ID,
					change.Parent.ID, change.ResourceID); err != nil {
					return err
				}
			}
		}
		if change.ResourceType != ResourceTypeIdentityPrincipal || change.Action != ActionDelete {
			continue
		}
		if change.Parent == nil || change.Parent.ID == "" {
			return fmt.Errorf("principal deletion requires its directory")
		}
		if err := p.verifyOrAuthorizeIdentityAPIKeys(ctx, plan, change); err != nil {
			return err
		}
		all, err := p.listIdentityBasicAuths(ctx, change.Parent.ID, change.ResourceID)
		if err != nil {
			return err
		}
		for _, child := range all {
			index := slices.IndexFunc(plan.Changes, func(c PlannedChange) bool {
				return c.ResourceType == ResourceTypeIdentityBasicAuth && c.Action == ActionDelete &&
					c.ResourceID == child.ID &&
					c.Parent != nil &&
					c.Parent.ID == change.ResourceID &&
					c.References[FieldDirectoryID].ID == change.Parent.ID
			})
			if index < 0 {
				return fmt.Errorf(
					"principal still has basic-auth credentials; remove them explicitly before deleting it",
				)
			}
			change.DependsOn = append(change.DependsOn, plan.Changes[index].ID)
		}
	}
	return nil
}
