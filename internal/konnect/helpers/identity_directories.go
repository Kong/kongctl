package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

type IdentityDirectoryAPI interface {
	ListKongDirectories(
		ctx context.Context,
		page *kkComps.CursorPageParameters,
		sort *string,
		opts ...kkOps.Option,
	) (*kkOps.ListKongDirectoriesResponse, error)
	CreateDirectory(
		ctx context.Context,
		request kkComps.CreateDirectoryBody,
		opts ...kkOps.Option,
	) (*kkOps.CreateDirectoryResponse, error)
	ReplaceDirectory(
		ctx context.Context,
		directoryID string,
		request kkComps.ReplaceDirectoryBody,
		opts ...kkOps.Option,
	) (*kkOps.ReplaceDirectoryResponse, error)
	GetDirectory(
		ctx context.Context,
		directoryID string,
		opts ...kkOps.Option,
	) (*kkOps.GetDirectoryResponse, error)
	DeleteDirectory(
		ctx context.Context,
		directoryID string,
		forceDestroy *kkOps.DeleteDirectoryQueryParamForce,
		opts ...kkOps.Option,
	) (*kkOps.DeleteDirectoryResponse, error)
	GetRealmConfig(
		ctx context.Context,
		directoryID string,
		opts ...kkOps.Option,
	) (*kkOps.GetRealmConfigResponse, error)
}

type IdentityDirectoryAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *IdentityDirectoryAPIImpl) ListKongDirectories(
	ctx context.Context,
	page *kkComps.CursorPageParameters,
	sort *string,
	opts ...kkOps.Option,
) (*kkOps.ListKongDirectoriesResponse, error) {
	return a.SDK.KongIdentityDirectories.ListKongDirectories(ctx, page, sort, opts...)
}

func (a *IdentityDirectoryAPIImpl) CreateDirectory(
	ctx context.Context,
	request kkComps.CreateDirectoryBody,
	opts ...kkOps.Option,
) (*kkOps.CreateDirectoryResponse, error) {
	return a.SDK.KongIdentityDirectories.CreateDirectory(ctx, request, opts...)
}

func (a *IdentityDirectoryAPIImpl) ReplaceDirectory(
	ctx context.Context,
	directoryID string,
	request kkComps.ReplaceDirectoryBody,
	opts ...kkOps.Option,
) (*kkOps.ReplaceDirectoryResponse, error) {
	return a.SDK.KongIdentityDirectories.ReplaceDirectory(ctx, directoryID, request, opts...)
}

func (a *IdentityDirectoryAPIImpl) GetDirectory(
	ctx context.Context,
	directoryID string,
	opts ...kkOps.Option,
) (*kkOps.GetDirectoryResponse, error) {
	return a.SDK.KongIdentityDirectories.GetDirectory(ctx, directoryID, opts...)
}

func (a *IdentityDirectoryAPIImpl) DeleteDirectory(
	ctx context.Context,
	directoryID string,
	forceDestroy *kkOps.DeleteDirectoryQueryParamForce,
	opts ...kkOps.Option,
) (*kkOps.DeleteDirectoryResponse, error) {
	return a.SDK.KongIdentityDirectories.DeleteDirectory(ctx, directoryID, forceDestroy, opts...)
}

func (a *IdentityDirectoryAPIImpl) GetRealmConfig(
	ctx context.Context,
	directoryID string,
	opts ...kkOps.Option,
) (*kkOps.GetRealmConfigResponse, error) {
	return a.SDK.KongIdentityDirectories.GetRealmConfig(ctx, directoryID, opts...)
}
