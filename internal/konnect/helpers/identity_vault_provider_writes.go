package helpers

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/konnect/identityproviders"
	"github.com/kong/kongctl/internal/util"
)

type IdentityVaultProviderCredentialsLister interface {
	ListVaultProviderCredentials(context.Context, kkOps.ListVaultProviderCredentialsRequest, ...kkOps.Option) (
		*kkOps.ListVaultProviderCredentialsResponse, error,
	)
}

type IdentityVaultProvidersAPI interface {
	IdentityVaultProvidersReader
	IdentityVaultProviderCredentialsLister
	CreateVaultProvider(context.Context, string, kkComps.CreateVaultProviderBody, ...kkOps.Option) (
		*kkOps.CreateVaultProviderResponse, error,
	)
	UpdateVaultProvider(context.Context, kkOps.UpdateVaultProviderRequest, ...kkOps.Option) (
		*kkOps.UpdateVaultProviderResponse, error,
	)
	DeleteVaultProvider(context.Context, string, string, ...kkOps.Option) (*kkOps.DeleteVaultProviderResponse, error)
}

func validateIdentityVaultProviderScope(api IdentityVaultProvidersReader, directory string) error {
	if api == nil {
		return fmt.Errorf("identity Vault provider API is not configured")
	}
	if !util.IsValidUUID(directory) {
		return fmt.Errorf("directory ID must be a UUID")
	}
	return nil
}

// CreateIdentityVaultProvider makes one create attempt and returns public
// configuration only. enabled is absent from the API create schema.
func CreateIdentityVaultProvider(ctx context.Context, api IdentityVaultProvidersAPI, directory string,
	request kkComps.CreateVaultProviderBody,
) (*IdentityVaultProvider, error) {
	if err := validateIdentityVaultProviderScope(api, directory); err != nil {
		return nil, err
	}
	if err := identityproviders.ValidateCreate(request); err != nil {
		return nil, err
	}
	response, err := api.CreateVaultProvider(httpclient.WithoutRetries(ctx), directory, request,
		kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}))
	if err != nil {
		return nil, identitySecretAPIError("create Vault provider", err)
	}
	if response == nil || response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("missing or unsuccessful created Vault provider response")
	}
	var value IdentityVaultProvider
	if err := decodeIdentityVaultProviderResponse(response.RawResponse, &value); err != nil {
		return nil, err
	}
	if value.DirectoryID != directory || value.Name != request.Name ||
		value.TemplateName != string(request.TemplateName) ||
		!providerOptionalEqual(request.ClientID, value.ClientID) ||
		!providerOptionalEqual(request.CredentialType, &value.CredentialType) ||
		!providerOptionalEqual(request.DcrEnabled, value.DCREnabled) ||
		!providerOptionalEqual(request.PkceEnabled, value.PKCEEnabled) ||
		!providerOptionalEqual(request.RefreshBufferSeconds, value.RefreshBufferSeconds) ||
		!providerOptionalEqual(request.TokenEndpoint, value.TokenEndpointEffective) ||
		!providerOptionalEqual(request.AuthorizationEndpoint, value.AuthorizationEndpointEffective) ||
		!providerOptionalEqual(request.RegistrationEndpoint, value.RegistrationEndpointEffective) ||
		request.Scopes != nil && !slices.Equal(request.Scopes, value.Scopes) {
		return nil, fmt.Errorf(
			"created Vault provider public configuration differs from the request; inspect before retrying",
		)
	}
	return &value, nil
}

// providerOptionalEqual checks only explicitly requested values. It does not
// turn omitted fields into defaults or make claims about write-only secrets.
func providerOptionalEqual[T comparable](requested, observed *T) bool {
	return requested == nil || observed != nil && *requested == *observed
}

func providerNullableEqual[T comparable](a, b *T) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// UpdateIdentityVaultProvider uses PATCH exclusively. Fresh public observations
// verify identity, requested settings and preservation of omitted settings;
// they cannot verify the value or continuity of a stored OAuth client secret.
func UpdateIdentityVaultProvider(ctx context.Context, api IdentityVaultProvidersAPI, directory, id, expectedName string,
	request kkComps.PatchVaultProviderBody,
) (*IdentityVaultProvider, error) {
	if err := identityproviders.ValidateName(expectedName); err != nil {
		return nil, err
	}
	if request.Name != nil {
		if err := identityproviders.ValidateName(*request.Name); err != nil {
			return nil, err
		}
	}
	if err := identityproviders.ValidateFields(request.Scopes, request.RefreshBufferSeconds,
		request.TokenEndpoint, request.AuthorizationEndpoint); err != nil {
		return nil, err
	}
	before, err := GetIdentityVaultProvider(ctx, api, directory, id)
	if err != nil {
		return nil, err
	}
	if before.Name != expectedName {
		return nil, fmt.Errorf("vault provider name changed; regenerate the plan")
	}
	if request.ClientSecret != nil && before.TemplateName == string(kkComps.ProviderTemplateNameStaticSecret) {
		return nil, fmt.Errorf("static_secret providers cannot configure OAuth client secrets")
	}
	response, err := api.UpdateVaultProvider(httpclient.WithoutRetries(ctx), kkOps.UpdateVaultProviderRequest{
		DirectoryID: directory, ProviderID: id, PatchVaultProviderBody: request,
	}, kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}))
	if err != nil {
		return nil, identitySecretAPIError("update Vault provider", err)
	}
	if response == nil || response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("missing or unsuccessful updated Vault provider response")
	}
	var returned IdentityVaultProvider
	if err := decodeIdentityVaultProviderResponse(response.RawResponse, &returned); err != nil {
		return nil, err
	}
	if err := verifyIdentityVaultProviderPatch(*before, returned, request); err != nil {
		return nil, err
	}
	after, err := GetIdentityVaultProvider(ctx, api, directory, id)
	if err != nil {
		return nil, err
	}
	if err := verifyIdentityVaultProviderPatch(*before, *after, request); err != nil {
		return nil, err
	}
	return after, nil
}

