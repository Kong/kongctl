package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOPS "github.com/Kong/sdk-konnect-go/models/operations"
)

type DCRProvidersAPI interface {
	ListDcrProviders(ctx context.Context, request kkOPS.ListDcrProvidersRequest,
		opts ...kkOPS.Option) (*kkOPS.ListDcrProvidersResponse, error)
	CreateDcrProvider(ctx context.Context,
		provider kkComps.CreateDcrProviderRequest) (*kkOPS.CreateDcrProviderResponse, error)
	UpdateDcrProvider(ctx context.Context, id string,
		provider kkComps.UpdateDcrProviderRequest) (*kkOPS.UpdateDcrProviderResponse, error)
	DeleteDcrProvider(ctx context.Context, id string) (*kkOPS.DeleteDcrProviderResponse, error)
}

// DCRProvidersAPIImpl provides an implementation of the DCRProvidersAPI interface
type DCRProvidersAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *DCRProvidersAPIImpl) ListDcrProviders(ctx context.Context,
	request kkOPS.ListDcrProvidersRequest,
	opts ...kkOPS.Option,
) (*kkOPS.ListDcrProvidersResponse, error) {
	return a.SDK.DCRProviders.ListDcrProviders(ctx, request, opts...)
}

func (a *DCRProvidersAPIImpl) CreateDcrProvider(ctx context.Context,
	provider kkComps.CreateDcrProviderRequest,
) (*kkOPS.CreateDcrProviderResponse, error) {
	return a.SDK.DCRProviders.CreateDcrProvider(ctx, provider)
}

func (a *DCRProvidersAPIImpl) UpdateDcrProvider(ctx context.Context, id string,
	provider kkComps.UpdateDcrProviderRequest,
) (*kkOPS.UpdateDcrProviderResponse, error) {
	return a.SDK.DCRProviders.UpdateDcrProvider(ctx, id, provider)
}

func (a *DCRProvidersAPIImpl) DeleteDcrProvider(ctx context.Context,
	id string,
) (*kkOPS.DeleteDcrProviderResponse, error) {
	return a.SDK.DCRProviders.DeleteDcrProvider(ctx, id)
}
