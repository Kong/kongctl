package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/pagination"
)

type IdentityVaultProvidersReader interface {
	ListVaultProviders(context.Context, kkOps.ListVaultProvidersRequest, ...kkOps.Option) (
		*kkOps.ListVaultProvidersResponse, error,
	)
	GetVaultProvider(context.Context, string, string, ...kkOps.Option) (*kkOps.GetVaultProviderResponse, error)
	ListCredentialProviderTemplates(context.Context, *kkComps.CursorPageParameters, ...kkOps.Option) (
		*kkOps.ListCredentialProviderTemplatesResponse, error,
	)
	GetCredentialProviderTemplate(context.Context, kkComps.ProviderTemplateName, ...kkOps.Option) (
		*kkOps.GetCredentialProviderTemplateResponse, error,
	)
}

// IdentityVaultProvider is public configuration only. Client secrets, credential
// values and enrollment capabilities cannot enter ordinary inspection output.
type IdentityVaultProvider struct {
	ID                             string    `json:"id"                               yaml:"id"`
	DirectoryID                    string    `json:"directory_id"                     yaml:"directory_id"`
	Name                           string    `json:"name"                             yaml:"name"`
	TemplateName                   string    `json:"template_name"                    yaml:"template_name"`
	BearerMethodsSupported         []string  `json:"bearer_methods_supported"         yaml:"bearer_methods_supported"`
	Enabled                        *bool     `json:"enabled"                          yaml:"enabled"`
	ClientID                       *string   `json:"client_id,omitempty"              yaml:"client_id,omitempty"`
	Scopes                         []string  `json:"scopes"                           yaml:"scopes"`
	TokenEndpointEffective         *string   `json:"token_endpoint_effective"         yaml:"token_endpoint_effective"`
	AuthorizationEndpointEffective *string   `json:"authorization_endpoint_effective" yaml:"authorization_endpoint_effective"` //nolint:lll
	RegistrationEndpointEffective  *string   `json:"registration_endpoint_effective"  yaml:"registration_endpoint_effective"`  //nolint:lll
	DCREnabled                     *bool     `json:"dcr_enabled"                      yaml:"dcr_enabled"`
	CredentialType                 string    `json:"credential_type"                  yaml:"credential_type"`
	PKCEEnabled                    *bool     `json:"pkce_enabled"                     yaml:"pkce_enabled"`
	RefreshBufferSeconds           *int64    `json:"refresh_buffer_seconds"           yaml:"refresh_buffer_seconds"`
	CreatedAt                      time.Time `json:"created_at"                       yaml:"created_at"`
	UpdatedAt                      time.Time `json:"updated_at"                       yaml:"updated_at"`
}

func (v *IdentityVaultProvider) UnmarshalJSON(data []byte) error {
	type alias IdentityVaultProvider
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("invalid Vault provider public configuration")
	}
	if !util.IsValidUUID(value.ID) || !util.IsValidUUID(value.DirectoryID) || value.Name == "" ||
		value.TemplateName == "" || value.BearerMethodsSupported == nil || value.Scopes == nil ||
		value.Enabled == nil || value.DCREnabled == nil || value.PKCEEnabled == nil || value.CredentialType == "" ||
		value.RefreshBufferSeconds == nil || *value.RefreshBufferSeconds < 0 ||
		value.CreatedAt.IsZero() || value.UpdatedAt.IsZero() {
		return fmt.Errorf("missing or invalid Vault provider identity or required configuration")
	}
	*v = IdentityVaultProvider(value)
	return nil
}

