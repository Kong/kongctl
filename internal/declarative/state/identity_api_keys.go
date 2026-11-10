package state

import (
	"context"
	"fmt"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	declerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/secretoutput"
)

type IdentityAPIKey struct{ helpers.IdentityAPIKey }

func (k IdentityAPIKey) GetID() string                          { return k.ID }
func (k IdentityAPIKey) GetName() string                        { return k.ManagedBy[resources.ManagedByRefKey] }
func (k IdentityAPIKey) GetLabels() map[string]string           { return nil }
func (k IdentityAPIKey) GetNormalizedLabels() map[string]string { return nil }
func (k IdentityAPIKey) GetManagedBy() map[string]string        { return k.ManagedBy }

func (c *Client) IdentityAPIKeyClient(
	kind resources.ResourceType,
	directory, parent string,
) (*helpers.IdentityAPIKeyClient, error) {
	switch kind { //nolint:exhaustive // Only principal/group API keys are valid here.
	case resources.ResourceTypeIdentityPrincipalAPIKey:
		return helpers.NewPrincipalAPIKeyClient(c.identityPrincipalKeysAPI, directory, parent)
	case resources.ResourceTypeIdentityGroupAPIKey:
		return helpers.NewGroupAPIKeyClient(c.identityGroupKeysAPI, directory, parent)
	default:
		return nil, fmt.Errorf("unsupported Identity API-key resource")
	}
}

func (c *Client) ListIdentityAPIKeys(
	ctx context.Context,
	kind resources.ResourceType,
	directory, parent string,
) ([]IdentityAPIKey, error) {
	var values []helpers.IdentityAPIKey
	var err error
	switch kind { //nolint:exhaustive // Only principal/group API keys are valid here.
	case resources.ResourceTypeIdentityPrincipalAPIKey:
		var api helpers.IdentityPrincipalKeysReader = c.identityPrincipalKeysAPI
		if api == nil {
			api = c.identityPrincipalKeysReader
		}
		values, err = helpers.ListPrincipalAPIKeys(ctx, api, directory, parent, 100)
	case resources.ResourceTypeIdentityGroupAPIKey:
		var api helpers.IdentityGroupKeysLister = c.identityGroupKeysAPI
		if api == nil {
			api = c.identityGroupsAPI
		}
		values, err = helpers.ListIdentityGroupAPIKeys(ctx, api, directory, parent, 100)
	default:
		return nil, fmt.Errorf("unsupported Identity API-key resource")
	}
	if err != nil {
		return nil, err
	}
	result := make([]IdentityAPIKey, 0, len(values))
	for _, value := range values {
		result = append(result, IdentityAPIKey{value})
	}
	return result, nil
}

func (c *Client) GetIdentityAPIKey(
	ctx context.Context,
	kind resources.ResourceType,
	directory, parent, id string,
) (*IdentityAPIKey, error) {
	values, err := c.ListIdentityAPIKeys(ctx, kind, directory, parent)
	if err != nil {
		if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
			if kind == resources.ResourceTypeIdentityPrincipalAPIKey {
				value, lookupErr := c.GetIdentityPrincipal(ctx, directory, parent)
				if lookupErr != nil {
					return nil, lookupErr
				}
				if value == nil {
					return nil, nil
				}
			} else {
				value, lookupErr := c.GetIdentityGroup(ctx, directory, parent)
				if lookupErr != nil {
					return nil, lookupErr
				}
				if value == nil {
					return nil, nil
				}
			}
		}
		return nil, err
	}
	for _, value := range values {
		if value.ID != id {
			continue
		}
		client, err := c.IdentityAPIKeyClient(kind, directory, parent)
		if err != nil {
			return nil, err
		}
		observed, err := client.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return &IdentityAPIKey{*observed}, nil
	}
	return nil, nil
}

func (c *Client) CreateIdentityAPIKey(ctx context.Context, kind resources.ResourceType, directory, parent string,
	request kkComps.CreatePrincipalKeyBodyv1, capture *secretoutput.File, local secretoutput.Metadata,
) (*IdentityAPIKey, error) {
	if err := stampManagedByRequest(ctx, kind, &request); err != nil {
		return nil, err
	}
	client, err := c.IdentityAPIKeyClient(kind, directory, parent)
	if err != nil {
		return nil, err
	}
	value, err := client.CreateWithReservedCapture(ctx, helpers.IdentityAPIKeyCreate{
		Name: request.Name, ExpiresAt: request.ExpiresAt, ManagedBy: request.ManagedBy,
	}, capture, local)
	if err != nil {
		return nil, err
	}
	return &IdentityAPIKey{*value}, nil
}

func (c *Client) UpdateIdentityAPIKey(ctx context.Context, kind resources.ResourceType, directory, parent, id string,
	request kkComps.UpdatePrincipalKeyBody,
) error {
	if request.ManagedBy != nil {
		return fmt.Errorf("declarative API-key creation identity markers cannot be changed")
	}
	client, err := c.IdentityAPIKeyClient(kind, directory, parent)
	if err != nil {
		return err
	}
	name, nameSet := request.Name.Get()
	_, err = client.Update(ctx, id, helpers.IdentityAPIKeyUpdate{
		Name: name, NameSet: nameSet, ExpiresAt: request.ExpiresAt,
	})
	return err
}

func (c *Client) DeleteIdentityAPIKey(
	ctx context.Context,
	kind resources.ResourceType,
	directory, parent, id string,
) error {
	client, err := c.IdentityAPIKeyClient(kind, directory, parent)
	if err != nil {
		return err
	}
	return client.Delete(ctx, id)
}
