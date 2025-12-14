package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalEmailsAPI exposes portal email operations used by the CLI.
type PortalEmailsAPI interface {
	ListEmailDomains(ctx context.Context, request kkOps.ListEmailDomainsRequest,
		opts ...kkOps.Option) (*kkOps.ListEmailDomainsResponse, error)
	GetEmailDomain(ctx context.Context, emailDomain string,
		opts ...kkOps.Option) (*kkOps.GetEmailDomainResponse, error)
	GetEmailConfig(ctx context.Context, portalID string,
		opts ...kkOps.Option) (*kkOps.GetEmailConfigResponse, error)
	CreatePortalEmailConfig(ctx context.Context, portalID string, body kkComps.PostPortalEmailConfig,
		opts ...kkOps.Option) (*kkOps.CreatePortalEmailConfigResponse, error)
	UpdatePortalEmailConfig(ctx context.Context, portalID string, body *kkComps.PatchPortalEmailConfig,
		opts ...kkOps.Option) (*kkOps.UpdatePortalEmailConfigResponse, error)
	DeletePortalEmailConfig(ctx context.Context, portalID string,
		opts ...kkOps.Option) (*kkOps.DeletePortalEmailConfigResponse, error)
}

// PortalEmailsAPIImpl provides a concrete implementation backed by the SDK.
type PortalEmailsAPIImpl struct {
	SDK *kkSDK.SDK
}

// ListEmailDomains delegates to the generated SDK.
func (p *PortalEmailsAPIImpl) ListEmailDomains(
	ctx context.Context, request kkOps.ListEmailDomainsRequest, opts ...kkOps.Option,
) (*kkOps.ListEmailDomainsResponse, error) {
	return p.SDK.PortalEmails.ListEmailDomains(ctx, request, opts...)
}

// GetEmailDomain delegates to the generated SDK.
func (p *PortalEmailsAPIImpl) GetEmailDomain(
	ctx context.Context, emailDomain string, opts ...kkOps.Option,
) (*kkOps.GetEmailDomainResponse, error) {
	return p.SDK.PortalEmails.GetEmailDomain(ctx, emailDomain, opts...)
}

// GetEmailConfig delegates to the generated SDK.
func (p *PortalEmailsAPIImpl) GetEmailConfig(
	ctx context.Context, portalID string, opts ...kkOps.Option,
) (*kkOps.GetEmailConfigResponse, error) {
	return p.SDK.PortalEmails.GetEmailConfig(ctx, portalID, opts...)
}

// CreatePortalEmailConfig delegates to the generated SDK.
func (p *PortalEmailsAPIImpl) CreatePortalEmailConfig(
	ctx context.Context, portalID string, body kkComps.PostPortalEmailConfig, opts ...kkOps.Option,
) (*kkOps.CreatePortalEmailConfigResponse, error) {
	return p.SDK.PortalEmails.CreatePortalEmailConfig(ctx, portalID, body, opts...)
}

// UpdatePortalEmailConfig delegates to the generated SDK.
func (p *PortalEmailsAPIImpl) UpdatePortalEmailConfig(
	ctx context.Context, portalID string, body *kkComps.PatchPortalEmailConfig, opts ...kkOps.Option,
) (*kkOps.UpdatePortalEmailConfigResponse, error) {
	return p.SDK.PortalEmails.UpdatePortalEmailConfig(ctx, portalID, body, opts...)
}

// DeletePortalEmailConfig delegates to the generated SDK.
func (p *PortalEmailsAPIImpl) DeletePortalEmailConfig(
	ctx context.Context, portalID string, opts ...kkOps.Option,
) (*kkOps.DeletePortalEmailConfigResponse, error) {
	return p.SDK.PortalEmails.DeletePortalEmailConfig(ctx, portalID, opts...)
}
