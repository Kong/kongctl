package planner

import (
	"context"
	"fmt"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func identityStaticCredentialFields(selector *string, priority *int64) map[string]any {
	fields := make(map[string]any)
	if selector != nil {
		fields[FieldSelector] = *selector
		fields[FieldPriority] = *priority
	}
	return fields
}

func identityStaticCredentialSlots(
	values []state.IdentityVaultProviderCredential,
) (map[string]state.IdentityVaultProviderCredential, error) {
	result := make(map[string]state.IdentityVaultProviderCredential)
	for _, value := range values {
		if value.CredentialType != "shared" || value.Status != "active" {
			return nil, fmt.Errorf("static provider has an unexpected credential type or status")
		}
		key := resources.IdentityVaultCredentialSlot(value.Priority)
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate credential routing slot; inspect before writing")
		}
		result[key] = value
	}
	return result, nil
}

func (p *identityDirectoryPlanner) planVaultProviderCredentials(ctx context.Context, config *Config, plan *Plan) error {
	rs := p.planner.resources
	if rs.Identity == nil {
		return nil
	}
	providers := resources.GetResourcesByNamespace[resources.IdentityVaultProviderResource](rs, config.Namespace)
	for _, provider := range providers {
		var desired []resources.IdentityVaultProviderCredentialResource
		for _, credential := range rs.Identity.VaultProviderCredentials {
			if resources.NormalizeResourceRef(credential.Provider) == provider.Ref {
				desired = append(desired, credential)
			}
		}
		scoped := rs.SyncScope != nil &&
			rs.SyncScope.ChildInScope(
				resources.ResourceTypeIdentityVaultProvider,
				provider.Ref,
				resources.ResourceTypeIdentityVaultProviderCredential,
			)
		if len(desired) == 0 && (plan.Metadata.Mode != PlanModeSync || !scoped) {
			continue
		}
		if provider.TemplateName != "static_secret" || provider.CredentialType == nil ||
			*provider.CredentialType != "shared" {
			return fmt.Errorf("shared credential declarations require a static_secret/shared provider")
		}
		directoryResource, ok := rs.GetResourceByRef(resources.NormalizeResourceRef(provider.Directory))
		if !ok {
			return fmt.Errorf("credential provider has no declared directory")
		}
		directory, ok := directoryResource.(*resources.IdentityDirectoryResource)
		if !ok || directory.IsExternal() {
			return fmt.Errorf("credential provider requires a managed directory")
		}
		var observed []state.IdentityVaultProviderCredential
		if provider.GetKonnectID() != "" {
			var err error
			observed, err = p.planner.client.ListIdentityVaultProviderCredentials(
				ctx,
				directory.GetKonnectID(),
				provider.GetKonnectID(),
			)
			if err != nil {
				return err
			}
		}
		bySlot, err := identityStaticCredentialSlots(observed)
		if err != nil {
			return err
		}
		retained := make(map[string]bool)
		add := func(action ActionType, ref, id string, fields map[string]any, changed map[string]FieldChange) {
			fields[FieldProviderName] = provider.Name
			plan.AddChange(PlannedChange{
				ID: p.NextChangeID(
					action,
					ResourceTypeIdentityVaultProviderCredential,
					ref,
				), ResourceType: ResourceTypeIdentityVaultProviderCredential,
				ResourceRef: ref, ResourceID: id, Action: action, Fields: fields, ChangedFields: changed,
				Namespace: config.Namespace, Protection: false,
				Parent: &ParentInfo{
					Ref: provider.Ref,
					ID:  provider.GetKonnectID(),
				}, References: map[string]ReferenceInfo{FieldDirectoryID: {Ref: directory.Ref, ID: directory.GetKonnectID()}},
			})
		}
		for _, credential := range desired {
			slot := credential.GetMoniker()
			retained[slot] = true
			current, exists := bySlot[slot]
			if exists {
				if target, ok := rs.GetResourceByRef(credential.Ref); ok {
					target.(*resources.IdentityVaultProviderCredentialResource).SetKonnectID(current.ID)
				}
			}
			if plan.Metadata.Mode == PlanModeDelete {
				if exists {
					add(
						ActionDelete,
						credential.Ref,
						current.ID,
						identityStaticCredentialFields(current.Selector, current.Priority),
						nil,
					)
				}
				continue
			}
			fields := identityStaticCredentialFields(credential.Selector, credential.Priority)
			if !exists {
				add(ActionCreate, credential.Ref, "", fields, nil)
				continue
			}
			if !reflect.DeepEqual(current.Selector, credential.Selector) {
				add(
					ActionUpdate,
					credential.Ref,
					current.ID,
					fields,
					map[string]FieldChange{FieldSelector: {Old: current.Selector, New: credential.Selector}},
				)
			}
		}
		if plan.Metadata.Mode == PlanModeSync && scoped {
			for _, current := range observed {
				if !retained[resources.IdentityVaultCredentialSlot(current.Priority)] {
					add(
						ActionDelete,
						"vault-credential-"+current.ID,
						current.ID,
						identityStaticCredentialFields(current.Selector, current.Priority),
						nil,
					)
				}
			}
		}
	}
	return nil
}

