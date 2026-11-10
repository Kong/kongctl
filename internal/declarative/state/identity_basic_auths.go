package state

import (
	"context"
	"fmt"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	declerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type IdentityBasicAuth struct {
	helpers.IdentityBasicAuth
	NormalizedLabels map[string]string
}

func (b IdentityBasicAuth) GetID() string                          { return b.ID }
func (b IdentityBasicAuth) GetName() string                        { return b.Username }
func (b IdentityBasicAuth) GetLabels() map[string]string           { return nil }
func (b IdentityBasicAuth) GetManagedBy() map[string]string        { return b.ManagedBy }
func (b IdentityBasicAuth) GetNormalizedLabels() map[string]string { return b.NormalizedLabels }

func normalizeIdentityBasicAuth(value helpers.IdentityBasicAuth) IdentityBasicAuth {
	var ownership map[string]string
	if value.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool {
		ownership = resources.NormalizedChildOwnership(value.ManagedBy)
	}
	return IdentityBasicAuth{IdentityBasicAuth: value, NormalizedLabels: ownership}
}

func (c *Client) ListIdentityBasicAuths(
	ctx context.Context,
	directoryID, principalID string,
) ([]IdentityBasicAuth, error) {
	values, err := helpers.ListIdentityBasicAuths(ctx, c.identityBasicAuthAPI, directoryID, principalID, 100)
	if err != nil {
		return nil, err
	}
	result := make([]IdentityBasicAuth, 0, len(values))
	for _, value := range values {
		result = append(result, normalizeIdentityBasicAuth(value))
	}
	return result, nil
}

func (c *Client) GetIdentityBasicAuth(
	ctx context.Context,
	directoryID, principalID, id string,
) (*IdentityBasicAuth, error) {
	values, err := c.ListIdentityBasicAuths(ctx, directoryID, principalID)
	if err != nil {
		if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
			parent, lookupErr := c.GetIdentityPrincipal(ctx, directoryID, principalID)
			if lookupErr != nil {
				return nil, lookupErr
			}
			if parent == nil {
				return nil, nil
			}
		}
		return nil, err
	}
	for _, value := range values {
		if value.ID != id {
			continue
		}
		matches := 0
		for _, other := range values {
			if other.Username == value.Username {
				matches++
			}
		}
		if matches != 1 {
			return nil, fmt.Errorf("multiple basic-auth credentials match the selected username")
		}
		fresh, err := helpers.GetIdentityBasicAuth(ctx, c.identityBasicAuthAPI, directoryID, principalID, id)
		if err != nil {
			if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
				return nil, nil
			}
			return nil, err
		}
		if fresh.Username != value.Username {
			return nil, fmt.Errorf("basic-auth username changed during lookup")
		}
		result := normalizeIdentityBasicAuth(*fresh)
		return &result, nil
	}
	return nil, nil
}

func (c *Client) CreateIdentityBasicAuth(ctx context.Context, directoryID, principalID string,
	request kkComps.CreateBasicAuthBody,
) (*IdentityBasicAuth, error) {
	if err := stampManagedByRequest(ctx, resources.ResourceTypeIdentityBasicAuth, &request); err != nil {
		return nil, err
	}
	value, err := helpers.CreateIdentityBasicAuth(ctx, c.identityBasicAuthAPI, directoryID, principalID, request)
	if err != nil {
		return nil, err
	}
	result := normalizeIdentityBasicAuth(*value)
	return &result, nil
}

func (c *Client) ReplaceIdentityBasicAuth(ctx context.Context, directoryID, principalID, id string,
	request kkComps.ReplaceBasicAuthBody,
) error {
	if err := stampManagedByRequest(ctx, resources.ResourceTypeIdentityBasicAuth, &request); err != nil {
		return err
	}
	_, err := helpers.ReplaceIdentityBasicAuth(ctx, c.identityBasicAuthAPI, directoryID, principalID, id, request)
	return err
}

func (c *Client) VerifyIdentityBasicAuthEmpty(ctx context.Context, directoryID, principalID, id string) error {
	return helpers.VerifyIdentityBasicAuthEmpty(ctx, c.identityBasicAuthAPI, directoryID, principalID, id)
}

func (c *Client) DeleteIdentityBasicAuth(ctx context.Context, directoryID, principalID, id string) error {
	return helpers.DeleteIdentityBasicAuth(ctx, c.identityBasicAuthAPI, directoryID, principalID, id)
}

func (c *Client) VerifyPrincipalAPIKeysEmpty(ctx context.Context, directoryID, principalID string) error {
	return helpers.VerifyPrincipalAPIKeysEmpty(ctx, c.identityPrincipalKeysReader, directoryID, principalID)
}

func (c *Client) VerifyPrincipalCredentialsEmpty(ctx context.Context, directoryID, principalID string) error {
	values, err := c.ListIdentityBasicAuths(ctx, directoryID, principalID)
	if err != nil {
		return fmt.Errorf("cannot verify principal basic-auth credentials: %w", err)
	}
	if len(values) != 0 {
		return fmt.Errorf("principal still has basic-auth credentials; remove them explicitly before deleting it")
	}
	return c.VerifyPrincipalAPIKeysEmpty(ctx, directoryID, principalID)
}
