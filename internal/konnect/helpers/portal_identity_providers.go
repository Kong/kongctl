package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalIdentityProviderAPI defines the interface for portal identity provider operations.
type PortalIdentityProviderAPI interface {
	ListPortalIdentityProviders(
		ctx context.Context,
		request kkOps.GetPortalIdentityProvidersRequest,
		opts ...kkOps.Option,
	) (*kkOps.GetPortalIdentityProvidersResponse, error)
	GetPortalIdentityProvider(
		ctx context.Context,
		portalID string,
		id string,
		opts ...kkOps.Option,
	) (*kkOps.GetPortalIdentityProviderResponse, error)
	CreatePortalIdentityProvider(
		ctx context.Context,
		portalID string,
		request kkComps.CreateIdentityProvider,
		opts ...kkOps.Option,
	) (*kkOps.CreatePortalIdentityProviderResponse, error)
	UpdatePortalIdentityProvider(
		ctx context.Context,
		request kkOps.UpdatePortalIdentityProviderRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdatePortalIdentityProviderResponse, error)
	DeletePortalIdentityProvider(
		ctx context.Context,
		portalID string,
		id string,
		opts ...kkOps.Option,
	) (*kkOps.DeletePortalIdentityProviderResponse, error)
}

// PortalIdentityProviderAPIImpl provides an implementation backed by the SDK.
type PortalIdentityProviderAPIImpl struct {
	SDK *kkSDK.SDK
}

// ListPortalIdentityProviders lists identity providers for a portal.
func (p *PortalIdentityProviderAPIImpl) ListPortalIdentityProviders(
	ctx context.Context,
	request kkOps.GetPortalIdentityProvidersRequest,
	opts ...kkOps.Option,
) (*kkOps.GetPortalIdentityProvidersResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalAuthSettings.GetPortalIdentityProviders(ctx, request.PortalID, request.Filter, opts...)
}

// GetPortalIdentityProvider fetches a single identity provider for a portal.
func (p *PortalIdentityProviderAPIImpl) GetPortalIdentityProvider(
	ctx context.Context,
	portalID string,
	id string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalIdentityProviderResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalAuthSettings.GetPortalIdentityProvider(ctx, portalID, id, opts...)
}

// CreatePortalIdentityProvider creates a new identity provider for a portal.
func (p *PortalIdentityProviderAPIImpl) CreatePortalIdentityProvider(
	ctx context.Context,
	portalID string,
	request kkComps.CreateIdentityProvider,
	opts ...kkOps.Option,
) (*kkOps.CreatePortalIdentityProviderResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	portalRequest, err := createPortalIdentityProviderRequest(request)
	if err != nil {
		return nil, err
	}
	return p.SDK.PortalAuthSettings.CreatePortalIdentityProvider(ctx, portalID, portalRequest, opts...)
}

// UpdatePortalIdentityProvider updates an identity provider for a portal.
func (p *PortalIdentityProviderAPIImpl) UpdatePortalIdentityProvider(
	ctx context.Context,
	request kkOps.UpdatePortalIdentityProviderRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdatePortalIdentityProviderResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalAuthSettings.UpdatePortalIdentityProvider(ctx, request, opts...)
}

// DeletePortalIdentityProvider deletes an identity provider from a portal.
func (p *PortalIdentityProviderAPIImpl) DeletePortalIdentityProvider(
	ctx context.Context,
	portalID string,
	id string,
	opts ...kkOps.Option,
) (*kkOps.DeletePortalIdentityProviderResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalAuthSettings.DeletePortalIdentityProvider(ctx, portalID, id, opts...)
}

func createPortalIdentityProviderRequest(
	request kkComps.CreateIdentityProvider,
) (kkComps.PortalCreateIdentityProvider, error) {
	converted := kkComps.PortalCreateIdentityProvider{
		Enabled: request.Enabled,
		Type:    request.Type,
	}
	if request.Config == nil {
		return converted, nil
	}

	config, err := createPortalIdentityProviderConfig(request.Config)
	if err != nil {
		return kkComps.PortalCreateIdentityProvider{}, err
	}
	converted.Config = config
	return converted, nil
}

func createPortalIdentityProviderConfig(
	config *kkComps.CreateIdentityProviderConfig,
) (*kkComps.PortalCreateIdentityProviderConfig, error) {
	switch config.Type {
	case kkComps.CreateIdentityProviderConfigTypeOIDCIdentityProviderConfig:
		if config.OIDCIdentityProviderConfig == nil {
			return nil, fmt.Errorf("oidc identity provider config is required")
		}
		converted := kkComps.CreatePortalCreateIdentityProviderConfigOIDCIdentityProviderConfig(
			*config.OIDCIdentityProviderConfig,
		)
		return &converted, nil
	case kkComps.CreateIdentityProviderConfigTypeSAMLIdentityProviderConfigInput:
		if config.SAMLIdentityProviderConfigInput == nil {
			return nil, fmt.Errorf("saml identity provider config is required")
		}
		convertedSAML := kkComps.PortalSAMLIdentityProviderConfigInput{
			IdpMetadataURL: config.SAMLIdentityProviderConfigInput.IdpMetadataURL,
			IdpMetadataXML: config.SAMLIdentityProviderConfigInput.IdpMetadataXML,
		}
		converted := kkComps.CreatePortalCreateIdentityProviderConfigPortalSAMLIdentityProviderConfigInput(
			convertedSAML,
		)
		return &converted, nil
	default:
		return nil, fmt.Errorf("identity provider config type is required")
	}
}

var _ PortalIdentityProviderAPI = (*PortalIdentityProviderAPIImpl)(nil)
