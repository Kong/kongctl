package state

import (
	"context"

	"github.com/kong/kongctl/internal/konnect/helpers"
)

func (c *Client) ListIdentityPrincipalIdentities(ctx context.Context, directoryID, principalID string,
) ([]helpers.PrincipalIdentity, error) {
	if err := ValidateAPIClient(c.identityPrincipalIdentitiesAPI, "Kong Identity principal identities API"); err != nil {
		return nil, err
	}
	return helpers.ListPrincipalIdentities(ctx, c.identityPrincipalIdentitiesAPI, directoryID, principalID, 100)
}