func verifyIdentityVaultProviderPatch(
	before, after IdentityVaultProvider,
	request kkComps.PatchVaultProviderBody,
) error {
	expected := before
	if request.Name != nil {
		expected.Name = *request.Name
	}
	if request.Enabled != nil {
		expected.Enabled = request.Enabled
	}
	if request.Scopes != nil {
		expected.Scopes = request.Scopes
	}
	if request.TokenEndpoint != nil {
		expected.TokenEndpointEffective = request.TokenEndpoint
	}
	if request.AuthorizationEndpoint != nil {
		expected.AuthorizationEndpointEffective = request.AuthorizationEndpoint
	}
	if request.PkceEnabled != nil {
		expected.PKCEEnabled = request.PkceEnabled
	}
	if request.RefreshBufferSeconds != nil {
		expected.RefreshBufferSeconds = request.RefreshBufferSeconds
	}
	if after.ID != expected.ID || after.DirectoryID != expected.DirectoryID || after.Name != expected.Name ||
		after.TemplateName != expected.TemplateName || after.CredentialType != expected.CredentialType ||
		!providerNullableEqual(after.ClientID, expected.ClientID) ||
		!providerNullableEqual(after.DCREnabled, expected.DCREnabled) ||
		!providerNullableEqual(after.RegistrationEndpointEffective, expected.RegistrationEndpointEffective) ||
		!providerNullableEqual(after.Enabled, expected.Enabled) ||
		!providerNullableEqual(after.PKCEEnabled, expected.PKCEEnabled) ||
		!providerNullableEqual(after.RefreshBufferSeconds, expected.RefreshBufferSeconds) ||
		!providerNullableEqual(after.TokenEndpointEffective, expected.TokenEndpointEffective) ||
		!providerNullableEqual(after.AuthorizationEndpointEffective, expected.AuthorizationEndpointEffective) ||
		!slices.Equal(after.Scopes, expected.Scopes) ||
		!slices.Equal(after.BearerMethodsSupported, expected.BearerMethodsSupported) {
		return fmt.Errorf(
			"updated Vault provider public configuration was not preserved as requested; inspect before retrying",
		)
	}
	return nil
}

// RequireEmptyIdentityVaultProviderCredentials requests an unfiltered first page
// without the SDK's currently incorrect credential cursor serialization. Only a
// complete, terminal empty response authorizes deletion. It never calls detail
// GET, which can issue enrollment capabilities.
func RequireEmptyIdentityVaultProviderCredentials(ctx context.Context, api IdentityVaultProviderCredentialsLister,
	directory, id string,
) error {
	if api == nil {
		return fmt.Errorf("vault provider credential API is not configured; deletion is blocked")
	}
	if !util.IsValidUUID(directory) || !util.IsValidUUID(id) {
		return fmt.Errorf("vault provider credential observation requires directory and provider UUIDs")
	}
	response, err := api.ListVaultProviderCredentials(ctx, kkOps.ListVaultProviderCredentialsRequest{
		DirectoryID: directory, ProviderID: id,
	})
	if err != nil {
		return identitySecretAPIError("observe Vault provider credentials before deletion", err)
	}
	if response == nil || response.StatusCode != http.StatusOK {
		return fmt.Errorf("missing Vault provider credential collection response; deletion is blocked")
	}
	if err := requireEmptyIdentityCollection(response.RawResponse, "Vault provider", "credentials"); err != nil {
		return fmt.Errorf("cannot delete Vault provider: %w", err)
	}
	return nil
}

// DeleteIdentityVaultProvider checks fresh identity and credential dependencies.
// Callers must separately establish management authority and protection. These
// checks cannot prevent a concurrent credential attachment before API DELETE.
func DeleteIdentityVaultProvider(ctx context.Context, api IdentityVaultProvidersAPI, directory, id, expectedName string,
) error {
	if err := identityproviders.ValidateName(expectedName); err != nil {
		return err
	}
	value, err := GetIdentityVaultProvider(ctx, api, directory, id)
	if err != nil {
		return err
	}
	if value.Name != expectedName {
		return fmt.Errorf("vault provider name changed; regenerate the plan")
	}
	if err := RequireEmptyIdentityVaultProviderCredentials(ctx, api, directory, id); err != nil {
		return err
	}
	response, err := api.DeleteVaultProvider(httpclient.WithoutRetries(ctx), directory, id,
		kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}))
	if err != nil {
		return identitySecretAPIError("delete Vault provider", err)
	}
	if response == nil || response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("missing or unsuccessful deleted Vault provider response")
	}
	return nil
}
