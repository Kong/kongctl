package konnect

import (
	"context"
	"fmt"
	"slices"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type VaultProvidersAPI struct {
	Credentials      map[string][]helpers.IdentityVaultProviderCredential
	CredentialWrites int
	DirectoryDependenciesAPI
	Providers                                              []helpers.IdentityVaultProvider
	AttachedCredentials                                    map[string]bool
	Writes, Creates, Updates, Deletes, Lists, SecretWrites int
	Counter                                                int
}

func (a *VaultProvidersAPI) ListVaultProviders(
	_ context.Context,
	request kkOps.ListVaultProvidersRequest,
	_ ...kkOps.Option,
) (*kkOps.ListVaultProvidersResponse, error) {
	a.Lists++
	values := make([]helpers.IdentityVaultProvider, 0)
	for _, value := range a.Providers {
		if value.DirectoryID == request.DirectoryID {
			values = append(values, value)
		}
	}
	return &kkOps.ListVaultProvidersResponse{StatusCode: 200, RawResponse: vaultKeyResponse(map[string]any{
		"data": values, "meta": map[string]any{"page": map[string]any{"size": 100, "next": nil}},
	})}, nil
}

func (a *VaultProvidersAPI) GetVaultProvider(
	_ context.Context,
	directory, id string,
	_ ...kkOps.Option,
) (*kkOps.GetVaultProviderResponse, error) {
	for _, value := range a.Providers {
		if value.ID == id && value.DirectoryID == directory {
			return &kkOps.GetVaultProviderResponse{StatusCode: 200, RawResponse: vaultKeyResponse(value)}, nil
		}
	}
	return nil, &sdkerrors.SDKError{StatusCode: 404}
}

func providerFixtureTemplate(name string) helpers.IdentityCredentialProviderTemplate {
	value := helpers.IdentityCredentialProviderTemplate{
		Name: name, DisplayName: name, SecretType: "authorization_code", BearerMethodsSupported: []string{"header"},
		DefaultScopes: []string{}, PKCERequired: new(false),
	}
	if name == "static_secret" {
		value.SecretType = "static_secret"
	}
	if name == "github" {
		value.DefaultScopes = []string{"repo"}
		value.TokenEndpoint = new("https://github.example.test/token")
		value.AuthorizationEndpoint = new("https://github.example.test/authorize")
	}
	return value
}

func (a *VaultProvidersAPI) GetCredentialProviderTemplate(
	_ context.Context,
	name kkComps.ProviderTemplateName,
	_ ...kkOps.Option,
) (*kkOps.GetCredentialProviderTemplateResponse, error) {
	return &kkOps.GetCredentialProviderTemplateResponse{
		StatusCode:  200,
		RawResponse: vaultKeyResponse(providerFixtureTemplate(string(name))),
	}, nil
}

func (a *VaultProvidersAPI) ListCredentialProviderTemplates(
	context.Context,
	*kkComps.CursorPageParameters,
	...kkOps.Option,
) (*kkOps.ListCredentialProviderTemplatesResponse, error) {
	return &kkOps.ListCredentialProviderTemplatesResponse{StatusCode: 200, RawResponse: vaultKeyResponse(map[string]any{
		"data": []helpers.IdentityCredentialProviderTemplate{
			providerFixtureTemplate("static_secret"),
			providerFixtureTemplate("github"),
			providerFixtureTemplate("custom-oauth"),
		},
		"meta": map[string]any{"page": map[string]any{"size": 100, "next": nil}},
	})}, nil
}

func (a *VaultProvidersAPI) CreateVaultProvider(
	_ context.Context,
	directory string,
	request kkComps.CreateVaultProviderBody,
	_ ...kkOps.Option,
) (*kkOps.CreateVaultProviderResponse, error) {
	for _, value := range a.Providers {
		if value.DirectoryID == directory && value.Name == request.Name {
			return nil, &sdkerrors.SDKError{StatusCode: 409}
		}
	}
	a.Counter++
	template := providerFixtureTemplate(string(request.TemplateName))
	value := helpers.IdentityVaultProvider{
		ID: fmt.Sprintf("731559d3-39bd-48c7-b33c-%012d", a.Counter), DirectoryID: directory, Name: request.Name,
		TemplateName: string(request.TemplateName), BearerMethodsSupported: []string{"header"}, Enabled: new(true),
		ClientID: request.ClientID, Scopes: slices.Clone(template.DefaultScopes),
		DCREnabled: new(false), PKCEEnabled: new(false),
		CredentialType: "user", RefreshBufferSeconds: new(int64(0)), TokenEndpointEffective: template.TokenEndpoint,
		AuthorizationEndpointEffective: template.AuthorizationEndpoint,
		CreatedAt: time.Date(
			2026,
			10,
			31,
			12,
			0,
			0,
			0,
			time.UTC,
		), UpdatedAt: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC),
	}
	if request.Scopes != nil {
		value.Scopes = slices.Clone(request.Scopes)
	}
	if request.CredentialType != nil {
		value.CredentialType = *request.CredentialType
	}
	if request.DcrEnabled != nil {
		value.DCREnabled = request.DcrEnabled
	}
	if request.PkceEnabled != nil {
		value.PKCEEnabled = request.PkceEnabled
	}
	if request.RefreshBufferSeconds != nil {
		value.RefreshBufferSeconds = request.RefreshBufferSeconds
	}
	if request.TokenEndpoint != nil {
		value.TokenEndpointEffective = request.TokenEndpoint
	}
	if request.AuthorizationEndpoint != nil {
		value.AuthorizationEndpointEffective = request.AuthorizationEndpoint
	}
	if request.RegistrationEndpoint != nil {
		value.RegistrationEndpointEffective = request.RegistrationEndpoint
	}
	if request.ClientSecret != nil {
		a.SecretWrites++
	}
	a.Providers = append(a.Providers, value)
	a.Writes++
	a.Creates++
	return &kkOps.CreateVaultProviderResponse{StatusCode: 201, RawResponse: vaultKeyResponse(value)}, nil
}

