package state

import (
	"context"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	declerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type IdentityVaultKey struct{ helpers.IdentityVaultKey }

func (k IdentityVaultKey) GetID() string                          { return k.ID }
func (k IdentityVaultKey) GetName() string                        { return k.Name }
func (k IdentityVaultKey) GetLabels() map[string]string           { return nil }
func (k IdentityVaultKey) GetNormalizedLabels() map[string]string { return nil }

func (c *Client) ListIdentityVaultKeys(ctx context.Context, directory string) ([]IdentityVaultKey, error) {
	values, err := helpers.ListIdentityVaultKeys(ctx, c.identityVaultKeysAPI, directory, 100)
	if err != nil {
		return nil, err
	}
	result := make([]IdentityVaultKey, 0, len(values))
	for _, value := range values {
		result = append(result, IdentityVaultKey{value})
	}
	return result, nil
}

func (c *Client) GetIdentityVaultKey(ctx context.Context, directory, id string) (*IdentityVaultKey, error) {
	value, err := helpers.GetIdentityVaultKey(ctx, c.identityVaultKeysAPI, directory, id)
	if err != nil {
		if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &IdentityVaultKey{*value}, nil
}

func (c *Client) CreateIdentityVaultKey(ctx context.Context, directory string,
	request kkComps.CreateVaultKeyBody,
) (*IdentityVaultKey, error) {
	value, err := helpers.CreateIdentityVaultKey(ctx, c.identityVaultKeysAPI, directory, request)
	if err != nil {
		return nil, err
	}
	return &IdentityVaultKey{*value}, nil
}

func (c *Client) UpdateIdentityVaultKey(ctx context.Context, directory, id, kid, name string) error {
	return helpers.UpdateIdentityVaultKey(ctx, c.identityVaultKeysAPI, directory, id, kid, name)
}

func (c *Client) DeleteIdentityVaultKey(ctx context.Context, directory, id string) error {
	err := helpers.DeleteIdentityVaultKey(ctx, c.identityVaultKeysAPI, directory, id)
	if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
		return nil
	}
	return err
}

func (c *Client) CheckIdentityDirectoryVaultRetirements(ctx context.Context, directory IdentityDirectory,
	keysToRemove map[string]string,
) error {
	return helpers.RequireIdentityDirectoryVaultRetirements(ctx, c.identityDirectoryDependenciesAPI,
		directory.ID, directory.VaultEnabled, keysToRemove)
}
