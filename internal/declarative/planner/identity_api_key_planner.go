package planner

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

type (
	identityAPIKeyDesired struct {
		fields resources.IdentityAPIKeyFields
		target interface{ SetKonnectID(string) }
	}
	identityAPIKeyParent struct {
		kind, parentKind                         resources.ResourceType
		ref, id, name, directoryRef, directoryID string
		protected, scoped, external              bool
		desired                                  []identityAPIKeyDesired
	}
)

func (p *identityDirectoryPlanner) planAPIKeys(ctx context.Context, config *Config, plan *Plan,
	directories map[string]state.IdentityDirectory,
) error {
	rs := p.planner.resources
	if rs.Identity == nil {
		return nil
	}
	var parents []identityAPIKeyParent
	principalScopes, err := p.principalChildScopes(
		ctx,
		config,
		directories,
		resources.ResourceTypeIdentityPrincipalAPIKey,
	)
	if err != nil {
		return err
	}
	for _, scope := range principalScopes {
		parentResource, found := rs.GetResourceByRef(scope.ref)
		if !found {
			return fmt.Errorf("API key requires its declared principal")
		}
		parent := parentResource.(*resources.IdentityPrincipalResource)
		if !parent.IsExternal() && resources.GetNamespace(parent.Kongctl) != config.Namespace {
			continue
		}
		value := identityAPIKeyParent{
			kind: resources.ResourceTypeIdentityPrincipalAPIKey, parentKind: resources.ResourceTypeIdentityPrincipal,
			ref: parent.Ref, id: scope.id, name: parent.Ref,
			directoryRef: scope.directoryRef, directoryID: scope.directoryID, protected: parent.Protected(),
			external: parent.IsExternal(), scoped: scope.scoped,
		}
		for i := range rs.Identity.PrincipalAPIKeys {
			key := &rs.Identity.PrincipalAPIKeys[i]
			if scope.refs[resources.NormalizeResourceRef(key.Principal)] &&
				(!parent.IsExternal() || resources.GetNamespace(key.Kongctl) == config.Namespace) {
				value.desired = append(value.desired, identityAPIKeyDesired{key.IdentityAPIKeyFields, key})
			}
		}
		parents = append(parents, value)
	}
	for _, parent := range resources.GetResourcesByNamespace[resources.IdentityGroupResource](rs, config.Namespace) {
		if parent.IsExternal() {
			continue
		}
		directoryResource, found := rs.GetResourceByRef(resources.NormalizeResourceRef(parent.Directory))
		if !found {
			return fmt.Errorf("API key requires a declared directory")
		}
		directory := directoryResource.(*resources.IdentityDirectoryResource)
		directoryID := directory.GetKonnectID()
		if current, found := directories[directory.Name]; found && !directory.IsExternal() {
			directoryID = current.ID
		}
		value := identityAPIKeyParent{
			kind: resources.ResourceTypeIdentityGroupAPIKey, parentKind: resources.ResourceTypeIdentityGroup,
			ref: parent.Ref, name: parent.Name,
			directoryRef: directory.Ref, directoryID: directoryID, protected: parent.Protected(),
		}
		if directoryID != "" {
			all, err := p.planner.listIdentityGroups(ctx, directoryID)
			if err != nil {
				return err
			}
			for _, current := range all {
				if current.Name == parent.Name {
					if value.id != "" {
						return fmt.Errorf("multiple groups match API-key parent")
					}
					value.id = current.ID
				}
			}
		}
		for i := range rs.Identity.GroupAPIKeys {
			key := &rs.Identity.GroupAPIKeys[i]
			if resources.NormalizeResourceRef(key.Group) == parent.Ref {
				value.desired = append(value.desired, identityAPIKeyDesired{key.IdentityAPIKeyFields, key})
			}
		}
		parents = append(parents, value)
	}
	for _, parent := range parents {
		if !parent.external {
			parent.scoped = rs.SyncScope != nil && rs.SyncScope.ChildInScope(parent.parentKind, parent.ref, parent.kind)
		}
		if len(parent.desired) == 0 && (plan.Metadata.Mode != PlanModeSync || !parent.scoped) {
			continue
		}
		if err := p.planAPIKeyParent(ctx, config, plan, parent); err != nil {
			return err
		}
	}
	return nil
}

