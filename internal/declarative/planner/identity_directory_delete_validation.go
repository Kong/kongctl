package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/identitykeys"
)

func (p *Planner) validateIdentityDirectoryDeletes(ctx context.Context, plan *Plan) error {
	for _, change := range plan.Changes {
		if change.ResourceType != ResourceTypeIdentityDirectory || change.Action != ActionDelete {
			continue
		}
		current, err := p.client.GetIdentityDirectoryByID(ctx, change.ResourceID)
		if err != nil {
			return err
		}
		if current == nil {
			continue
		}
		if current.ID != change.ResourceID || current.Name != change.Fields[FieldName] {
			return fmt.Errorf("directory identity changed; regenerate the plan")
		}
		removals := make(map[string]string)
		providers := make(map[string]string)
		idps := make(map[string]string)
		for _, child := range plan.Changes {
			if child.ResourceType == ResourceTypeIdentityTrustedIDP && child.Action == ActionDelete && child.Parent != nil &&
				child.Parent.ID == current.ID &&
				child.Namespace == change.Namespace {
				issuer, ok := child.Fields[FieldIssuerURL].(string)
				if !ok || issuer == "" {
					return fmt.Errorf("planned Trusted IdP deletion requires an issuer")
				}
				idps[child.ResourceID] = issuer
			}
			if child.ResourceType == ResourceTypeIdentityVaultProvider && child.Action == ActionDelete &&
				child.Parent != nil &&
				child.Parent.ID == current.ID &&
				child.Namespace == change.Namespace {
				name, ok := child.Fields[FieldName].(string)
				if !ok || name == "" {
					return fmt.Errorf("planned Vault provider deletion requires a name")
				}
				providers[child.ResourceID] = name
			}
			if child.ResourceType != ResourceTypeIdentityVaultKey || child.Action != ActionDelete ||
				child.Parent == nil || child.Parent.ID != current.ID || child.Namespace != change.Namespace {
				continue
			}
			material, _ := child.Fields[FieldPublicKey].(string)
			key, err := identitykeys.ParsePublicKey(material)
			if err != nil {
				return err
			}
			removals[child.ResourceID] = key.Kid
		}
		if err := p.client.CheckIdentityDirectoryVaultRetirements(ctx, *current, helpers.IdentityDirectoryVaultRetirements{
			Keys: removals, Providers: providers, TrustedIDPs: idps,
		}); err != nil {
			return fmt.Errorf("cannot delete directory %q: %w", current.Name, err)
		}
	}
	return nil
}
