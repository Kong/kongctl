package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

type IdentityPrincipalAPI interface {
	ListPrincipals(
		ctx context.Context,
		request kkOps.ListPrincipalsRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListPrincipalsResponse, error)
	GetPrincipal(
		ctx context.Context,
		directoryID string,
		principalID string,
		opts ...kkOps.Option,
	) (*kkOps.GetPrincipalResponse, error)
}

type IdentityPrincipalAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *IdentityPrincipalAPIImpl) ListPrincipals(
	ctx context.Context,
	request kkOps.ListPrincipalsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListPrincipalsResponse, error) {
	return a.SDK.KongPrincipal.ListPrincipals(ctx, request, opts...)
}

func (a *IdentityPrincipalAPIImpl) GetPrincipal(
	ctx context.Context,
	directoryID string,
	principalID string,
	opts ...kkOps.Option,
) (*kkOps.GetPrincipalResponse, error) {
	return a.SDK.KongPrincipal.GetPrincipal(ctx, directoryID, principalID, opts...)
}

type IdentityPrincipalIdentityAPI interface {
	ListIdentities(
		ctx context.Context,
		request kkOps.ListIdentitiesRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListIdentitiesResponse, error)
	GetIdentity(
		ctx context.Context,
		request kkOps.GetIdentityRequest,
		opts ...kkOps.Option,
	) (*kkOps.GetIdentityResponse, error)
}

type IdentityPrincipalIdentityAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *IdentityPrincipalIdentityAPIImpl) ListIdentities(
	ctx context.Context,
	request kkOps.ListIdentitiesRequest,
	opts ...kkOps.Option,
) (*kkOps.ListIdentitiesResponse, error) {
	return a.SDK.KongPrincipalIdentity.ListIdentities(ctx, request, opts...)
}

func (a *IdentityPrincipalIdentityAPIImpl) GetIdentity(
	ctx context.Context,
	request kkOps.GetIdentityRequest,
	opts ...kkOps.Option,
) (*kkOps.GetIdentityResponse, error) {
	return a.SDK.KongPrincipalIdentity.GetIdentity(ctx, request, opts...)
}
