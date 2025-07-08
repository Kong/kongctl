package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalSnippetAPI defines the interface for operations on Portal Snippets
type PortalSnippetAPI interface {
	// Portal Snippet operations
	CreatePortalSnippet(ctx context.Context, portalID string, request kkComponents.CreatePortalSnippetRequest,
		opts ...kkOps.Option) (*kkOps.CreatePortalSnippetResponse, error)
	UpdatePortalSnippet(ctx context.Context, request kkOps.UpdatePortalSnippetRequest,
		opts ...kkOps.Option) (*kkOps.UpdatePortalSnippetResponse, error)
	DeletePortalSnippet(ctx context.Context, portalID string, snippetID string,
		opts ...kkOps.Option) (*kkOps.DeletePortalSnippetResponse, error)
	ListPortalSnippets(ctx context.Context, request kkOps.ListPortalSnippetsRequest,
		opts ...kkOps.Option) (*kkOps.ListPortalSnippetsResponse, error)
	GetPortalSnippet(ctx context.Context, portalID string, snippetID string,
		opts ...kkOps.Option) (*kkOps.GetPortalSnippetResponse, error)
}

// PortalSnippetAPIImpl provides an implementation of the PortalSnippetAPI interface
type PortalSnippetAPIImpl struct {
	SDK *kkSDK.SDK
}

// CreatePortalSnippet implements the PortalSnippetAPI interface
func (p *PortalSnippetAPIImpl) CreatePortalSnippet(
	ctx context.Context, portalID string, request kkComponents.CreatePortalSnippetRequest,
	opts ...kkOps.Option,
) (*kkOps.CreatePortalSnippetResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.Snippets.CreatePortalSnippet(ctx, portalID, request, opts...)
}

// UpdatePortalSnippet implements the PortalSnippetAPI interface
func (p *PortalSnippetAPIImpl) UpdatePortalSnippet(
	ctx context.Context, request kkOps.UpdatePortalSnippetRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdatePortalSnippetResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.Snippets.UpdatePortalSnippet(ctx, request, opts...)
}

// DeletePortalSnippet implements the PortalSnippetAPI interface
func (p *PortalSnippetAPIImpl) DeletePortalSnippet(
	ctx context.Context, portalID string, snippetID string,
	opts ...kkOps.Option,
) (*kkOps.DeletePortalSnippetResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.Snippets.DeletePortalSnippet(ctx, portalID, snippetID, opts...)
}

// ListPortalSnippets implements the PortalSnippetAPI interface
func (p *PortalSnippetAPIImpl) ListPortalSnippets(
	ctx context.Context, request kkOps.ListPortalSnippetsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListPortalSnippetsResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.Snippets.ListPortalSnippets(ctx, request, opts...)
}

// GetPortalSnippet implements the PortalSnippetAPI interface
func (p *PortalSnippetAPIImpl) GetPortalSnippet(
	ctx context.Context, portalID string, snippetID string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalSnippetResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.Snippets.GetPortalSnippet(ctx, portalID, snippetID, opts...)
}