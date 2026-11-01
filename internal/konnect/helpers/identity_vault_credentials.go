package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/util"
)

// This interface deliberately excludes credential detail GET: for shared OAuth
// credentials that operation mints a new enrollment capability.
type IdentityVaultProviderCredentialsReader interface {
	IdentityVaultProviderCredentialsLister
}
type IdentityPrincipalVaultCredentialsReader interface {
	ListVaultCredentials(
		context.Context,
		kkOps.ListVaultCredentialsRequest,
		...kkOps.Option,
	) (*kkOps.ListVaultCredentialsResponse, error)
}

type IdentityVaultCredentialPrincipal struct {
	ID          string `json:"id"           yaml:"id"`
	DisplayName string `json:"display_name" yaml:"display_name"`
}

// IdentityVaultProviderCredential contains only public metadata, regardless of
// additional secret/capability fields in an SDK or HTTP response.
type IdentityVaultProviderCredential struct {
	ID             string                            `json:"id"                  yaml:"id"`
	CredentialType string                            `json:"credential_type"     yaml:"credential_type"`
	Status         string                            `json:"status"              yaml:"status"`
	Selector       *string                           `json:"selector"            yaml:"selector"`
	Priority       *int64                            `json:"priority"            yaml:"priority"`
	Principal      *IdentityVaultCredentialPrincipal `json:"principal,omitempty" yaml:"principal,omitempty"`
	ExpiresAt      *time.Time                        `json:"expires_at"          yaml:"expires_at"`
	CreatedAt      time.Time                         `json:"created_at"          yaml:"created_at"`
	UpdatedAt      time.Time                         `json:"updated_at"          yaml:"updated_at"`
}

func (v IdentityVaultProviderCredential) publicFields() map[string]any {
	fields := map[string]any{
		"id": v.ID, "credential_type": v.CredentialType, "status": v.Status,
		"created_at": v.CreatedAt, "updated_at": v.UpdatedAt,
	}
	if v.CredentialType == "shared" {
		fields["selector"] = v.Selector
		fields["priority"] = v.Priority
	} else {
		fields["principal"] = v.Principal
		fields["expires_at"] = v.ExpiresAt
	}
	return fields
}

func (v IdentityVaultProviderCredential) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.publicFields())
}
func (v IdentityVaultProviderCredential) MarshalYAML() (any, error) { return v.publicFields(), nil }

type IdentityVaultCredentialProvider struct {
	ID   string `json:"id"   yaml:"id"`
	Name string `json:"name" yaml:"name"`
}
type IdentityPrincipalVaultCredential struct {
	ID           string                          `json:"id"            yaml:"id"`
	Provider     IdentityVaultCredentialProvider `json:"provider"      yaml:"provider"`
	TemplateName string                          `json:"template_name" yaml:"template_name"`
	Status       string                          `json:"status"        yaml:"status"`
	ExpiresAt    *time.Time                      `json:"expires_at"    yaml:"expires_at"`
	CreatedAt    time.Time                       `json:"created_at"    yaml:"created_at"`
	UpdatedAt    time.Time                       `json:"updated_at"    yaml:"updated_at"`
}

func ListIdentityVaultProviderCredentials(ctx context.Context, api IdentityVaultProviderCredentialsReader,
	directory, provider string, pageSize int64,
) ([]IdentityVaultProviderCredential, error) {
	if api == nil {
		return nil, fmt.Errorf("identity Vault provider credential reader is not configured")
	}
	if !util.IsValidUUID(directory) || !util.IsValidUUID(provider) {
		return nil, fmt.Errorf("directory and provider IDs must be UUIDs")
	}
	return listIdentityVaultPublicCollection(pageSize, "Vault provider credential",
		func(page *kkComps.CursorPageParameters) (*http.Response, error) {
			response, err := api.ListVaultProviderCredentials(ctx, kkOps.ListVaultProviderCredentialsRequest{
				DirectoryID: directory, ProviderID: provider, Page: page,
			})
			if err != nil {
				return nil, identitySecretAPIError("list Vault provider credentials", err)
			}
			if response == nil || response.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("missing or unsuccessful Vault provider credential collection")
			}
			return response.RawResponse, nil
		}, func(value IdentityVaultProviderCredential) (string, error) {
			if !util.IsValidUUID(value.ID) || value.Status == "" || value.CreatedAt.IsZero() ||
				value.UpdatedAt.IsZero() {
				return "", fmt.Errorf("missing or invalid Vault provider credential public identity")
			}
			switch value.CredentialType {
			case "shared":
				if (value.Selector == nil) != (value.Priority == nil) || value.Priority != nil && *value.Priority < 0 {
					return "", fmt.Errorf("invalid shared credential routing metadata")
				}
			case "user":
				if value.Principal == nil || !util.IsValidUUID(value.Principal.ID) {
					return "", fmt.Errorf("missing user credential principal")
				}
			default:
				return "", fmt.Errorf("unrecognized Vault provider credential type")
			}
			return value.ID, nil
		})
}

func ListIdentityPrincipalVaultCredentials(ctx context.Context, api IdentityPrincipalVaultCredentialsReader,
	directory, principal string, pageSize int64,
) ([]IdentityPrincipalVaultCredential, error) {
	if api == nil {
		return nil, fmt.Errorf("identity principal Vault credential reader is not configured")
	}
	if !util.IsValidUUID(directory) || !util.IsValidUUID(principal) {
		return nil, fmt.Errorf("directory and principal IDs must be UUIDs")
	}
	return listIdentityVaultPublicCollection(pageSize, "principal Vault credential",
		func(page *kkComps.CursorPageParameters) (*http.Response, error) {
			response, err := api.ListVaultCredentials(ctx, kkOps.ListVaultCredentialsRequest{
				DirectoryID: directory, PrincipalID: principal, Page: page,
			})
			if err != nil {
				return nil, identitySecretAPIError("list principal Vault credentials", err)
			}
			if response == nil || response.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("missing or unsuccessful principal Vault credential collection")
			}
			return response.RawResponse, nil
		}, func(value IdentityPrincipalVaultCredential) (string, error) {
			if !util.IsValidUUID(value.ID) || !util.IsValidUUID(value.Provider.ID) || value.Provider.Name == "" ||
				value.TemplateName == "" || value.Status == "" || value.CreatedAt.IsZero() || value.UpdatedAt.IsZero() {
				return "", fmt.Errorf("missing or invalid principal Vault credential public metadata")
			}
			return value.ID, nil
		})
}
