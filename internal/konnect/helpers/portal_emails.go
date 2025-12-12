package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalEmailsAPI exposes portal email operations used by the CLI.
type PortalEmailsAPI interface {
	ListEmailDomains(ctx context.Context, request kkOps.ListEmailDomainsRequest,
		opts ...kkOps.Option) (*kkOps.ListEmailDomainsResponse, error)
	GetEmailDomain(ctx context.Context, emailDomain string,
		opts ...kkOps.Option) (*kkOps.GetEmailDomainResponse, error)
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
