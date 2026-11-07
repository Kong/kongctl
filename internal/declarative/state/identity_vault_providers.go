package state

import (
	"context"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	declerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type IdentityVaultProvider struct{ helpers.IdentityVaultProvider }

func (p IdentityVaultProvider) GetID() string                          { return p.ID }
func (p IdentityVaultProvider) GetName() string                        { return p.Name }
func (p IdentityVaultProvider) GetLabels() map[string]string           { return nil }
func (p IdentityVaultProvider) GetNormalizedLabels() map[string]string { return nil }

func (c *Client) ListIdentityVaultProviders(ctx context.Context, directory string) ([]IdentityVaultProvider, error) {
	values, err := helpers.ListIdentityVaultProviders(ctx, c.identityVaultProvidersAPI, directory, 100)
	if err != nil {
		return nil, err
	}
	result := make([]IdentityVaultProvider, 0, len(values))
	for _, value := range values {
		result = append(result, IdentityVaultProvider{value})
	}
	return result, nil
}

func (c *Client) GetIdentityVaultProvider(ctx context.Context, directory, id string) (*IdentityVaultProvider, error) {
	value, err := helpers.GetIdentityVaultProvider(ctx, c.identityVaultProvidersAPI, directory, id)
	if err != nil {
		if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &IdentityVaultProvider{*value}, nil
}

func (c *Client) GetIdentityVaultProviderTemplate(
	ctx context.Context,
	name string,
) (*helpers.IdentityCredentialProviderTemplate, error) {
	return helpers.GetIdentityCredentialProviderTemplate(ctx, c.identityVaultProvidersAPI, name)
}

func (c *Client) CreateIdentityVaultProvider(
	ctx context.Context,
	directory string,
	request kkComps.CreateVaultProviderBody,
) (*IdentityVaultProvider, error) {
	value, err := helpers.CreateIdentityVaultProvider(ctx, c.identityVaultProvidersAPI, directory, request)
	if err != nil {
		return nil, err
	}
	return &IdentityVaultProvider{*value}, nil
}

func (c *Client) UpdateIdentityVaultProvider(
	ctx context.Context,
	directory, id, name string,
	request kkComps.PatchVaultProviderBody,
) error {
	_, err := helpers.UpdateIdentityVaultProvider(ctx, c.identityVaultProvidersAPI, directory, id, name, request)
	return err
}

func (c *Client) CheckIdentityVaultProviderCredentials(ctx context.Context, directory, id string) error {
	return helpers.RequireEmptyIdentityVaultProviderCredentials(ctx, c.identityVaultProvidersAPI, directory, id)
}

func (c *Client) DeleteIdentityVaultProvider(ctx context.Context, directory, id, name string) error {
	err := helpers.DeleteIdentityVaultProvider(ctx, c.identityVaultProvidersAPI, directory, id, name)
	if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
		return nil
	}
	return err
}