type IdentityCredentialProviderTemplate struct {
	Name                   string   `json:"name"                     yaml:"name"`
	DisplayName            string   `json:"display_name"             yaml:"display_name"`
	SecretType             string   `json:"secret_type"              yaml:"secret_type"`
	BearerMethodsSupported []string `json:"bearer_methods_supported" yaml:"bearer_methods_supported"`
	DefaultScopes          []string `json:"default_scopes"           yaml:"default_scopes"`
	PKCERequired           *bool    `json:"pkce_required,omitempty"  yaml:"pkce_required,omitempty"`
	TokenEndpoint          *string  `json:"token_endpoint"           yaml:"token_endpoint"`
	AuthorizationEndpoint  *string  `json:"authorization_endpoint"   yaml:"authorization_endpoint"`
	RegistrationEndpoint   *string  `json:"registration_endpoint"    yaml:"registration_endpoint"`
}

func (t *IdentityCredentialProviderTemplate) UnmarshalJSON(data []byte) error {
	type alias IdentityCredentialProviderTemplate
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("invalid credential provider template public configuration")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("invalid credential provider template response")
	}
	if value.Name == "" || value.DisplayName == "" || value.SecretType == "" ||
		value.BearerMethodsSupported == nil || value.DefaultScopes == nil ||
		len(fields["token_endpoint"]) == 0 || len(fields["authorization_endpoint"]) == 0 {
		return fmt.Errorf("missing or invalid credential provider template identity or required configuration")
	}
	*t = IdentityCredentialProviderTemplate(value)
	return nil
}

func decodeIdentityVaultProviderResponse(response *http.Response, destination any) error {
	if response == nil || response.Body == nil {
		return fmt.Errorf("missing Vault provider HTTP response")
	}
	data, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("cannot read Vault provider response")
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("cannot decode Vault provider public response")
	}
	return nil
}

func listIdentityVaultPublicCollection[T any](pageSize int64, resource string,
	fetch func(*kkComps.CursorPageParameters) (*http.Response, error), identify func(T) (string, error),
) ([]T, error) {
	if pageSize < 1 {
		pageSize = 100
	}
	request := &kkComps.CursorPageParameters{Size: &pageSize}
	result := make([]T, 0)
	seen := make(map[string]bool)
	var tracker pagination.CursorTracker
	for {
		response, err := fetch(request)
		if err != nil {
			return nil, err
		}
		var page identityCredentialPage[T]
		if err := decodeIdentityVaultProviderResponse(response, &page); err != nil {
			return nil, err
		}
		if page.Meta == nil || page.Meta.Page == nil || page.Meta.Page.Size == nil ||
			math.Trunc(*page.Meta.Page.Size) != *page.Meta.Page.Size {
			return nil, fmt.Errorf("invalid %s pagination", resource)
		}
		next, err := page.next()
		if err != nil {
			return nil, fmt.Errorf("invalid %s collection: %w", resource, err)
		}
		for _, value := range page.Data {
			id, err := identify(value)
			if err != nil {
				return nil, err
			}
			if id == "" || seen[id] {
				return nil, fmt.Errorf("%s collection repeats a resource identity", resource)
			}
			seen[id] = true
			result = append(result, value)
		}
		cursor, err := tracker.Next(next)
		if err != nil {
			return nil, fmt.Errorf("cannot paginate %s: %w", resource, err)
		}
		if cursor == "" {
			return result, nil
		}
		request.After = &cursor
	}
}

func ListIdentityVaultProviders(ctx context.Context, api IdentityVaultProvidersReader, directory string,
	pageSize int64,
) ([]IdentityVaultProvider, error) {
	if api == nil {
		return nil, fmt.Errorf("identity Vault provider API is not configured")
	}
	if !util.IsValidUUID(directory) {
		return nil, fmt.Errorf("directory ID must be a UUID")
	}
	names := make(map[string]bool)
	return listIdentityVaultPublicCollection(pageSize, "vault provider",
		func(page *kkComps.CursorPageParameters) (*http.Response, error) {
			response, err := api.ListVaultProviders(
				ctx,
				kkOps.ListVaultProvidersRequest{DirectoryID: directory, Page: page},
			)
			if err != nil {
				return nil, identitySecretAPIError("list Vault providers", err)
			}
			if response == nil {
				return nil, fmt.Errorf("missing Vault provider collection response")
			}
			return response.RawResponse, nil
		}, func(value IdentityVaultProvider) (string, error) {
			if value.DirectoryID != directory || names[value.Name] {
				return "", fmt.Errorf("vault provider collection contains a different directory or duplicate name")
			}
			names[value.Name] = true
			return value.ID, nil
		})
}

