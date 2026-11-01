package state

import (
	"context"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type IdentityVaultProviderCredential struct {
	helpers.IdentityVaultProviderCredential
}

func (c IdentityVaultProviderCredential) GetID() string { return c.ID }
func (c IdentityVaultProviderCredential) GetName() string {
	return resources.IdentityVaultCredentialSlot(c.Priority)
}
func (c IdentityVaultProviderCredential) GetLabels() map[string]string           { return nil }
func (c IdentityVaultProviderCredential) GetNormalizedLabels() map[string]string { return nil }

func (c *Client) ListIdentityVaultProviderCredentials(
	ctx context.Context,
	directory, provider string,
) ([]IdentityVaultProviderCredential, error) {
	values, err := helpers.ListIdentityVaultProviderCredentials(
		ctx,
		c.identityStaticVaultCredentialsAPI,
		directory,
		provider,
		100,
	)
	if err != nil {
		return nil, err
	}
	result := make([]IdentityVaultProviderCredential, 0, len(values))
	for _, value := range values {
		result = append(result, IdentityVaultProviderCredential{value})
	}
	return result, nil
}

func (c *Client) CreateIdentityStaticVaultCredential(
	ctx context.Context,
	scope helpers.IdentityStaticVaultCredentialScope,
	routing helpers.IdentityVaultCredentialRouting,
	secret string,
) (*IdentityVaultProviderCredential, error) {
	value, err := helpers.CreateIdentityStaticVaultCredential(
		ctx,
		c.identityStaticVaultCredentialsAPI,
		scope,
		routing,
		secret,
	)
	if err != nil {
		return nil, err
	}
	return &IdentityVaultProviderCredential{*value}, nil
}

func (c *Client) ReplaceIdentityStaticVaultCredential(
	ctx context.Context,
	scope helpers.IdentityStaticVaultCredentialScope,
	id string,
	expected, routing helpers.IdentityVaultCredentialRouting,
	secret string,
) (*IdentityVaultProviderCredential, error) {
	value, err := helpers.ReplaceIdentityStaticVaultCredential(
		ctx,
		c.identityStaticVaultCredentialsAPI,
		scope,
		id,
		expected,
		&routing,
		secret,
	)
	if err != nil {
		return nil, err
	}
	return &IdentityVaultProviderCredential{*value}, nil
}

func (c *Client) DeleteIdentityStaticVaultCredential(
	ctx context.Context,
	scope helpers.IdentityStaticVaultCredentialScope,
	id string,
	expected helpers.IdentityVaultCredentialRouting,
) error {
	return helpers.DeleteIdentityStaticVaultCredential(ctx, c.identityStaticVaultCredentialsAPI, scope, id, expected)
}
