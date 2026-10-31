package state

import (
	"context"
	"fmt"
	"maps"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	decerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/secretoutput"
)

type IdentityAuthServerClient struct {
	kkComps.AuthServerClient
	NormalizedLabels map[string]string
}

func (c IdentityAuthServerClient) GetID() string                          { return c.ID }
func (c IdentityAuthServerClient) GetName() string                        { return c.Name }
func (c IdentityAuthServerClient) GetLabels() map[string]string           { return c.Labels }
func (c IdentityAuthServerClient) GetNormalizedLabels() map[string]string { return c.NormalizedLabels }

func normalizeIdentityClient(value kkComps.AuthServerClient) IdentityAuthServerClient {
	return IdentityAuthServerClient{AuthServerClient: value, NormalizedLabels: maps.Clone(value.Labels)}
}

func (c *Client) CreateIdentityAuthServerClient(ctx context.Context, parent string, request kkComps.CreateClient,
	output *secretoutput.File, metadata secretoutput.Metadata,
) (*IdentityAuthServerClient, error) {
	value, err := helpers.CreateAndCaptureIdentityAuthServerClient(ctx, c.identityAuthServerClientAPI,
		parent, request, output, metadata)
	if err != nil {
		return nil, err
	}
	result := normalizeIdentityClient(*value)
	return &result, nil
}

func (c *Client) GetIdentityAuthServerClient(
	ctx context.Context,
	parent, id string,
) (*IdentityAuthServerClient, error) {
	if err := ValidateAPIClient(c.identityAuthServerClientAPI, "auth-server clients API"); err != nil {
		return nil, err
	}
	response, err := c.identityAuthServerClientAPI.GetAuthServerClient(ctx, parent, id)
	if err != nil {
		if decerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get auth-server client: %w", err)
	}
	if response == nil || response.AuthServerClient == nil || response.AuthServerClient.ID != id {
		return nil, fmt.Errorf("missing or inconsistent auth-server client response")
	}
	value := normalizeIdentityClient(*response.AuthServerClient)
	return &value, nil
}

func (c *Client) UpdateIdentityAuthServerClient(ctx context.Context, parent, id string,
	request kkComps.UpdateClient,
) (*IdentityAuthServerClient, error) {
	if err := ValidateAPIClient(c.identityAuthServerClientAPI, "auth-server clients API"); err != nil {
		return nil, err
	}
	if request.ClientSecret != nil {
		return nil, fmt.Errorf("ordinary client updates cannot replace secret material")
	}
	response, err := c.identityAuthServerClientAPI.UpdateAuthServerClient(ctx, kkOps.UpdateAuthServerClientRequest{
		AuthServerID: parent, ClientID: id, UpdateClient: request,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update auth-server client: %w", err)
	}
	if response == nil || response.AuthServerClient == nil || response.AuthServerClient.ID != id {
		return nil, fmt.Errorf("missing or inconsistent auth-server client update response")
	}
	value := normalizeIdentityClient(*response.AuthServerClient)
	return &value, nil
}

func (c *Client) DeleteIdentityAuthServerClient(ctx context.Context, parent, id string) error {
	if err := ValidateAPIClient(c.identityAuthServerClientAPI, "auth-server clients API"); err != nil {
		return err
	}
	response, err := c.identityAuthServerClientAPI.DeleteAuthServerClient(ctx, parent, id)
	if err != nil {
		if decerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("failed to delete auth-server client: %w", err)
	}
	if response == nil || response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("missing or inconsistent auth-server client delete response")
	}
	return nil
}
