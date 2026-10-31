package state

import (
	"context"
	"fmt"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/retry"
	declerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
)

type IdentityAuthServerScope struct{ kkComps.AuthServerScope }

func (s IdentityAuthServerScope) GetID() string                          { return s.ID }
func (s IdentityAuthServerScope) GetName() string                        { return s.Name }
func (s IdentityAuthServerScope) GetLabels() map[string]string           { return nil }
func (s IdentityAuthServerScope) GetNormalizedLabels() map[string]string { return nil }

type IdentityAuthServerClaim struct{ kkComps.Claim }

func (c IdentityAuthServerClaim) GetID() string                          { return c.ID }
func (c IdentityAuthServerClaim) GetName() string                        { return c.Name }
func (c IdentityAuthServerClaim) GetLabels() map[string]string           { return nil }
func (c IdentityAuthServerClaim) GetNormalizedLabels() map[string]string { return nil }

func (c *Client) ListIdentityAuthServerScopes(ctx context.Context, parent string) ([]IdentityAuthServerScope, error) {
	values, err := helpers.ListIdentityAuthServerScopes(ctx, c.identityAuthServerScopesAPI, parent, 100)
	if err != nil {
		return nil, err
	}
	result := make([]IdentityAuthServerScope, 0, len(values))
	for _, value := range values {
		result = append(result, IdentityAuthServerScope{value})
	}
	return result, nil
}

func (c *Client) ListIdentityAuthServerClaims(ctx context.Context, parent string) ([]IdentityAuthServerClaim, error) {
	values, err := helpers.ListIdentityAuthServerClaims(ctx, c.identityAuthServerClaimsAPI, parent, 100)
	if err != nil {
		return nil, err
	}
	result := make([]IdentityAuthServerClaim, 0, len(values))
	for _, value := range values {
		result = append(result, IdentityAuthServerClaim{value})
	}
	return result, nil
}

func (c *Client) ListIdentityAuthServerClients(ctx context.Context, parent string) ([]kkComps.AuthServerClient, error) {
	return helpers.ListIdentityAuthServerClients(ctx, c.identityAuthServerClientsAPI, parent, 100)
}

func (c *Client) GetIdentityAuthServerScope(ctx context.Context, parent, id string) (*IdentityAuthServerScope, error) {
	if err := ValidateAPIClient(c.identityAuthServerScopesAPI, "auth-server scopes API"); err != nil {
		return nil, err
	}
	response, err := c.identityAuthServerScopesAPI.GetAuthServerScope(ctx, parent, id)
	if err != nil {
		if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get auth-server scope: %w", err)
	}
	if response == nil || response.AuthServerScope == nil || response.AuthServerScope.ID != id {
		return nil, fmt.Errorf("missing or inconsistent auth-server scope response")
	}
	return &IdentityAuthServerScope{*response.AuthServerScope}, nil
}

func (c *Client) GetIdentityAuthServerClaim(ctx context.Context, parent, id string) (*IdentityAuthServerClaim, error) {
	if err := ValidateAPIClient(c.identityAuthServerClaimsAPI, "auth-server claims API"); err != nil {
		return nil, err
	}
	response, err := c.identityAuthServerClaimsAPI.GetAuthServerClaim(ctx, parent, id)
	if err != nil {
		if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get auth-server claim: %w", err)
	}
	if response == nil || response.Claim == nil || response.Claim.ID != id {
		return nil, fmt.Errorf("missing or inconsistent auth-server claim response")
	}
	return &IdentityAuthServerClaim{*response.Claim}, nil
}

func (c *Client) CreateIdentityAuthServerScope(ctx context.Context, parent string,
	request kkComps.CreateScope,
) (*IdentityAuthServerScope, error) {
	if err := ValidateAPIClient(c.identityAuthServerScopesAPI, "auth-server scopes API"); err != nil {
		return nil, err
	}
	response, err := c.identityAuthServerScopesAPI.CreateAuthServerScope(ctx, parent, request)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth-server scope: %w", err)
	}
	if response == nil || response.AuthServerScope == nil || response.AuthServerScope.ID == "" {
		return nil, fmt.Errorf("missing created auth-server scope response")
	}
	return &IdentityAuthServerScope{*response.AuthServerScope}, nil
}

