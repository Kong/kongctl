package dump

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
)

func collectIdentityVaultProviders(ctx context.Context, d *declarativeDumpContext,
	directory *resources.IdentityDirectoryResource,
) error {
	values, err := d.stateClient.ListIdentityVaultProviders(ctx, directory.Ref)
	if err != nil {
		return err
	}
	for _, current := range values {
		provider := resources.IdentityVaultProviderResource{
			Ref: "vault-provider-" + current.ID, Name: current.Name, TemplateName: current.TemplateName,
			ClientID: current.ClientID, CredentialType: &current.CredentialType,
			DCREnabled: current.DCREnabled, PKCEEnabled: current.PKCEEnabled, Enabled: current.Enabled,
			RefreshBufferSeconds: current.RefreshBufferSeconds,
			TokenEndpoint:        current.TokenEndpointEffective, AuthorizationEndpoint: current.AuthorizationEndpointEffective,
			RegistrationEndpoint: current.RegistrationEndpointEffective,
		}
		if len(current.Scopes) > 0 {
			provider.Scopes = slices.Clone(current.Scopes)
		} else {
			// Omission is reproducible only when the current template also has
			// no default scopes; explicit [] cannot be reconciled through PATCH.
			template, err := d.stateClient.GetIdentityVaultProviderTemplate(ctx, current.TemplateName)
			if err != nil {
				return err
			}
			if len(template.DefaultScopes) > 0 {
				return fmt.Errorf(
					"vault provider %q has empty scopes differing from template defaults; cannot export a reproducible declaration",
					current.Name,
				)
			}
		}
		if err := provider.Validate(); err != nil {
			return fmt.Errorf("cannot export Vault provider %q: %w", current.Name, err)
		}
		directory.VaultProviders = append(directory.VaultProviders, provider)
	}
	slices.SortFunc(directory.VaultProviders, func(a, b resources.IdentityVaultProviderResource) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return nil
}
