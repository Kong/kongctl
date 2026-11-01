package planner

import (
	"context"
	"fmt"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/identitytrust"
)

func identityTrustedIDPFields(value resources.IdentityTrustedIDPResource) map[string]any {
	fields := map[string]any{FieldIssuerURL: value.IssuerURL}
	putNullable := func(key string, present bool, value *string) {
		if !present {
			return
		}
		if value == nil {
			fields[key] = nil
		} else {
			fields[key] = *value
		}
	}
	putNullable(FieldDisplayName, value.DisplayNameDeclared(), value.DisplayName)
	putNullable(FieldJwksURI, value.JwksURIDeclared(), value.JwksURI)
	if value.JitProvisioningEnabled != nil {
		fields[FieldJitProvisioningEnabled] = *value.JitProvisioningEnabled
	}
	if value.VaultAccessEnabled != nil {
		fields[FieldVaultAccessEnabled] = *value.VaultAccessEnabled
	}
	if value.Jit != nil {
		fields[FieldJit] = value.Jit
	}
	return fields
}

func identityTrustedIDPObservedFields(value state.IdentityTrustedIDP) map[string]any {
	fields := map[string]any{
		FieldIssuerURL: value.IssuerURL, FieldDisplayName: nil, FieldJwksURI: nil, FieldJit: value.Jit,
		FieldJitProvisioningEnabled: *value.JitProvisioningEnabled, FieldVaultAccessEnabled: *value.VaultAccessEnabled,
	}
	if value.DisplayName != nil {
		fields[FieldDisplayName] = *value.DisplayName
	}
	if value.JwksURI != nil {
		fields[FieldJwksURI] = *value.JwksURI
	}
	return fields
}

func (p *identityDirectoryPlanner) planTrustedIDPs(ctx context.Context, config *Config, plan *Plan,
	byName map[string]state.IdentityDirectory,
) error {
	rs := p.planner.resources
	if rs.Identity == nil {
		return nil
	}
	directories := resources.GetResourcesByNamespace[resources.IdentityDirectoryResource](rs, config.Namespace)
	for _, directory := range directories {
		if directory.IsExternal() {
			continue
		}
		var desired []resources.IdentityTrustedIDPResource
		for _, value := range rs.Identity.TrustedIDPs {
			if resources.NormalizeResourceRef(value.Directory) == directory.Ref {
				desired = append(desired, value)
			}
		}
		scoped := rs.SyncScope != nil && rs.SyncScope.ChildInScope(resources.ResourceTypeIdentityDirectory,
			directory.Ref, resources.ResourceTypeIdentityTrustedIDP)
		if len(desired) == 0 && (plan.Metadata.Mode != PlanModeSync || !scoped) {
			continue
		}
		current, exists := byName[directory.Name]
		var observed []state.IdentityTrustedIDP
		if exists {
			if current.VaultEnabled == nil || !*current.VaultEnabled {
				return fmt.Errorf(
					"directory %q Trusted IdPs cannot be observed; explicitly enable Vault first",
					directory.Name,
				)
			}
			var err error
			observed, err = p.planner.client.ListIdentityTrustedIDPs(ctx, current.ID)
			if err != nil {
				return err
			}
		} else if directory.VaultEnabled == nil || !*directory.VaultEnabled {
			return fmt.Errorf("new directory %q requires explicit vault_enabled: true for Trusted IdPs", directory.Name)
		}
		if directory.VaultEnabled != nil && !*directory.VaultEnabled && plan.Metadata.Mode != PlanModeDelete {
			return fmt.Errorf("cannot manage Trusted IdPs while disabling directory %q Vault", directory.Name)
		}
		byIssuer := make(map[string]state.IdentityTrustedIDP, len(observed))
		for _, value := range observed {
			byIssuer[value.IssuerURL] = value
		}
		retained := make(map[string]bool)
		start := len(plan.Changes)
		add := func(action ActionType, ref, id string, fields map[string]any, changed map[string]FieldChange) error {
			if exists && resources.IsChildProtected(current.ManagedBy) && action != ActionCreate {
				return fmt.Errorf("directory %q is protected", directory.Name)
			}
			plan.AddChange(PlannedChange{
				ID: p.NextChangeID(
					action,
					ResourceTypeIdentityTrustedIDP,
					ref,
				), ResourceType: ResourceTypeIdentityTrustedIDP,
				ResourceRef: ref, ResourceID: id, Action: action, Fields: fields, ChangedFields: changed,
				Namespace: config.Namespace, Protection: false, Parent: &ParentInfo{Ref: directory.Ref, ID: current.ID},
				ProtectingParent: &ProtectingParentInfo{
					ResourceType: ResourceTypeIdentityDirectory,
					ResourceRef:  directory.Ref, ResourceID: current.ID, ResourceName: directory.Name,
				},
			})
			return nil
		}
		for _, value := range desired {
			retained[value.IssuerURL] = true
			remote, found := byIssuer[value.IssuerURL]
			if found {
				if target, ok := rs.GetResourceByRef(value.Ref); ok {
					target.(*resources.IdentityTrustedIDPResource).SetKonnectID(remote.ID)
				}
			}
			if plan.Metadata.Mode == PlanModeDelete {
				if found {
					if err := add(ActionDelete, value.Ref, remote.ID,
						map[string]any{FieldIssuerURL: remote.IssuerURL}, nil); err != nil {
						return err
					}
				}
				continue
			}
			fields := identityTrustedIDPFields(value)
			if !found {
				if err := add(ActionCreate, value.Ref, "", fields, nil); err != nil {
					return err
				}
				continue
			}
			actual := identityTrustedIDPObservedFields(remote)
			changed := make(map[string]FieldChange)
			for field, next := range fields {
				equal := reflect.DeepEqual(actual[field], next)
				if field == FieldJit {
					equal = reflect.DeepEqual(
						identitytrust.NormalizeJIT(remote.Jit),
						identitytrust.NormalizeJIT(value.Jit),
					)
				}
				if !equal {
					changed[field] = FieldChange{Old: actual[field], New: next}
				}
			}
			if len(changed) > 0 {
				if err := add(ActionUpdate, value.Ref, remote.ID, fields, changed); err != nil {
					return err
				}
			}
		}
		if plan.Metadata.Mode == PlanModeSync && scoped {
			for _, value := range observed {
				if !retained[value.IssuerURL] {
					if err := add(ActionDelete, "trusted-idp-"+value.ID, value.ID,
						map[string]any{FieldIssuerURL: value.IssuerURL}, nil); err != nil {
						return err
					}
				}
			}
		}
		for i := start; i < len(plan.Changes); i++ {
			change := &plan.Changes[i]
			for j := range plan.Changes {
				other := &plan.Changes[j]
				if change.Action == ActionDelete && other.ResourceType == ResourceTypeIdentityTrustedIDP &&
					other.Action == ActionCreate && other.Parent != nil && other.Parent.Ref == directory.Ref {
					change.DependsOn = appendDependsOn(change.DependsOn, other.ID)
				}
				if other.ResourceType == ResourceTypeIdentityDirectory && other.Action == ActionCreate &&
					other.ResourceRef == directory.Ref {
					change.DependsOn = appendDependsOn(change.DependsOn, other.ID)
				}
			}
		}
	}
	return nil
}
