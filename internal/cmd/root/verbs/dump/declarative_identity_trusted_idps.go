package dump

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/identitytrust"
)

func collectIdentityTrustedIDPs(ctx context.Context, d *declarativeDumpContext,
	directory *resources.IdentityDirectoryResource,
) error {
	values, err := d.stateClient.ListIdentityTrustedIDPs(ctx, directory.Ref)
	if err != nil {
		return err
	}
	for _, current := range values {
		value := resources.IdentityTrustedIDPResource{
			Ref: "trusted-idp-" + current.ID, IssuerURL: current.IssuerURL,
			DisplayName: current.DisplayName, JwksURI: current.JwksURI,
			JitProvisioningEnabled: current.JitProvisioningEnabled, VaultAccessEnabled: current.VaultAccessEnabled,
			Jit: identitytrust.NormalizeJIT(current.Jit),
		}
		if current.DisplayName == nil {
			value.DeclareNullDisplayName()
		}
		if current.JwksURI == nil {
			value.DeclareDiscoveryJWKS()
		}
		if value.Jit == nil {
			value.Jit = &kkComps.TrustedIdpJit{}
		}
		if err := value.Validate(); err != nil {
			return fmt.Errorf("cannot export Trusted IdP: %w", err)
		}
		directory.TrustedIDPs = append(directory.TrustedIDPs, value)
	}
	slices.SortFunc(directory.TrustedIDPs, func(a, b resources.IdentityTrustedIDPResource) int {
		return cmp.Compare(a.IssuerURL, b.IssuerURL)
	})
	return nil
}