// Secret-only updates are synthesized after namespace planning. Bind them to
// the same complete public scope and fresh routing snapshot as ordinary changes.
func (p *Planner) prepareIdentityVaultCredentialChanges(ctx context.Context, plan *Plan) error {
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.ResourceType != ResourceTypeIdentityVaultProviderCredential {
			continue
		}
		if change.Parent == nil {
			return fmt.Errorf("vault credential requires a provider parent")
		}
		provider := resources.IdentityVaultCredentialProvider(p.resources, change.Parent.Ref)
		if provider == nil {
			return fmt.Errorf("vault credential requires a declared provider")
		}
		root, ok := p.resources.GetResourceByRef(resources.NormalizeResourceRef(provider.Directory))
		if !ok {
			return fmt.Errorf("vault credential requires a directory")
		}
		directory, ok := root.(*resources.IdentityDirectoryResource)
		if !ok || directory.IsExternal() {
			return fmt.Errorf("vault credential requires a managed directory")
		}
		change.Parent.ID = provider.GetKonnectID()
		change.Fields[FieldProviderName] = provider.Name
		change.References = map[string]ReferenceInfo{
			FieldDirectoryID: {Ref: directory.Ref, ID: directory.GetKonnectID()},
		}
		protection, err := p.getInheritedProtectionState(ctx, make(map[string]*inheritedProtectionState), provider.Ref)
		if err != nil {
			return err
		}
		if protection == nil {
			return fmt.Errorf("vault credential requires a protecting directory")
		}
		if protection.protected && change.Action != ActionCreate {
			return fmt.Errorf("vault credential directory is protected")
		}
		change.ProtectingParent = &protection.info
		if change.Action != ActionDelete {
			if len(change.SecretWrites) != 1 || change.SecretWrites[0].Field != "/secret" {
				return fmt.Errorf(
					"vault credential %q requires a deferred secret source and explicit secret-write selection for updates",
					change.ResourceRef,
				)
			}
		}
		if change.Action != ActionCreate {
			values, err := p.client.ListIdentityVaultProviderCredentials(
				ctx,
				directory.GetKonnectID(),
				provider.GetKonnectID(),
			)
			if err != nil {
				return err
			}
			if _, err := identityStaticCredentialSlots(values); err != nil {
				return err
			}
			found := false
			for _, value := range values {
				if value.ID == change.ResourceID {
					change.Fields[FieldExpectedSelector] = value.Selector
					change.Fields[FieldExpectedPriority] = value.Priority
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("vault credential disappeared; regenerate the plan")
			}
		}
		for _, other := range plan.Changes {
			createsProvider := other.ResourceType == ResourceTypeIdentityVaultProvider && other.ResourceRef == provider.Ref
			createsDirectory := other.ResourceType == ResourceTypeIdentityDirectory && other.ResourceRef == directory.Ref
			if other.Action == ActionCreate && (createsProvider || createsDirectory) {
				change.DependsOn = appendDependsOn(change.DependsOn, other.ID)
			}
			if change.Action == ActionDelete && other.ResourceType == ResourceTypeIdentityVaultProviderCredential &&
				other.Action == ActionCreate &&
				other.Parent != nil &&
				other.Parent.Ref == provider.Ref {
				change.DependsOn = appendDependsOn(change.DependsOn, other.ID)
			}
		}
		if change.Action != ActionDelete {
			if err := p.bindIdentityVaultCredentialKeys(ctx, plan, change, directory); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Planner) bindIdentityVaultCredentialKeys(
	ctx context.Context,
	plan *Plan,
	change *PlannedChange,
	directory *resources.IdentityDirectoryResource,
) error {
	remaining := make(map[string]bool)
	if directory.GetKonnectID() != "" {
		keys, err := p.client.ListIdentityVaultKeys(ctx, directory.GetKonnectID())
		if err != nil {
			return err
		}
		for _, key := range keys {
			remaining[key.ID] = true
		}
	}
	for i := range plan.Changes {
		key := &plan.Changes[i]
		if key.ResourceType != ResourceTypeIdentityVaultKey || key.Parent == nil || key.Parent.Ref != directory.Ref {
			continue
		}
		if key.Action == ActionDelete {
			delete(remaining, key.ResourceID)
			key.DependsOn = appendDependsOn(key.DependsOn, change.ID)
		}
		if key.Action == ActionCreate {
			remaining[key.ID] = true
			change.DependsOn = appendDependsOn(change.DependsOn, key.ID)
		}
	}
	if len(remaining) == 0 {
		return fmt.Errorf(
			"vault credential %q requires at least one registered or planned encryption key",
			change.ResourceRef,
		)
	}
	return nil
}

func (p *Planner) resolveIdentityVaultProviderDeletes(ctx context.Context, plan *Plan) error {
	for i := range plan.Changes {
		parent := &plan.Changes[i]
		if parent.ResourceType != ResourceTypeIdentityVaultProvider || parent.Action != ActionDelete ||
			parent.Parent == nil {
			continue
		}
		retired := make(map[string]PlannedChange)
		for _, child := range plan.Changes {
			if child.ResourceType == ResourceTypeIdentityVaultProviderCredential && child.Action == ActionDelete &&
				child.Parent != nil &&
				child.Parent.ID == parent.ResourceID &&
				child.Namespace == parent.Namespace &&
				child.References[FieldDirectoryID].ID == parent.Parent.ID {
				retired[child.ResourceID] = child
			}
		}
		if len(retired) == 0 {
			if err := p.client.CheckIdentityVaultProviderCredentials(ctx, parent.Parent.ID, parent.ResourceID); err != nil {
				return err
			}
			continue
		}
		values, err := p.client.ListIdentityVaultProviderCredentials(ctx, parent.Parent.ID, parent.ResourceID)
		if err != nil {
			return err
		}
		if _, err := identityStaticCredentialSlots(values); err != nil {
			return err
		}
		for _, value := range values {
			child, ok := retired[value.ID]
			if !ok || child.Fields[FieldProviderName] != parent.Fields[FieldName] {
				return fmt.Errorf("cannot delete provider: credentials lack explicit authorized retirement")
			}
			actual := identityStaticCredentialFields(value.Selector, value.Priority)
			for _, field := range []string{FieldSelector, FieldPriority} {
				if !reflect.DeepEqual(actual[field], child.Fields[field]) {
					return fmt.Errorf("credential routing changed before provider retirement")
				}
			}
			parent.DependsOn = appendDependsOn(parent.DependsOn, child.ID)
		}
	}
	return nil
}
