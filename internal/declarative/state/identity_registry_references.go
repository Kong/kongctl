package state

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
)

func (c *Client) ListRegistryAgentPrincipalReferences(
	ctx context.Context,
) ([]helpers.RegistryAgentPrincipalReference, error) {
	return helpers.ListRegistryAgentPrincipalReferences(ctx, c.registryAgentReferencesReader, 100)
}

// VerifyIdentityRegistryReferencesEmpty always observes fresh state. A Registry
// association has no implicit retirement authority from its Identity target.
func (c *Client) VerifyIdentityRegistryReferencesEmpty(ctx context.Context, directoryID, principalID string) error {
	links, err := c.ListRegistryAgentPrincipalReferences(ctx)
	if err != nil {
		return err
	}
	return RequireEmptyIdentityRegistryReferences(links, directoryID, principalID)
}

func RequireEmptyIdentityRegistryReferences(links []helpers.RegistryAgentPrincipalReference,
	directoryID, principalID string,
) error {
	if !util.IsValidUUID(directoryID) || principalID != "" && !util.IsValidUUID(principalID) {
		return fmt.Errorf("registry reference checks require a directory UUID and optional principal UUID")
	}
	for _, link := range links {
		if link.DirectoryID == directoryID && (principalID == "" || link.PrincipalID == principalID) {
			return fmt.Errorf("identity resource is referenced by Registry Agent %s (association %s); "+
				"remove the association explicitly before deleting its target", link.AgentID, link.ID)
		}
	}
	return nil
}