func GetIdentityVaultProvider(ctx context.Context, api IdentityVaultProvidersReader, directory, id string,
) (*IdentityVaultProvider, error) {
	if api == nil {
		return nil, fmt.Errorf("identity Vault provider API is not configured")
	}
	if !util.IsValidUUID(directory) || !util.IsValidUUID(id) {
		return nil, fmt.Errorf("directory and provider IDs must be UUIDs")
	}
	response, err := api.GetVaultProvider(ctx, directory, id)
	if err != nil {
		return nil, identitySecretAPIError("get Vault provider", err)
	}
	if response == nil {
		return nil, fmt.Errorf("missing Vault provider detail response")
	}
	var value IdentityVaultProvider
	if err := decodeIdentityVaultProviderResponse(response.RawResponse, &value); err != nil {
		return nil, err
	}
	if value.ID != id || value.DirectoryID != directory {
		return nil, fmt.Errorf("vault provider response differs from the selected directory/provider identity")
	}
	return &value, nil
}

func ResolveIdentityVaultProviderName(ctx context.Context, api IdentityVaultProvidersReader, directory, name string,
	pageSize int64,
) (*IdentityVaultProvider, error) {
	if name == "" {
		return nil, fmt.Errorf("provider name must not be empty")
	}
	values, err := ListIdentityVaultProviders(ctx, api, directory, pageSize)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if value.Name != name {
			continue
		}
		current, err := GetIdentityVaultProvider(ctx, api, directory, value.ID)
		if err != nil {
			return nil, err
		}
		if current.Name != name {
			return nil, fmt.Errorf("vault provider name changed during lookup")
		}
		return current, nil
	}
	return nil, fmt.Errorf("vault provider name %q not found in the selected directory", name)
}

func ListIdentityCredentialProviderTemplates(ctx context.Context, api IdentityVaultProvidersReader,
	pageSize int64,
) ([]IdentityCredentialProviderTemplate, error) {
	if api == nil {
		return nil, fmt.Errorf("identity Vault provider API is not configured")
	}
	return listIdentityVaultPublicCollection(pageSize, "credential provider template",
		func(page *kkComps.CursorPageParameters) (*http.Response, error) {
			response, err := api.ListCredentialProviderTemplates(ctx, page)
			if err != nil {
				return nil, identitySecretAPIError("list credential provider templates", err)
			}
			if response == nil {
				return nil, fmt.Errorf("missing credential provider template collection response")
			}
			return response.RawResponse, nil
		}, func(value IdentityCredentialProviderTemplate) (string, error) { return value.Name, nil })
}

func GetIdentityCredentialProviderTemplate(ctx context.Context, api IdentityVaultProvidersReader, name string,
) (*IdentityCredentialProviderTemplate, error) {
	if api == nil {
		return nil, fmt.Errorf("identity Vault provider API is not configured")
	}
	if name == "" {
		return nil, fmt.Errorf("credential provider template name must not be empty")
	}
	response, err := api.GetCredentialProviderTemplate(ctx, kkComps.ProviderTemplateName(name))
	if err != nil {
		return nil, identitySecretAPIError("get credential provider template", err)
	}
	if response == nil {
		return nil, fmt.Errorf("missing credential provider template detail response")
	}
	var value IdentityCredentialProviderTemplate
	if err := decodeIdentityVaultProviderResponse(response.RawResponse, &value); err != nil {
		return nil, err
	}
	if value.Name != name {
		return nil, fmt.Errorf("credential provider template response differs from the selected name")
	}
	return &value, nil
}
