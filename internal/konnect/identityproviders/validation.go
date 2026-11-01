// Package identityproviders validates public Token Vault provider configuration.
package identityproviders

import (
	"fmt"
	"regexp"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

var identityVaultProviderName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidateName follows the API pattern without borrowing
// the unrelated Vault key name length limit.
func ValidateName(name string) error {
	if !identityVaultProviderName.MatchString(name) {
		return fmt.Errorf("vault provider name must contain only ASCII letters, digits, underscores or hyphens")
	}
	return nil
}

func ValidateFields(scopes []string, refresh *int64, endpoints ...*string) error {
	if scopes != nil && len(scopes) == 0 {
		return fmt.Errorf("empty provider scopes are not supported: create selects defaults and PATCH rejects clearing")
	}
	for _, scope := range scopes {
		if scope == "" {
			return fmt.Errorf("provider scopes must be nonempty RFC 6749 scope-tokens")
		}
		for _, char := range scope {
			if char < 0x21 || char > 0x7e || char == '"' || char == '\\' {
				return fmt.Errorf("provider scopes must be nonempty RFC 6749 scope-tokens")
			}
		}
	}
	if refresh != nil && *refresh < 0 {
		return fmt.Errorf("provider refresh_buffer_seconds must be nonnegative")
	}
	for _, endpoint := range endpoints {
		if endpoint != nil && *endpoint == "" {
			return fmt.Errorf("empty provider endpoints do not restore template defaults and are not supported")
		}
	}
	return nil
}

func ValidateCreate(request kkComps.CreateVaultProviderBody) error {
	if err := ValidateName(request.Name); err != nil {
		return err
	}
	if request.TemplateName == "" {
		return fmt.Errorf("vault provider template_name is required")
	}
	if request.CredentialType != nil && *request.CredentialType != "user" && *request.CredentialType != "shared" {
		return fmt.Errorf("vault provider credential_type must be user or shared")
	}
	if request.TemplateName == kkComps.ProviderTemplateNameStaticSecret {
		if request.CredentialType == nil || *request.CredentialType != "shared" {
			return fmt.Errorf("static_secret providers require explicit credential_type: shared")
		}
		if request.ClientID != nil || request.ClientSecret != nil || request.DcrEnabled != nil && *request.DcrEnabled {
			return fmt.Errorf("static_secret providers cannot configure OAuth clients or enable DCR")
		}
	}
	if request.DcrEnabled != nil && *request.DcrEnabled && request.ClientID != nil {
		return fmt.Errorf("DCR providers must omit client_id")
	}
	return ValidateFields(request.Scopes, request.RefreshBufferSeconds,
		request.TokenEndpoint, request.AuthorizationEndpoint, request.RegistrationEndpoint)
}
