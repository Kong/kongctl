package state

import (
	"context"
	"fmt"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	declerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type IdentityTrustedIDP struct{ helpers.IdentityTrustedIDP }

func (p IdentityTrustedIDP) GetID() string                          { return p.ID }
func (p IdentityTrustedIDP) GetName() string                        { return p.IssuerURL }
func (p IdentityTrustedIDP) GetLabels() map[string]string           { return nil }
func (p IdentityTrustedIDP) GetNormalizedLabels() map[string]string { return nil }

func (c *Client) ListIdentityTrustedIDPs(ctx context.Context, directory string) ([]IdentityTrustedIDP, error) {
	values, err := helpers.ListIdentityTrustedIDPs(ctx, c.identityTrustedIDPsAPI, directory, 100)
	if err != nil {
		return nil, err
	}
	result := make([]IdentityTrustedIDP, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		if seen[value.IssuerURL] {
			return nil, fmt.Errorf("trusted IdP collection contains a duplicate issuer")
		}
		seen[value.IssuerURL] = true
		result = append(result, IdentityTrustedIDP{value})
	}
	return result, nil
}

func (c *Client) GetIdentityTrustedIDP(ctx context.Context, directory, id string) (*IdentityTrustedIDP, error) {
	value, err := helpers.GetIdentityTrustedIDP(ctx, c.identityTrustedIDPsAPI, directory, id)
	if err != nil {
		if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &IdentityTrustedIDP{*value}, nil
}

func (c *Client) ListIdentityTrustedIDPPrincipals(
	ctx context.Context,
	directory, id string,
) ([]helpers.IdentityPrincipal, error) {
	return helpers.ListIdentityTrustedIDPPrincipals(ctx, c.identityTrustedIDPsAPI, directory, id, 100)
}

func (c *Client) CreateIdentityTrustedIDP(
	ctx context.Context,
	directory string,
	request kkComps.CreateTrustedIdpPayload,
) (*IdentityTrustedIDP, error) {
	value, err := helpers.CreateIdentityTrustedIDP(ctx, c.identityTrustedIDPsAPI, directory, request)
	if err != nil {
		return nil, err
	}
	return &IdentityTrustedIDP{*value}, nil
}

func (c *Client) ReplaceIdentityTrustedIDP(
	ctx context.Context,
	directory, id, issuer string,
	request kkComps.ReplaceTrustedIdpBody,
) error {
	_, err := helpers.ReplaceIdentityTrustedIDP(ctx, c.identityTrustedIDPsAPI, directory, id, issuer, request)
	return err
}

func (c *Client) DeleteIdentityTrustedIDP(ctx context.Context, directory, id, issuer string) error {
	err := helpers.DeleteIdentityTrustedIDP(ctx, c.identityTrustedIDPsAPI, directory, id, issuer)
	if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
		return nil
	}
	return err
}
