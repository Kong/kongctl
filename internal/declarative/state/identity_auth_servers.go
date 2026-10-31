package state

import (
	"context"
	"fmt"
	"maps"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	declerrors "github.com/kong/kongctl/internal/declarative/errors"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type IdentityAuthServer struct {
	kkComps.AuthServer
	NormalizedLabels map[string]string
}

func (a IdentityAuthServer) GetID() string                          { return a.ID }
func (a IdentityAuthServer) GetName() string                        { return a.Name }
func (a IdentityAuthServer) GetLabels() map[string]string           { return a.Labels }
func (a IdentityAuthServer) GetNormalizedLabels() map[string]string { return a.NormalizedLabels }
func normalizeIdentityAuthServer(value kkComps.AuthServer) IdentityAuthServer {
	return IdentityAuthServer{AuthServer: value, NormalizedLabels: maps.Clone(value.Labels)}
}

func (c *Client) ListIdentityAuthServers(ctx context.Context) ([]IdentityAuthServer, error) {
	if err := ValidateAPIClient(c.identityAuthServersAPI, "Kong Identity auth-server API"); err != nil {
		return nil, err
	}
	values, err := helpers.ListIdentityAuthServers(ctx, c.identityAuthServersAPI, 100)
	if err != nil {
		return nil, err
	}
	result := make([]IdentityAuthServer, 0, len(values))
	for _, value := range values {
		result = append(result, normalizeIdentityAuthServer(value))
	}
	return result, nil
}

func (c *Client) GetIdentityAuthServerByName(ctx context.Context, name string) (*IdentityAuthServer, error) {
	values, err := c.ListIdentityAuthServers(ctx)
	if err != nil {
		return nil, err
	}
	var result *IdentityAuthServer
	for i := range values {
		if values[i].Name != name {
			continue
		}
		if result != nil {
			return nil, fmt.Errorf("multiple auth servers have name %q", name)
		}
		result = &values[i]
	}
	return result, nil
}

func (c *Client) GetIdentityAuthServerByID(ctx context.Context, id string) (*IdentityAuthServer, error) {
	if err := ValidateAPIClient(c.identityAuthServersAPI, "Kong Identity auth-server API"); err != nil {
		return nil, err
	}
	response, err := c.identityAuthServersAPI.GetAuthServer(ctx, id)
	if err != nil {
		if declerrors.ExtractStatusCodeFromError(err) == http.StatusNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get auth server: %w", err)
	}
	if response == nil || response.AuthServer == nil || response.AuthServer.ID != id {
		return nil, fmt.Errorf("missing or inconsistent auth-server response")
	}
	value := normalizeIdentityAuthServer(*response.AuthServer)
	return &value, nil
}

func (c *Client) CreateIdentityAuthServer(
	ctx context.Context, request kkComps.CreateAuthServer,
) (*IdentityAuthServer, error) {
	if err := ValidateAPIClient(c.identityAuthServersAPI, "Kong Identity auth-server API"); err != nil {
		return nil, err
	}
	response, err := c.identityAuthServersAPI.CreateAuthServer(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth server: %w", err)
	}
	if response == nil || response.AuthServer == nil || response.AuthServer.ID == "" {
		return nil, fmt.Errorf("missing created auth-server response")
	}
	value := normalizeIdentityAuthServer(*response.AuthServer)
	return &value, nil
}

func (c *Client) UpdateIdentityAuthServer(ctx context.Context, id string, request kkComps.UpdateAuthServer) error {
	if err := ValidateAPIClient(c.identityAuthServersAPI, "Kong Identity auth-server API"); err != nil {
		return err
	}
	response, err := c.identityAuthServersAPI.UpdateAuthServer(ctx, id, request)
	if err != nil {
		return fmt.Errorf("failed to update auth server: %w", err)
	}
	if response == nil || response.AuthServer == nil || response.AuthServer.ID != id {
		return fmt.Errorf("missing updated auth-server response")
	}
	return nil
}

func (c *Client) DeleteIdentityAuthServer(ctx context.Context, id string) error {
	if err := ValidateAPIClient(c.identityAuthServersAPI, "Kong Identity auth-server API"); err != nil {
		return err
	}
	_, err := c.identityAuthServersAPI.DeleteAuthServer(ctx, id, kkOps.DeleteAuthServerForceFalse.ToPointer())
	if err != nil && declerrors.ExtractStatusCodeFromError(err) != http.StatusNotFound {
		return fmt.Errorf("failed to delete auth server without cascading: %w", err)
	}
	return nil
}
