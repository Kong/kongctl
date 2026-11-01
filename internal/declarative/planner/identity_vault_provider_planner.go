package planner

import (
	"context"
	"fmt"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func identityVaultProviderFields(p resources.IdentityVaultProviderResource) map[string]any {
	fields := map[string]any{FieldName: p.Name, FieldTemplateName: p.TemplateName}
	putString := func(key string, value *string) {
		if value != nil {
			fields[key] = *value
		}
	}
	putBool := func(key string, value *bool) {
		if value != nil {
			fields[key] = *value
		}
	}
	putString(FieldClientID, p.ClientID)
	putString(FieldClientSecret, p.ClientSecret)
	putString(FieldCredentialType, p.CredentialType)
	putString(FieldTokenEndpoint, p.TokenEndpoint)
	putString(FieldAuthorizationEndpoint, p.AuthorizationEndpoint)
	putString(FieldRegistrationEndpoint, p.RegistrationEndpoint)
	putBool(FieldDCREnabled, p.DCREnabled)
	putBool(FieldPKCEEnabled, p.PKCEEnabled)
	putBool(FieldEnabled, p.Enabled)
	if p.RefreshBufferSeconds != nil {
		fields[FieldRefreshBufferSeconds] = *p.RefreshBufferSeconds
	}
	if p.Scopes != nil {
		fields[FieldScopes] = p.Scopes
	}
	return fields
}

func identityVaultProviderObservedFields(p state.IdentityVaultProvider) map[string]any {
	fields := map[string]any{
		FieldName: p.Name, FieldTemplateName: p.TemplateName, FieldCredentialType: p.CredentialType,
		FieldEnabled: *p.Enabled, FieldDCREnabled: *p.DCREnabled, FieldPKCEEnabled: *p.PKCEEnabled,
		FieldScopes: p.Scopes, FieldRefreshBufferSeconds: *p.RefreshBufferSeconds,
	}
	for key, value := range map[string]*string{
		FieldClientID: p.ClientID, FieldTokenEndpoint: p.TokenEndpointEffective,
		FieldAuthorizationEndpoint: p.AuthorizationEndpointEffective,
		FieldRegistrationEndpoint:  p.RegistrationEndpointEffective,
	} {
		if value != nil {
			fields[key] = *value
		}
	}
	return fields
}

func identityVaultProviderCreateOnlyField(field string) bool {
	switch field {
	case FieldTemplateName, FieldClientID, FieldCredentialType, FieldDCREnabled, FieldRegistrationEndpoint:
		return true
	}
	return false
}

func (p *identityDirectoryPlanner) planVaultProviders(ctx context.Context, config *Config, plan *Plan,
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
		var desired []resources.IdentityVaultProviderResource
		for _, provider := range rs.Identity.VaultProviders {
			if resources.NormalizeResourceRef(provider.Directory) == directory.Ref {
				desired = append(desired, provider)
			}
		}
		scoped := rs.SyncScope != nil && rs.SyncScope.ChildInScope(resources.ResourceTypeIdentityDirectory,
			directory.Ref, resources.ResourceTypeIdentityVaultProvider)
		if len(desired) == 0 && (plan.Metadata.Mode != PlanModeSync || !scoped) {
			continue
		}
		current, exists := byName[directory.Name]
		var observed []state.IdentityVaultProvider
		if exists {
			if current.VaultEnabled == nil || !*current.VaultEnabled {
				return fmt.Errorf(
					"directory %q Vault providers cannot be observed; explicitly enable Vault first",
					directory.Name,
				)
			}
			var err error
			observed, err = p.planner.client.ListIdentityVaultProviders(ctx, current.ID)
			if err != nil {
				return err
			}
			// Secret-only updates need the actual root ID even without root drift.
			if root, ok := rs.GetResourceByRef(directory.Ref); ok {
				root.(*resources.IdentityDirectoryResource).SetKonnectID(current.ID)
			}
		} else if directory.VaultEnabled == nil || !*directory.VaultEnabled {
			return fmt.Errorf("new directory %q requires explicit vault_enabled: true for Vault providers", directory.Name)
		}
		if directory.VaultEnabled != nil && !*directory.VaultEnabled && plan.Metadata.Mode != PlanModeDelete {
			return fmt.Errorf("cannot manage Vault providers while disabling directory %q Vault", directory.Name)
		}
		byProviderName := make(map[string]state.IdentityVaultProvider, len(observed))
		for _, provider := range observed {
			byProviderName[provider.Name] = provider
		}
		retained := make(map[string]bool)
		start := len(plan.Changes)
		protected := exists && resources.IsChildProtected(current.ManagedBy)
		add := func(action ActionType, ref, id string, fields map[string]any, changed map[string]FieldChange) error {
			if protected && action != ActionCreate {
				return fmt.Errorf("directory %q is protected", directory.Name)
			}
			if action == ActionDelete {
				if err := p.planner.client.CheckIdentityVaultProviderCredentials(ctx, current.ID, id); err != nil {
					return err
				}
			}
			plan.AddChange(PlannedChange{
				ID: p.NextChangeID(
					action,
					ResourceTypeIdentityVaultProvider,
					ref,
				), ResourceType: ResourceTypeIdentityVaultProvider,
				ResourceRef: ref, ResourceID: id, Action: action, Fields: fields, ChangedFields: changed,
				Namespace: config.Namespace, Protection: false, Parent: &ParentInfo{Ref: directory.Ref, ID: current.ID},
				ProtectingParent: &ProtectingParentInfo{
					ResourceType: ResourceTypeIdentityDirectory,
					ResourceRef:  directory.Ref, ResourceID: current.ID, ResourceName: directory.Name,
				},
			})
			return nil
		}
		for _, provider := range desired {
			retained[provider.Name] = true
			value, found := byProviderName[provider.Name]
			if found {
				if target, ok := rs.GetResourceByRef(provider.Ref); ok {
					target.(*resources.IdentityVaultProviderResource).SetKonnectID(value.ID)
				}
			}
			if plan.Metadata.Mode == PlanModeDelete {
				if found {
					if err := add(ActionDelete, provider.Ref, value.ID, map[string]any{
						FieldName: value.Name, FieldTemplateName: value.TemplateName,
					}, nil); err != nil {
						return err
					}
				}
				continue
			}
			fields := identityVaultProviderFields(provider)
			if !found {
				template, err := p.planner.client.GetIdentityVaultProviderTemplate(ctx, provider.TemplateName)
				if err != nil {
					return err
				}
				if template.PKCERequired != nil && *template.PKCERequired && provider.PKCEEnabled != nil &&
					!*provider.PKCEEnabled {
					return fmt.Errorf("provider %q template requires PKCE", provider.Ref)
				}
				if err := add(ActionCreate, provider.Ref, "", fields, nil); err != nil {
					return err
				}
				continue
			}
			actual := identityVaultProviderObservedFields(value)
			changed := make(map[string]FieldChange)
			for field, next := range fields {
				if field == FieldClientSecret {
					continue
				}
				if !reflect.DeepEqual(actual[field], next) {
					if identityVaultProviderCreateOnlyField(field) {
						return fmt.Errorf(
							"provider %q field %s cannot be changed through the verified PATCH API; explicitly replace the provider",
							provider.Ref,
							field,
						)
					}
					changed[field] = FieldChange{Old: actual[field], New: next}
				}
			}
			if len(changed) > 0 {
				if err := add(ActionUpdate, provider.Ref, value.ID, fields, changed); err != nil {
					return err
				}
			}
		}
		if plan.Metadata.Mode == PlanModeSync && scoped {
			for _, value := range observed {
				if !retained[value.Name] {
					if err := add(ActionDelete, "vault-provider-"+value.ID, value.ID, map[string]any{
						FieldName: value.Name, FieldTemplateName: value.TemplateName,
					}, nil); err != nil {
						return err
					}
				}
			}
		}
		for i := start; i < len(plan.Changes); i++ {
			change := &plan.Changes[i]
			for j := range plan.Changes {
				other := &plan.Changes[j]
				// Keep existing configurations if registering a replacement fails.
				if change.Action == ActionDelete && other.ResourceType == ResourceTypeIdentityVaultProvider &&
					other.Action == ActionCreate && other.Parent != nil && other.Parent.Ref == directory.Ref {
					change.DependsOn = appendDependsOn(change.DependsOn, other.ID)
				}
				if other.ResourceType == ResourceTypeIdentityDirectory {
					if other.Action == ActionCreate && other.ResourceRef == directory.Ref {
						change.DependsOn = appendDependsOn(change.DependsOn, other.ID)
					}
					if other.Action == ActionDelete && other.ResourceID == current.ID && change.Action == ActionDelete {
						other.DependsOn = appendDependsOn(other.DependsOn, change.ID)
					}
				}
			}
		}
	}
	return nil
}
