package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/identitykeys"
)

func (p *identityDirectoryPlanner) planVaultKeys(ctx context.Context, config *Config, plan *Plan,
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
		var desired []resources.IdentityVaultKeyResource
		for _, key := range rs.Identity.VaultKeys {
			if resources.NormalizeResourceRef(key.Directory) == directory.Ref {
				desired = append(desired, key)
			}
		}
		scoped := rs.SyncScope != nil && rs.SyncScope.ChildInScope(
			resources.ResourceTypeIdentityDirectory, directory.Ref, resources.ResourceTypeIdentityVaultKey,
		)
		if len(desired) == 0 && (plan.Metadata.Mode != PlanModeSync || !scoped) {
			continue
		}
		var observed []state.IdentityVaultKey
		current, exists := byName[directory.Name]
		if exists {
			if current.VaultEnabled == nil || !*current.VaultEnabled {
				return fmt.Errorf(
					"directory %q Vault keys cannot be observed; explicitly enable Vault before planning its keys",
					directory.Name,
				)
			}
			var err error
			observed, err = p.planner.client.ListIdentityVaultKeys(ctx, current.ID)
			if err != nil {
				return err
			}
		} else if directory.VaultEnabled == nil || !*directory.VaultEnabled {
			return fmt.Errorf("new directory %q requires explicit vault_enabled: true to create Vault keys", directory.Name)
		}
		if directory.VaultEnabled != nil && !*directory.VaultEnabled && plan.Metadata.Mode != PlanModeDelete {
			return fmt.Errorf("cannot manage Vault keys while disabling directory %q Vault", directory.Name)
		}
		start := len(plan.Changes)
		byKid := make(map[string]state.IdentityVaultKey, len(observed))
		for _, key := range observed {
			byKid[key.Kid] = key
		}
		retained := make(map[string]bool)
		protected := exists && resources.IsChildProtected(current.ManagedBy)
		add := func(action ActionType, ref, id, name, publicKey string, changed map[string]FieldChange) error {
			if protected && action != ActionCreate {
				return fmt.Errorf("directory %q is protected", directory.Name)
			}
			plan.AddChange(PlannedChange{
				ID: p.NextChangeID(
					action,
					ResourceTypeIdentityVaultKey,
					ref,
				), ResourceType: ResourceTypeIdentityVaultKey,
				ResourceRef: ref, ResourceID: id, Action: action, Namespace: config.Namespace, Protection: false,
				Fields: map[string]any{FieldName: name, FieldPublicKey: publicKey}, ChangedFields: changed,
				Parent: &ParentInfo{Ref: directory.Ref, ID: current.ID},
				ProtectingParent: &ProtectingParentInfo{
					ResourceType: ResourceTypeIdentityDirectory,
					ResourceRef:  directory.Ref, ResourceID: current.ID, ResourceName: directory.Name,
				},
			})
			return nil
		}
		creates := 0
		for _, key := range desired {
			material, err := identitykeys.ParsePublicKey(key.PublicKey)
			if err != nil {
				return err
			}
			retained[material.Kid] = true
			value, found := byKid[material.Kid]
			if found {
				if target, ok := rs.GetResourceByRef(key.Ref); ok {
					target.(*resources.IdentityVaultKeyResource).SetKonnectID(value.ID)
				}
			}
			if plan.Metadata.Mode == PlanModeDelete {
				if found {
					if err := add(ActionDelete, key.Ref, value.ID, value.Name, material.PEM, nil); err != nil {
						return err
					}
				}
			} else if !found {
				creates++
				if err := add(ActionCreate, key.Ref, "", key.Name, material.PEM, nil); err != nil {
					return err
				}
			} else if value.Name != key.Name {
				if err := add(ActionUpdate, key.Ref, value.ID, key.Name, material.PEM,
					map[string]FieldChange{FieldName: {Old: value.Name, New: key.Name}}); err != nil {
					return err
				}
			}
		}
		if creates+len(observed) > 5 {
			return fmt.Errorf(
				"directory %q allows five Vault keys; explicitly retire a selected key before adding new material at capacity",
				directory.Name,
			)
		}
		if plan.Metadata.Mode == PlanModeSync && scoped {
			for _, value := range observed {
				if !retained[value.Kid] {
					if err := add(ActionDelete, "vault-key-"+value.ID, value.ID, value.Name, value.PublicKey, nil); err != nil {
						return err
					}
				}
			}
		}
		for i := start; i < len(plan.Changes); i++ {
			change := &plan.Changes[i]
			for j := range plan.Changes {
				other := &plan.Changes[j]
				if other.ResourceType == ResourceTypeIdentityDirectory {
					if other.Action == ActionCreate && other.ResourceRef == directory.Ref {
						change.DependsOn = appendDependsOn(change.DependsOn, other.ID)
					}
					if other.Action == ActionDelete && other.ResourceID == current.ID && change.Action == ActionDelete {
						other.DependsOn = appendDependsOn(other.DependsOn, change.ID)
					}
				}
				// Register new material before sync retires old keys. At capacity,
				// the earlier check requires an explicit operator-selected retirement.
				if change.Action == ActionDelete && other.ResourceType == ResourceTypeIdentityVaultKey &&
					other.Action == ActionCreate && other.Parent != nil && other.Parent.Ref == directory.Ref {
					change.DependsOn = appendDependsOn(change.DependsOn, other.ID)
				}
			}
		}
	}
	return nil
}