func (p *identityDirectoryPlanner) planAPIKeyParent(
	ctx context.Context,
	config *Config,
	plan *Plan,
	parent identityAPIKeyParent,
) error {
	var all []state.IdentityAPIKey
	if parent.id != "" {
		var protected bool
		var err error
		if parent.external {
			err = p.planner.client.VerifyIdentityMembershipPrincipal(ctx, parent.directoryID, parent.id)
		} else {
			protected, err = p.planner.client.VerifyIdentityAPIKeyParent(
				ctx,
				parent.kind,
				parent.directoryID,
				parent.id,
				config.Namespace,
				parent.name,
			)
		}
		if err != nil {
			return err
		}
		parent.protected = parent.protected || protected
		all, err = p.planner.client.ListIdentityAPIKeys(ctx, parent.kind, parent.directoryID, parent.id)
		if err != nil {
			return err
		}
	}
	var namespace []string
	if parent.external {
		namespace = []string{config.Namespace}
	}
	indexed, err := state.IndexIdentityAPIKeys(all, namespace...)
	if err != nil {
		return err
	}
	seen := make(map[string]bool)
	add := func(action ActionType, ref, id string, fields map[string]any, changed map[string]FieldChange) error {
		if parent.protected && action != ActionCreate {
			return fmt.Errorf("API-key parent %q is protected", parent.ref)
		}
		change := PlannedChange{
			ID: p.NextChangeID(
				action,
				string(parent.kind),
				ref,
			), ResourceType: string(parent.kind), ResourceRef: ref, ResourceID: id, Action: action,
			Fields: fields, ChangedFields: changed, Namespace: config.Namespace, Protection: false,
			Parent: &ParentInfo{Ref: parent.ref, ID: parent.id},
			References: map[string]ReferenceInfo{
				FieldDirectoryID: {Ref: parent.directoryRef, ID: parent.directoryID},
			},
			ProtectingParent: &ProtectingParentInfo{
				ResourceType: string(parent.parentKind),
				ResourceRef:  parent.ref,
				ResourceID:   parent.id,
				ResourceName: parent.name,
			},
		}
		if parent.external {
			change.ProtectingParent = nil
		}
		plan.AddChange(change)
		return nil
	}
	for _, desired := range parent.desired {
		if err := desired.fields.Validate(); err != nil {
			return err
		}
		if seen[desired.fields.Ref] {
			return fmt.Errorf("multiple declarations match API-key ref %q", desired.fields.Ref)
		}
		seen[desired.fields.Ref] = true
		current, err := state.FindIdentityAPIKey(all, desired.fields.Ref, namespace...)
		if err != nil {
			return err
		}
		if current != nil {
			desired.target.SetKonnectID(current.ID)
		}
		if plan.Metadata.Mode == PlanModeDelete {
			if current != nil {
				if err := add(ActionDelete, desired.fields.Ref, current.ID, nil, nil); err != nil {
					return err
				}
			}
			continue
		}
		if current == nil {
			fields := map[string]any{
				FieldType:      desired.fields.Type,
				FieldManagedBy: map[string]string{resources.ManagedByRefKey: desired.fields.Ref},
			}
			if parent.external {
				fields[FieldManagedBy].(map[string]string)[resources.ChildNamespaceKey] = config.Namespace
			}
			if desired.fields.NameDeclared() {
				fields[FieldName] = identityAPIKeyName(desired.fields.Name)
			}
			if desired.fields.ExpirationDeclared() {
				fields[FieldExpiresAt] = apiKeyExpiration(desired.fields.ExpiresAt)
			}
			if err := add(ActionCreate, desired.fields.Ref, "", fields, nil); err != nil {
				return err
			}
			continue
		}
		fields := make(map[string]any)
		changed := make(map[string]FieldChange)
		if desired.fields.NameDeclared() && !sameAPIKeyName(current.Name, desired.fields.Name) {
			fields[FieldName] = identityAPIKeyName(desired.fields.Name)
			changed[FieldName] = FieldChange{Old: current.Name, New: identityAPIKeyName(desired.fields.Name)}
		}
		if desired.fields.ExpirationDeclared() && !sameAPIKeyExpiration(current.ExpiresAt, desired.fields.ExpiresAt) {
			fields[FieldExpiresAt] = apiKeyExpiration(desired.fields.ExpiresAt)
			changed[FieldExpiresAt] = FieldChange{Old: apiKeyExpiration(current.ExpiresAt), New: fields[FieldExpiresAt]}
		}
		if len(fields) > 0 {
			if err := add(ActionUpdate, desired.fields.Ref, current.ID, fields, changed); err != nil {
				return err
			}
		}
	}
	if plan.Metadata.Mode == PlanModeSync && parent.scoped {
		var refs []string
		for ref := range indexed {
			refs = append(refs, ref)
		}
		slices.Sort(refs)
		for _, ref := range refs {
			current := indexed[ref]
			if !seen[ref] {
				if err := add(ActionDelete, ref, current.ID, nil, nil); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func apiKeyExpiration(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func sameAPIKeyName(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func sameAPIKeyExpiration(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}

func identityAPIKeyKind(parentKind string) resources.ResourceType {
	if parentKind == ResourceTypeIdentityPrincipal {
		return resources.ResourceTypeIdentityPrincipalAPIKey
	}
	return resources.ResourceTypeIdentityGroupAPIKey
}

func (p *Planner) authorizeIdentityAPIKeyDeletes(ctx context.Context, plan *Plan, parent *PlannedChange) error {
	kind := identityAPIKeyKind(parent.ResourceType)
	values, err := p.client.ListIdentityAPIKeys(ctx, kind, parent.Parent.ID, parent.ResourceID)
	if err != nil {
		return err
	}
	for _, value := range values {
		found := false
		for _, child := range plan.Changes {
			if child.ResourceType == string(kind) && child.Action == ActionDelete && child.ResourceID == value.ID &&
				child.Parent != nil &&
				child.Parent.ID == parent.ResourceID &&
				child.References[FieldDirectoryID].ID == parent.Parent.ID {
				parent.DependsOn = appendDependsOn(parent.DependsOn, child.ID)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf(
				"%s %q still has API keys; explicitly authorize each key retirement before parent deletion",
				parent.ResourceType,
				parent.ResourceRef,
			)
		}
	}
	return nil
}

func (p *Planner) verifyOrAuthorizeIdentityAPIKeys(ctx context.Context, plan *Plan, parent *PlannedChange) error {
	kind := identityAPIKeyKind(parent.ResourceType)
	for _, child := range plan.Changes {
		if child.ResourceType == string(kind) && child.Action == ActionDelete && child.Parent != nil &&
			child.Parent.ID == parent.ResourceID &&
			child.References[FieldDirectoryID].ID == parent.Parent.ID {
			return p.authorizeIdentityAPIKeyDeletes(ctx, plan, parent)
		}
	}
	if parent.ResourceType == ResourceTypeIdentityPrincipal {
		return p.client.VerifyPrincipalAPIKeysEmpty(ctx, parent.Parent.ID, parent.ResourceID)
	}
	return p.client.VerifyIdentityGroupKeysEmpty(ctx, parent.Parent.ID, parent.ResourceID)
}

func (p *Planner) resolveIdentityAPIKeyDependencies(plan *Plan) error {
	for i := range plan.Changes {
		key := &plan.Changes[i]
		if key.ResourceType != ResourceTypeIdentityPrincipalAPIKey &&
			key.ResourceType != ResourceTypeIdentityGroupAPIKey {
			continue
		}
		if key.Parent == nil {
			return fmt.Errorf("API key requires an explicit parent")
		}
		parentKind := ResourceTypeIdentityPrincipal
		if key.ResourceType == ResourceTypeIdentityGroupAPIKey {
			parentKind = ResourceTypeIdentityGroup
		}
		for _, other := range plan.Changes {
			if other.ResourceType == parentKind && other.ResourceRef == key.Parent.Ref &&
				other.Action == ActionDelete &&
				key.Action != ActionDelete {
				return fmt.Errorf("cannot write an API key while deleting its parent")
			}
			createsParent := other.ResourceType == parentKind && other.ResourceRef == key.Parent.Ref
			createsDirectory := other.ResourceType == ResourceTypeIdentityDirectory &&
				other.ResourceRef == key.References[FieldDirectoryID].Ref
			if other.Action == ActionCreate && (createsParent || createsDirectory) {
				key.DependsOn = appendDependsOn(key.DependsOn, other.ID)
			}
			if key.Action == ActionDelete && other.Action == ActionCreate && other.ResourceType == key.ResourceType &&
				other.Parent != nil &&
				other.Parent.Ref == key.Parent.Ref &&
				other.References[FieldDirectoryID].Ref == key.References[FieldDirectoryID].Ref {
				key.DependsOn = appendDependsOn(key.DependsOn, other.ID)
			}
		}
	}
	return nil
}

func identityAPIKeyName(name *string) any {
	if name == nil {
		return nil
	}
	return *name
}