func (a *VaultProvidersAPI) UpdateVaultProvider(
	_ context.Context,
	request kkOps.UpdateVaultProviderRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateVaultProviderResponse, error) {
	for i := range a.Providers {
		value := &a.Providers[i]
		if value.ID != request.ProviderID || value.DirectoryID != request.DirectoryID {
			continue
		}
		body := request.PatchVaultProviderBody
		if body.Name != nil {
			value.Name = *body.Name
		}
		if body.Enabled != nil {
			value.Enabled = body.Enabled
		}
		if body.Scopes != nil {
			value.Scopes = slices.Clone(body.Scopes)
		}
		if body.PkceEnabled != nil {
			value.PKCEEnabled = body.PkceEnabled
		}
		if body.RefreshBufferSeconds != nil {
			value.RefreshBufferSeconds = body.RefreshBufferSeconds
		}
		if body.TokenEndpoint != nil {
			value.TokenEndpointEffective = body.TokenEndpoint
		}
		if body.AuthorizationEndpoint != nil {
			value.AuthorizationEndpointEffective = body.AuthorizationEndpoint
		}
		if body.ClientSecret != nil {
			a.SecretWrites++
		}
		a.Writes++
		a.Updates++
		return &kkOps.UpdateVaultProviderResponse{StatusCode: 200, RawResponse: vaultKeyResponse(value)}, nil
	}
	return nil, &sdkerrors.SDKError{StatusCode: 404}
}

func (a *VaultProvidersAPI) ListVaultProviderCredentials(
	_ context.Context,
	request kkOps.ListVaultProviderCredentialsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListVaultProviderCredentialsResponse, error) {
	values := make([]any, 0)
	for _, value := range a.Credentials[request.ProviderID] {
		values = append(values, value)
	}
	if a.AttachedCredentials[request.ProviderID] {
		values = append(values, map[string]any{"id": "attached"})
	}
	return &kkOps.ListVaultProviderCredentialsResponse{StatusCode: 200, RawResponse: vaultKeyResponse(map[string]any{
		"data": values, "meta": map[string]any{"page": map[string]any{"size": 100, "next": nil}},
	})}, nil
}

func (a *VaultProvidersAPI) DeleteVaultProvider(
	_ context.Context,
	directory, id string,
	_ ...kkOps.Option,
) (*kkOps.DeleteVaultProviderResponse, error) {
	for i, value := range a.Providers {
		if value.DirectoryID == directory && value.ID == id {
			a.Providers = append(a.Providers[:i], a.Providers[i+1:]...)
			a.Writes++
			a.Deletes++
			return &kkOps.DeleteVaultProviderResponse{StatusCode: 204}, nil
		}
	}
	return nil, &sdkerrors.SDKError{StatusCode: 404}
}
