package state

import (
	"context"

	"github.com/kong/kongctl/internal/konnect/helpers"
)

func (c *Client) CheckIdentityDirectoryVaultRetirements(ctx context.Context, directory IdentityDirectory,
	removals helpers.IdentityDirectoryVaultRetirements,
) error {
	return helpers.RequireIdentityDirectoryVaultResourceRetirements(ctx, c.identityDirectoryDependenciesAPI,
		directory.ID, directory.VaultEnabled, removals)
}
