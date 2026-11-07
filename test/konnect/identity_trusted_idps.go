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
	"github.com/kong/kongctl/internal/konnect/identitytrust"
)

type TrustedIDPsAPI struct {
	DirectoryDependenciesAPI
	Records                                           map[string][]helpers.IdentityTrustedIDP
	Associations                                      func(string, string) []helpers.IdentityPrincipal
	Writes, Creates, Updates, Deletes, Lists, Counter int
}

func (a *TrustedIDPsAPI) ListTrustedIdps(_ context.Context, directory string, _ *kkComps.CursorPageParameters,
	_ ...kkOps.Option,
) (*kkOps.ListTrustedIdpsResponse, error) {
	a.Lists++
	values := slices.Clone(a.Records[directory])
	if values == nil {
		values = []helpers.IdentityTrustedIDP{}
	}
	return &kkOps.ListTrustedIdpsResponse{StatusCode: 200, RawResponse: vaultKeyResponse(map[string]any{
		"data": values, "meta": map[string]any{"page": map[string]any{"size": 100, "next": nil}},
	})}, nil
}

func (a *TrustedIDPsAPI) GetTrustedIdp(
	_ context.Context,
	directory, id string,
	_ ...kkOps.Option,
) (*kkOps.GetTrustedIdpResponse, error) {
	for _, value := range a.Records[directory] {
		if value.ID == id {
			return &kkOps.GetTrustedIdpResponse{StatusCode: 200, RawResponse: vaultKeyResponse(value)}, nil
		}
	}
	return nil, &sdkerrors.SDKError{StatusCode: 404}
}

func (a *TrustedIDPsAPI) ListTrustedIdpPrincipals(_ context.Context, request kkOps.ListTrustedIdpPrincipalsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListTrustedIdpPrincipalsResponse, error) {
	values := []helpers.IdentityPrincipal{}
	if a.Associations != nil {
		values = append(values, a.Associations(request.DirectoryID, request.TrustedIdpID)...)
	}
	return &kkOps.ListTrustedIdpPrincipalsResponse{StatusCode: 200, RawResponse: vaultKeyResponse(map[string]any{
		"data": values, "meta": map[string]any{"page": map[string]any{"size": 100, "next": nil}},
	})}, nil
}

func (a *TrustedIDPsAPI) CreateTrustedIdp(_ context.Context, directory string, request kkComps.CreateTrustedIdpPayload,
	_ ...kkOps.Option,
) (*kkOps.CreateTrustedIdpResponse, error) {
	a.Counter++
	value := helpers.IdentityTrustedIDP{
		ID: fmt.Sprintf("731559d3-39bd-48c7-b33c-%012d", a.Counter), IssuerURL: request.IssuerURL,
		DisplayName: request.DisplayName, JwksURI: request.JwksURI, JitProvisioningEnabled: request.JitProvisioningEnabled,
		VaultAccessEnabled: request.VaultAccessEnabled, Jit: identitytrust.NormalizeJIT(request.Jit),
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
	if value.JitProvisioningEnabled == nil {
		value.JitProvisioningEnabled = new(true)
	}
	if value.VaultAccessEnabled == nil {
		value.VaultAccessEnabled = new(false)
	}
	if a.Records == nil {
		a.Records = make(map[string][]helpers.IdentityTrustedIDP)
	}
	a.Records[directory] = append(a.Records[directory], value)
	a.Writes++
	a.Creates++
	return &kkOps.CreateTrustedIdpResponse{StatusCode: 201, RawResponse: vaultKeyResponse(value)}, nil
}

func (a *TrustedIDPsAPI) UpdateTrustedIdp(_ context.Context, request kkOps.UpdateTrustedIdpRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateTrustedIdpResponse, error) {
	for i := range a.Records[request.DirectoryID] {
		value := &a.Records[request.DirectoryID][i]
		if value.ID != request.TrustedIdpID {
			continue
		}
		body := request.ReplaceTrustedIdpBody
		value.IssuerURL = body.IssuerURL
		value.DisplayName = body.DisplayName
		value.JwksURI = body.JwksURI
		value.JitProvisioningEnabled = body.JitProvisioningEnabled
		value.VaultAccessEnabled = body.VaultAccessEnabled
		if value.JitProvisioningEnabled == nil {
			value.JitProvisioningEnabled = new(true)
		}
		if value.VaultAccessEnabled == nil {
			value.VaultAccessEnabled = new(false)
		}
		value.Jit = identitytrust.NormalizeJIT(body.Jit)
		a.Writes++
		a.Updates++
		return &kkOps.UpdateTrustedIdpResponse{StatusCode: 200, RawResponse: vaultKeyResponse(value)}, nil
	}
	return nil, &sdkerrors.SDKError{StatusCode: 404}
}

func (a *TrustedIDPsAPI) DeleteTrustedIdp(
	_ context.Context,
	directory, id string,
	_ ...kkOps.Option,
) (*kkOps.DeleteTrustedIdpResponse, error) {
	index := slices.IndexFunc(
		a.Records[directory],
		func(value helpers.IdentityTrustedIDP) bool { return value.ID == id },
	)
	if index < 0 {
		return nil, &sdkerrors.SDKError{StatusCode: 404}
	}
	a.Records[directory] = slices.Delete(a.Records[directory], index, index+1)
	a.Writes++
	a.Deletes++
	return &kkOps.DeleteTrustedIdpResponse{StatusCode: 204}, nil
}
