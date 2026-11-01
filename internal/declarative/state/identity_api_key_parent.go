package state

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

// VerifyIdentityAPIKeyParent observes current authority independently of a key's
// immutable markers. The identity is a principal ref or an ordinary group name.
func (c *Client) VerifyIdentityAPIKeyParent(ctx context.Context, kind resources.ResourceType,
	directory, parent, namespace, identity string,
) (bool, error) {
	var metadata map[string]string
	switch kind { //nolint:exhaustive // Only the two key kinds have this parent contract.
	case resources.ResourceTypeIdentityPrincipalAPIKey:
		if err := c.VerifyIdentityMembershipPrincipal(ctx, directory, parent); err != nil {
			return false, err
		}
		current, err := c.GetIdentityPrincipal(ctx, directory, parent)
		if err != nil {
			return false, err
		}
		if current == nil {
			return false, fmt.Errorf("API-key parent principal no longer exists")
		}
		if err := helpers.ValidateIdentityPrincipalManagement(current.IdentityPrincipal); err != nil {
			return false, err
		}
		if current.GetName() != identity {
			return false, fmt.Errorf("API-key parent principal ref changed; regenerate the plan")
		}
		metadata = current.ManagedBy
	case resources.ResourceTypeIdentityGroupAPIKey:
		current, err := c.GetIdentityGroup(ctx, directory, parent)
		if err != nil {
			return false, err
		}
		if current == nil {
			return false, fmt.Errorf("API-key parent group no longer exists")
		}
		if err := helpers.ValidateIdentityGroupManagement(current.IdentityGroup); err != nil {
			return false, err
		}
		if current.Name != identity {
			return false, fmt.Errorf("API-key parent group name changed; regenerate the plan")
		}
		metadata = current.ManagedBy
	default:
		return false, fmt.Errorf("unsupported Identity API-key parent")
	}
	if metadata[resources.ManagedByToolKey] != resources.ManagedByTool {
		return false, fmt.Errorf("API-key parent is not managed by kongctl; explicitly adopt it before managing keys")
	}
	if err := resources.ValidateChildOwnership("API-key parent", metadata, namespace); err != nil {
		return false, err
	}
	return resources.IsChildProtected(metadata), nil
}