func (c *Client) CreateIdentityAuthServerClaim(ctx context.Context, parent string,
	request kkComps.CreateClaim,
) (*IdentityAuthServerClaim, error) {
	if err := ValidateAPIClient(c.identityAuthServerClaimsAPI, "auth-server claims API"); err != nil {
		return nil, err
	}
	// Names are not unique (Q12); neither transport nor SDK may replay a create
	// after an uncertain outcome. Replanning observes any successfully created record.
	response, err := c.identityAuthServerClaimsAPI.CreateAuthServerClaim(httpclient.WithoutRetries(ctx), parent, request,
		kkOps.WithRetries(retry.Config{Strategy: "none"}))
	if err != nil {
		return nil, fmt.Errorf("failed to create auth-server claim: %w", err)
	}
	if response == nil || response.Claim == nil || response.Claim.ID == "" {
		return nil, fmt.Errorf("missing created auth-server claim response")
	}
	return &IdentityAuthServerClaim{*response.Claim}, nil
}

func (c *Client) UpdateIdentityAuthServerScope(ctx context.Context, parent, id string,
	request kkComps.UpdateScope,
) error {
	if err := ValidateAPIClient(c.identityAuthServerScopesAPI, "auth-server scopes API"); err != nil {
		return err
	}
	response, err := c.identityAuthServerScopesAPI.UpdateAuthServerScope(ctx, kkOps.UpdateAuthServerScopeRequest{
		AuthServerID: parent, ScopeID: id, UpdateScope: request,
	})
	if err != nil {
		return fmt.Errorf("failed to update auth-server scope: %w", err)
	}
	if response == nil || response.AuthServerScope == nil || response.AuthServerScope.ID != id {
		return fmt.Errorf("missing updated auth-server scope response")
	}
	return nil
}

func (c *Client) UpdateIdentityAuthServerClaim(ctx context.Context, parent, id string,
	request kkComps.UpdateClaim,
) error {
	if err := ValidateAPIClient(c.identityAuthServerClaimsAPI, "auth-server claims API"); err != nil {
		return err
	}
	response, err := c.identityAuthServerClaimsAPI.UpdateAuthServerClaim(ctx, kkOps.UpdateAuthServerClaimRequest{
		AuthServerID: parent, ClaimID: id, UpdateClaim: request,
	})
	if err != nil {
		return fmt.Errorf("failed to update auth-server claim: %w", err)
	}
	if response == nil || response.Claim == nil || response.Claim.ID != id {
		return fmt.Errorf("missing updated auth-server claim response")
	}
	return nil
}

func (c *Client) DeleteIdentityAuthServerScope(ctx context.Context, parent, id string) error {
	if err := ValidateAPIClient(c.identityAuthServerScopesAPI, "auth-server scopes API"); err != nil {
		return err
	}
	_, err := c.identityAuthServerScopesAPI.DeleteAuthServerScope(ctx, parent, id)
	if err != nil && declerrors.ExtractStatusCodeFromError(err) != http.StatusNotFound {
		return fmt.Errorf("failed to delete auth-server scope: %w", err)
	}
	return nil
}

func (c *Client) DeleteIdentityAuthServerClaim(ctx context.Context, parent, id string) error {
	if err := ValidateAPIClient(c.identityAuthServerClaimsAPI, "auth-server claims API"); err != nil {
		return err
	}
	_, err := c.identityAuthServerClaimsAPI.DeleteAuthServerClaim(ctx, parent, id)
	if err != nil && declerrors.ExtractStatusCodeFromError(err) != http.StatusNotFound {
		return fmt.Errorf("failed to delete auth-server claim: %w", err)
	}
	return nil
}
