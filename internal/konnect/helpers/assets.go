package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AssetsAPI defines the interface for operations on Portal Assets
type AssetsAPI interface {
	// Logo operations
	GetPortalAssetLogo(ctx context.Context, portalID string,
		opts ...kkOps.Option) (*kkOps.GetPortalAssetLogoResponse, error)
	GetPortalAssetLogoRaw(ctx context.Context, portalID string,
		opts ...kkOps.Option) (*kkOps.GetPortalAssetLogoRawResponse, error)
	ReplacePortalAssetLogo(ctx context.Context, portalID string, request *kkComponents.ReplacePortalImageAsset,
		opts ...kkOps.Option) (*kkOps.ReplacePortalAssetLogoResponse, error)

	// Favicon operations
	GetPortalAssetFavicon(ctx context.Context, portalID string,
		opts ...kkOps.Option) (*kkOps.GetPortalAssetFaviconResponse, error)
	GetPortalAssetFaviconRaw(ctx context.Context, portalID string,
		opts ...kkOps.Option) (*kkOps.GetPortalAssetFaviconRawResponse, error)
	ReplacePortalAssetFavicon(ctx context.Context, portalID string, request *kkComponents.ReplacePortalImageAsset,
		opts ...kkOps.Option) (*kkOps.ReplacePortalAssetFaviconResponse, error)
}

// AssetsAPIImpl provides an implementation of the AssetsAPI interface
type AssetsAPIImpl struct {
	SDK *kkSDK.SDK
}

// GetPortalAssetLogo implements the AssetsAPI interface
func (a *AssetsAPIImpl) GetPortalAssetLogo(
	ctx context.Context, portalID string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalAssetLogoResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return a.SDK.Assets.GetPortalAssetLogo(ctx, portalID, opts...)
}

// GetPortalAssetLogoRaw implements the AssetsAPI interface
func (a *AssetsAPIImpl) GetPortalAssetLogoRaw(
	ctx context.Context, portalID string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalAssetLogoRawResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return a.SDK.Assets.GetPortalAssetLogoRaw(ctx, portalID, opts...)
}

// ReplacePortalAssetLogo implements the AssetsAPI interface
func (a *AssetsAPIImpl) ReplacePortalAssetLogo(
	ctx context.Context, portalID string, request *kkComponents.ReplacePortalImageAsset,
	opts ...kkOps.Option,
) (*kkOps.ReplacePortalAssetLogoResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return a.SDK.Assets.ReplacePortalAssetLogo(ctx, portalID, request, opts...)
}

// GetPortalAssetFavicon implements the AssetsAPI interface
func (a *AssetsAPIImpl) GetPortalAssetFavicon(
	ctx context.Context, portalID string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalAssetFaviconResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return a.SDK.Assets.GetPortalAssetFavicon(ctx, portalID, opts...)
}

// GetPortalAssetFaviconRaw implements the AssetsAPI interface
func (a *AssetsAPIImpl) GetPortalAssetFaviconRaw(
	ctx context.Context, portalID string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalAssetFaviconRawResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return a.SDK.Assets.GetPortalAssetFaviconRaw(ctx, portalID, opts...)
}

// ReplacePortalAssetFavicon implements the AssetsAPI interface
func (a *AssetsAPIImpl) ReplacePortalAssetFavicon(
	ctx context.Context, portalID string, request *kkComponents.ReplacePortalImageAsset,
	opts ...kkOps.Option,
) (*kkOps.ReplacePortalAssetFaviconResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return a.SDK.Assets.ReplacePortalAssetFavicon(ctx, portalID, request, opts...)
}
