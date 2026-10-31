package helpers

import (
	"context"
	"fmt"
	"regexp"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

var identityClientIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,48}$`)

type IdentityAuthServerClientsAPI interface {
	IdentityAuthServerClientsReader
	CreateAuthServerClient(context.Context, string, kkComps.CreateClient, ...kkOps.Option) (
		*kkOps.CreateAuthServerClientResponse, error,
	)
	UpdateAuthServerClient(context.Context, kkOps.UpdateAuthServerClientRequest, ...kkOps.Option) (
		*kkOps.UpdateAuthServerClientResponse, error,
	)
	DeleteAuthServerClient(context.Context, string, string, ...kkOps.Option) (
		*kkOps.DeleteAuthServerClientResponse, error,
	)
	GetAuthServerClient(context.Context, string, string, ...kkOps.Option) (
		*kkOps.GetAuthServerClientResponse, error,
	)
	ListClientSecrets(context.Context, kkOps.ListClientSecretsRequest, ...kkOps.Option) (
		*kkOps.ListClientSecretsResponse, error,
	)
	GetClientSecret(context.Context, kkOps.GetClientSecretRequest, ...kkOps.Option) (
		*kkOps.GetClientSecretResponse, error,
	)
	CreateClientSecret(context.Context, kkOps.CreateClientSecretRequest, ...kkOps.Option) (
		*kkOps.CreateClientSecretResponse, error,
	)
	UpdateClientSecret(context.Context, kkOps.UpdateClientSecretRequest, ...kkOps.Option) (
		*kkOps.UpdateClientSecretResponse, error,
	)
	DeleteClientSecret(context.Context, kkOps.DeleteClientSecretRequest, ...kkOps.Option) (
		*kkOps.DeleteClientSecretResponse, error,
	)
}

// Secret values are deliberately absent even if a read response contains one.
type IdentityClientSecret struct {
	ID        string    `json:"id"                yaml:"id"`
	Enabled   *bool     `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	CreatedAt time.Time `json:"created_at"        yaml:"created_at"`
	UpdatedAt time.Time `json:"updated_at"        yaml:"updated_at"`
}

func identityClientSecret(value kkComps.ClientSecretItem) IdentityClientSecret {
	return IdentityClientSecret{
		ID: value.ID, Enabled: value.Enabled, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func ResolveIdentityAuthServerClient(ctx context.Context, api IdentityAuthServerClientsAPI,
	serverID, identifier string, byName bool, pageSize int64,
) (*kkComps.AuthServerClient, error) {
	if api == nil {
		return nil, fmt.Errorf("identity auth-server clients API is not configured")
	}
	id := identifier
	if byName {
		values, err := ListIdentityAuthServerClients(ctx, api, serverID, pageSize)
		if err != nil {
			return nil, err
		}
		id, err = authServerChildID(identifier, "clients", values,
			func(value kkComps.AuthServerClient) string { return value.ID },
			func(value kkComps.AuthServerClient) string { return value.Name })
		if err != nil {
			return nil, err
		}
	}
	if !identityClientIDPattern.MatchString(id) {
		return nil, fmt.Errorf("client ID must contain 1–48 ASCII letters, digits, underscores or hyphens")
	}
	response, err := api.GetAuthServerClient(ctx, serverID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth-server client: %w", err)
	}
	if response == nil || response.AuthServerClient == nil || response.AuthServerClient.ID != id ||
		(byName && response.AuthServerClient.Name != identifier) {
		return nil, fmt.Errorf("missing or inconsistent auth-server client response")
	}
	return response.AuthServerClient, nil
}

func ListIdentityClientSecrets(ctx context.Context, api IdentityAuthServerClientsAPI,
	serverID, clientID string, pageSize int64,
) ([]IdentityClientSecret, error) {
	if api == nil {
		return nil, fmt.Errorf("identity auth-server clients API is not configured")
	}
	return listAuthServerChildren(ctx, "client secrets", pageSize, true,
		func(ctx context.Context, number, size int64) ([]IdentityClientSecret, *kkComps.PageMeta, error) {
			response, err := api.ListClientSecrets(ctx, kkOps.ListClientSecretsRequest{
				AuthServerID: serverID, ClientID: clientID, PageNumber: &number, PageSize: &size,
			})
			if err != nil {
				return nil, nil, identitySecretAPIError("list client secrets", err)
			}
			if err != nil || response == nil || response.ClientSecretList == nil ||
				response.ClientSecretList.Data == nil {
				return nil, nil, err
			}
			values := make([]IdentityClientSecret, 0, len(response.ClientSecretList.Data))
			for _, value := range response.ClientSecretList.Data {
				values = append(values, identityClientSecret(value))
			}
			return values, &response.ClientSecretList.Meta.Page, nil
		}, func(value IdentityClientSecret) string { return value.ID })
}

func GetIdentityClientSecret(ctx context.Context, api IdentityAuthServerClientsAPI,
	serverID, clientID, secretID string,
) (*IdentityClientSecret, error) {
	if api == nil {
		return nil, fmt.Errorf("identity auth-server clients API is not configured")
	}
	response, err := api.GetClientSecret(ctx, kkOps.GetClientSecretRequest{
		AuthServerID: serverID, ClientID: clientID, ClientSecretID: secretID,
	})
	if err != nil {
		return nil, identitySecretAPIError("get client secret", err)
	}
	if response == nil || response.ClientSecretItem == nil || response.ClientSecretItem.ID != secretID {
		return nil, fmt.Errorf("missing or inconsistent auth-server client secret response")
	}
	value := identityClientSecret(*response.ClientSecretItem)
	return &value, nil
}
