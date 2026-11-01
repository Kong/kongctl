package konnect

import (
	"context"
	"fmt"
	"slices"
	"time"

	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

func (a *VaultProvidersAPI) CreateVaultProviderCredential(
	_ context.Context,
	r kkOps.CreateVaultProviderCredentialRequest,
	_ ...kkOps.Option,
) (*kkOps.CreateVaultProviderCredentialResponse, error) {
	if a.Credentials == nil {
		a.Credentials = make(map[string][]helpers.IdentityVaultProviderCredential)
	}
	a.Counter++
	value := helpers.IdentityVaultProviderCredential{
		ID: fmt.Sprintf("731559d3-39bd-48c7-b33c-%012d", a.Counter), CredentialType: "shared", Status: "active",
		Selector: r.SetVaultProviderCredentialBody.Selector, Priority: r.SetVaultProviderCredentialBody.Priority,
		CreatedAt: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC),
	}
	a.Credentials[r.ProviderID] = append(a.Credentials[r.ProviderID], value)
	a.CredentialWrites++
	return &kkOps.CreateVaultProviderCredentialResponse{StatusCode: 201, RawResponse: vaultKeyResponse(value)}, nil
}

func (a *VaultProvidersAPI) UpdateVaultProviderCredential(
	_ context.Context,
	r kkOps.UpdateVaultProviderCredentialRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateVaultProviderCredentialResponse, error) {
	for i := range a.Credentials[r.ProviderID] {
		value := &a.Credentials[r.ProviderID][i]
		if value.ID == r.CredentialID {
			value.Selector, value.Priority = r.SetVaultProviderCredentialBody.Selector, r.SetVaultProviderCredentialBody.Priority
			a.CredentialWrites++
			return &kkOps.UpdateVaultProviderCredentialResponse{
				StatusCode:  200,
				RawResponse: vaultKeyResponse(value),
			}, nil
		}
	}
	return nil, &sdkerrors.SDKError{StatusCode: 404}
}

func (a *VaultProvidersAPI) DeleteVaultProviderCredential(
	_ context.Context,
	r kkOps.DeleteVaultProviderCredentialRequest,
	_ ...kkOps.Option,
) (*kkOps.DeleteVaultProviderCredentialResponse, error) {
	for i, value := range a.Credentials[r.ProviderID] {
		if value.ID == r.CredentialID {
			a.Credentials[r.ProviderID] = slices.Delete(a.Credentials[r.ProviderID], i, i+1)
			a.CredentialWrites++
			return &kkOps.DeleteVaultProviderCredentialResponse{StatusCode: 204}, nil
		}
	}
	return nil, &sdkerrors.SDKError{StatusCode: 404}
}
