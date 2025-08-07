package adapters

import (
	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AdapterFactory creates concrete resolution adapter implementations
type AdapterFactory struct {
	client *state.Client
}

// NewAdapterFactory creates a new adapter factory
func NewAdapterFactory(client *state.Client) *AdapterFactory {
	return &AdapterFactory{client: client}
}

// CreatePortalAdapter creates a portal resolution adapter
func (f *AdapterFactory) CreatePortalAdapter() external.ResolutionAdapter {
	return NewPortalResolutionAdapter(f.client)
}

// CreateAPIAdapter creates an API resolution adapter
func (f *AdapterFactory) CreateAPIAdapter() external.ResolutionAdapter {
	return NewAPIResolutionAdapter(f.client)
}

// CreateControlPlaneAdapter creates a control plane resolution adapter
func (f *AdapterFactory) CreateControlPlaneAdapter() external.ResolutionAdapter {
	return NewControlPlaneResolutionAdapter(f.client)
}

// CreateApplicationAuthStrategyAdapter creates an application auth strategy resolution adapter
func (f *AdapterFactory) CreateApplicationAuthStrategyAdapter() external.ResolutionAdapter {
	return NewApplicationAuthStrategyResolutionAdapter(f.client)
}

// CreatePortalCustomizationAdapter creates a portal customization resolution adapter
func (f *AdapterFactory) CreatePortalCustomizationAdapter() external.ResolutionAdapter {
	return NewPortalCustomizationResolutionAdapter(f.client)
}

// CreatePortalCustomDomainAdapter creates a portal custom domain resolution adapter
func (f *AdapterFactory) CreatePortalCustomDomainAdapter() external.ResolutionAdapter {
	return NewPortalCustomDomainResolutionAdapter(f.client)
}

// CreatePortalPageAdapter creates a portal page resolution adapter
func (f *AdapterFactory) CreatePortalPageAdapter() external.ResolutionAdapter {
	return NewPortalPageResolutionAdapter(f.client)
}

// CreatePortalSnippetAdapter creates a portal snippet resolution adapter
func (f *AdapterFactory) CreatePortalSnippetAdapter() external.ResolutionAdapter {
	return NewPortalSnippetResolutionAdapter(f.client)
}

// CreateAPIVersionAdapter creates an API version resolution adapter
func (f *AdapterFactory) CreateAPIVersionAdapter() external.ResolutionAdapter {
	return NewAPIVersionResolutionAdapter(f.client)
}

// CreateAPIPublicationAdapter creates an API publication resolution adapter
func (f *AdapterFactory) CreateAPIPublicationAdapter() external.ResolutionAdapter {
	return NewAPIPublicationResolutionAdapter(f.client)
}

// CreateAPIImplementationAdapter creates an API implementation resolution adapter
func (f *AdapterFactory) CreateAPIImplementationAdapter() external.ResolutionAdapter {
	return NewAPIImplementationResolutionAdapter(f.client)
}

// CreateAPIDocumentAdapter creates an API document resolution adapter
func (f *AdapterFactory) CreateAPIDocumentAdapter() external.ResolutionAdapter {
	return NewAPIDocumentResolutionAdapter(f.client)
}

// CreateCEServiceAdapter creates a CE service (core entity) resolution adapter  
func (f *AdapterFactory) CreateCEServiceAdapter() external.ResolutionAdapter {
	return NewCEServiceResolutionAdapter(f.client)
}