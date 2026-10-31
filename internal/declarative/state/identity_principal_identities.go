package state

import (
	"context"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	declerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/declarative/resources"

	"github.com/kong/kongctl/internal/konnect/helpers"
)

func (c *Client) ListIdentityPrincipalIdentities(ctx context.Context, directoryID, principalID string,
) ([]helpers.PrincipalIdentity, error) {
	if err := ValidateAPIClient(c.identityPrincipalIdentitiesAPI, "Kong Identity principal identities API"); err != nil {
		return nil, err
	}
	return helpers.ListPrincipalIdentities(ctx, c.identityPrincipalIdentitiesAPI, directoryID, principalID, 100)
}

type IdentityPrincipalIdentity struct {
	helpers.PrincipalIdentity
	NormalizedLabels map[string]string
}

func (i IdentityPrincipalIdentity) GetID() string { return i.ID }

func (i IdentityPrincipalIdentity) GetName() string {
	return i.ManagedBy[resources.ManagedByRefKey]
}

func (i IdentityPrincipalIdentity) GetLabels() map[string]string           { return i.Labels }
func (i IdentityPrincipalIdentity) GetManagedBy() map[string]string        { return i.ManagedBy }
func (i IdentityPrincipalIdentity) GetNormalizedLabels() map[string]string { return i.NormalizedLabels }

func NormalizeIdentityPrincipalIdentity(i helpers.PrincipalIdentity) IdentityPrincipalIdentity {
	var labels map[string]string
	if i.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool {
		labels = resources.NormalizedChildOwnership(i.ManagedBy)
	}
	return IdentityPrincipalIdentity{PrincipalIdentity: i, NormalizedLabels: labels}
}

func (c *Client) GetIdentityPrincipalIdentity(ctx context.Context, directoryID, principalID, id string,
) (*IdentityPrincipalIdentity, error) {
	if err := ValidateAPIClient(c.identityPrincipalIdentitiesAPI, "Kong Identity principal identities API"); err != nil {
		return nil, err
	}
	v, err := helpers.GetPrincipalIdentity(ctx, c.identityPrincipalIdentitiesAPI, directoryID, principalID, id, 100)
	if err != nil {
		if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	value := NormalizeIdentityPrincipalIdentity(*v)
	return &value, nil
}

func (c *Client) CreateIdentityPrincipalIdentity(ctx context.Context, directoryID, principalID string,
	body kkComps.KongPrincipalIdentityBody,
) (*IdentityPrincipalIdentity, error) {
	if err := ValidateAPIClient(c.identityPrincipalIdentitiesAPI, "Kong Identity principal identities API"); err != nil {
		return nil, err
	}
	if err := stampManagedByRequest(ctx, resources.ResourceTypeIdentityPrincipalIdentity, &body); err != nil {
		return nil, err
	}
	v, err := helpers.CreatePrincipalIdentity(ctx, c.identityPrincipalIdentitiesAPI, directoryID, principalID, body)
	if err != nil {
		return nil, err
	}
	value := NormalizeIdentityPrincipalIdentity(*v)
	return &value, nil
}

func (c *Client) ReplaceIdentityPrincipalIdentity(ctx context.Context, directoryID, principalID, id string,
	body kkComps.KongPrincipalIdentityBody,
) error {
	if err := ValidateAPIClient(c.identityPrincipalIdentitiesAPI, "Kong Identity principal identities API"); err != nil {
		return err
	}
	if err := stampManagedByRequest(ctx, resources.ResourceTypeIdentityPrincipalIdentity, &body); err != nil {
		return err
	}
	_, err := helpers.ReplacePrincipalIdentity(
		ctx,
		c.identityPrincipalIdentitiesAPI,
		directoryID,
		principalID,
		id,
		body,
	)
	return err
}

func (c *Client) DeleteIdentityPrincipalIdentity(ctx context.Context, directoryID, principalID, id string) error {
	if err := ValidateAPIClient(c.identityPrincipalIdentitiesAPI, "Kong Identity principal identities API"); err != nil {
		return err
	}
	err := helpers.DeletePrincipalIdentity(ctx, c.identityPrincipalIdentitiesAPI, directoryID, principalID, id)
	if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
		return nil
	}
	return err
}
