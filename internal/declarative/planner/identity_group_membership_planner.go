package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/konnect/helpers"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func membershipFields(kind, id string) map[string]any {
	return map[string]any{FieldMember: map[string]any{FieldType: kind, FieldID: id}}
}

func (p *identityDirectoryPlanner) planMemberships(ctx context.Context, config *Config, plan *Plan) error {
	rs := p.planner.resources
	if rs.Identity == nil {
		return nil
	}
	for _, group := range resources.GetResourcesByNamespace[resources.IdentityGroupResource](rs, config.Namespace) {
		if group.IsExternal() {
			continue
		}
		var desired []resources.IdentityGroupMembershipResource
		for _, value := range rs.Identity.Memberships {
			if resources.NormalizeResourceRef(value.Group) == group.Ref {
				desired = append(desired, value)
			}
		}
		scoped := rs.SyncScope != nil &&
			rs.SyncScope.ChildInScope(
				resources.ResourceTypeIdentityGroup,
				group.Ref,
				resources.ResourceTypeIdentityGroupMembership,
			)
		if len(desired) == 0 && (plan.Metadata.Mode != PlanModeSync || !scoped) {
			continue
		}
		directory, found := rs.GetResourceByRef(resources.NormalizeResourceRef(group.Directory))
		if !found {
			return fmt.Errorf("group membership requires a declared directory")
		}
		directoryID, groupID := directory.GetKonnectID(), group.GetKonnectID()
		var observed []state.IdentityGroupMembership
		protected := false
		if groupID != "" {
			current, err := p.planner.client.GetIdentityGroup(ctx, directoryID, groupID)
			if err != nil {
				return err
			}
			if current == nil || current.Name != group.Name {
				return fmt.Errorf("membership group identity changed")
			}
			if err := resources.ValidateChildOwnership("membership group", current.ManagedBy, config.Namespace); err != nil {
				return err
			}
			if err := helpers.ValidateIdentityGroupManagement(current.IdentityGroup); err != nil {
				return err
			}
			protected = resources.IsChildProtected(current.ManagedBy)
			observed, err = p.planner.client.ListIdentityGroupMemberships(ctx, directoryID, groupID)
			if err != nil {
				return err
			}
		}
		byMember := make(map[string]state.IdentityGroupMembership)
		for _, value := range observed {
			byMember[value.GetName()] = value
		}
		seen := make(map[string]bool)
		add := func(action ActionType, ref, id, kind, memberID string) error {
			if (protected || group.Protected()) && action != ActionCreate {
				return fmt.Errorf("group %q is protected", group.Name)
			}
			change := PlannedChange{
				ID:           p.NextChangeID(action, ResourceTypeIdentityGroupMembership, ref),
				ResourceType: ResourceTypeIdentityGroupMembership, ResourceRef: ref, ResourceID: id,
				Action: action, Fields: membershipFields(kind, memberID), Namespace: config.Namespace, Protection: false,
				Parent: &ParentInfo{Ref: group.Ref, ID: groupID},
				References: map[string]ReferenceInfo{
					FieldDirectoryID: {Ref: directory.GetRef(), ID: directoryID},
				},
				ProtectingParent: &ProtectingParentInfo{
					ResourceType: ResourceTypeIdentityGroup,
					ResourceRef:  group.Ref,
					ResourceID:   groupID,
					ResourceName: group.Name,
				},
			}
			plan.AddChange(change)
			return nil
		}
		for _, value := range desired {
			if err := value.Validate(); err != nil {
				return err
			}
			memberID := value.Member.ID
			if tags.IsRefPlaceholder(memberID) {
				resolved, err := rs.ResolvePayloadReference(memberID)
				if err != nil {
					return err
				}
				var ok bool
				memberID, ok = resolved.(string)
				if !ok {
					return fmt.Errorf("membership reference must resolve to a principal ID string")
				}
				if ref, _, unresolved := tags.ParseRefPlaceholder(memberID); unresolved {
					target, ok := rs.GetResourceByRef(ref)
					if !ok || target.GetType() != resources.ResourceTypeIdentityPrincipal {
						return fmt.Errorf("membership must reference a principal")
					}
					parent := target.(*resources.IdentityPrincipalResource)
					principalDirectory, ok := rs.GetResourceByRef(resources.NormalizeResourceRef(parent.Directory))
					sameDirectory := ok && (principalDirectory.GetRef() == directory.GetRef() ||
						(directoryID != "" && directoryID == principalDirectory.GetKonnectID()))
					if !sameDirectory {
						return fmt.Errorf("membership principal must belong to the same directory as its group")
					}
				}
			}
			key := value.Member.Type + "/" + memberID
			if seen[key] {
				return fmt.Errorf("duplicate member under group %q", group.Name)
			}
			seen[key] = true
			current, exists := byMember[key]
			if exists {
				if target, ok := rs.GetResourceByRef(value.Ref); ok {
					target.(*resources.IdentityGroupMembershipResource).SetKonnectID(current.ID)
				}
			}
			if plan.Metadata.Mode == PlanModeDelete {
				if exists {
					if err := add(ActionDelete, value.Ref, current.ID, value.Member.Type, memberID); err != nil {
						return err
					}
				}
				continue
			}
			if exists {
				continue
			}
			if !tags.IsRefPlaceholder(memberID) {
				if directoryID == "" {
					return fmt.Errorf("literal principal IDs require an existing directory")
				}
				if err := p.planner.client.VerifyIdentityMembershipPrincipal(ctx, directoryID, memberID); err != nil {
					return err
				}
			}
			if err := add(ActionCreate, value.Ref, "", value.Member.Type, memberID); err != nil {
				return err
			}
		}
		if plan.Metadata.Mode == PlanModeSync && scoped {
			for _, current := range observed {
				if seen[current.GetName()] {
					continue
				}
				if current.Member.Type != kkComps.KongGroupMembershipTypePrincipal {
					return fmt.Errorf(
						"group has unsupported nested/imported relationships; implicit removal is deferred (Q32)",
					)
				}
				if err := add(ActionDelete, current.ID, current.ID, string(current.Member.Type), current.Member.ID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// The merged plan must explicitly authorize every observed detachment. Fresh
// executor checks still reject late memberships before principal/group deletion.
func (p *Planner) resolveIdentityMembershipDeletes(ctx context.Context, plan *Plan) error {
	deletion := func(directoryID, groupID, entryID string) *PlannedChange {
		for i := range plan.Changes {
			value := &plan.Changes[i]
			if value.ResourceType == ResourceTypeIdentityGroupMembership && value.Action == ActionDelete &&
				value.ResourceID == entryID && value.Parent != nil && value.Parent.ID == groupID &&
				value.References[FieldDirectoryID].ID == directoryID {
				return value
			}
		}
		return nil
	}
	for i := range plan.Changes {
		parent := &plan.Changes[i]
		if parent.Action != ActionDelete ||
			(parent.ResourceType != ResourceTypeIdentityGroup && parent.ResourceType != ResourceTypeIdentityPrincipal) {
			continue
		}
		if parent.Parent == nil || parent.Parent.ID == "" {
			return fmt.Errorf("identity deletion requires its directory")
		}
		directoryID := parent.Parent.ID
		var groups []state.IdentityGroup
		if parent.ResourceType == ResourceTypeIdentityGroup {
			current, err := p.client.GetIdentityGroup(ctx, directoryID, parent.ResourceID)
			if err != nil {
				return err
			}
			if current == nil {
				return fmt.Errorf("group disappeared while planning deletion")
			}
			groups = []state.IdentityGroup{*current}
			if err := p.verifyOrAuthorizeIdentityAPIKeys(ctx, plan, parent); err != nil {
				return err
			}
		} else {
			var err error
			groups, err = p.listIdentityGroups(ctx, directoryID)
			if err != nil {
				return err
			}
		}
		for _, group := range groups {
			entries, err := p.client.ListIdentityGroupMemberships(ctx, directoryID, group.ID)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if parent.ResourceType == ResourceTypeIdentityPrincipal &&
					(entry.Member.Type != kkComps.KongGroupMembershipTypePrincipal || entry.Member.ID != parent.ResourceID) {
					continue
				}
				detach := deletion(directoryID, group.ID, entry.ID)
				if detach == nil {
					if parent.ResourceType == ResourceTypeIdentityPrincipal {
						return fmt.Errorf(
							"principal %q still has a direct membership in group %q; remove it explicitly before deletion",
							parent.ResourceRef,
							group.Name,
						)
					}
					return fmt.Errorf(
						"%s %q still has group memberships; remove them explicitly before deletion",
						parent.ResourceType,
						parent.ResourceRef,
					)
				}
				parent.DependsOn = appendDependsOn(parent.DependsOn, detach.ID)
			}
		}
	}
	for i := range plan.Changes {
		entry := &plan.Changes[i]
		if entry.ResourceType != ResourceTypeIdentityGroupMembership || entry.Action != ActionCreate {
			continue
		}
		fields, _ := entry.Fields[FieldMember].(map[string]any)
		memberID, _ := fields[FieldID].(string)
		ref, _, tagged := tags.ParseRefPlaceholder(memberID)
		for _, other := range plan.Changes {
			if other.Action != ActionDelete {
				continue
			}
			if other.ResourceType == ResourceTypeIdentityGroup && entry.Parent != nil &&
				(entry.Parent.ID == other.ResourceID && other.ResourceID != "" || entry.Parent.Ref == other.ResourceRef) ||
				other.ResourceType == ResourceTypeIdentityPrincipal &&
					(memberID == other.ResourceID || tagged && ref == other.ResourceRef) {
				return fmt.Errorf("cannot add membership while deleting its group or principal")
			}
		}
		for _, other := range plan.Changes {
			if other.Action == ActionCreate && other.ResourceType == ResourceTypeIdentityDirectory &&
				other.ResourceRef == entry.References[FieldDirectoryID].Ref {
				entry.DependsOn = appendDependsOn(entry.DependsOn, other.ID)
			}
		}
	}
	return nil
}
