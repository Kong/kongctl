package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayVaultsAPI defines the interface for AI Gateway Vault operations needed by kongctl.
type AIGatewayVaultsAPI interface {
	ListAiGatewayVaults(
		ctx context.Context,
		request kkOps.ListAiGatewayVaultsRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayVaultsResponse, error)
	CreateAiGatewayVault(
		ctx context.Context,
		gatewayID string,
		request kkComps.CreateAIGatewayVaultRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayVaultResponse, error)
	GetAiGatewayVault(
		ctx context.Context,
		gatewayID string,
		vaultID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayVaultResponse, error)
	UpdateAiGatewayVault(
		ctx context.Context,
		request kkOps.UpdateAiGatewayVaultRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayVaultResponse, error)
	DeleteAiGatewayVault(
		ctx context.Context,
		gatewayID string,
		vaultID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayVaultResponse, error)
}

// AIGatewayVaultsAPIImpl provides the real SDK implementation.
type AIGatewayVaultsAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayVaultsAPIImpl) ListAiGatewayVaults(
	ctx context.Context,
	request kkOps.ListAiGatewayVaultsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayVaultsResponse, error) {
	return a.SDK.AIGatewayVaults.ListAiGatewayVaults(ctx, request, opts...)
}

func (a *AIGatewayVaultsAPIImpl) CreateAiGatewayVault(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateAIGatewayVaultRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayVaultResponse, error) {
	return a.SDK.AIGatewayVaults.CreateAiGatewayVault(ctx, gatewayID, request, opts...)
}

func (a *AIGatewayVaultsAPIImpl) GetAiGatewayVault(
	ctx context.Context,
	gatewayID string,
	vaultID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayVaultResponse, error) {
	return a.SDK.AIGatewayVaults.GetAiGatewayVault(ctx, gatewayID, vaultID, opts...)
}

func (a *AIGatewayVaultsAPIImpl) UpdateAiGatewayVault(
	ctx context.Context,
	request kkOps.UpdateAiGatewayVaultRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayVaultResponse, error) {
	return a.SDK.AIGatewayVaults.UpdateAiGatewayVault(ctx, request, opts...)
}

func (a *AIGatewayVaultsAPIImpl) DeleteAiGatewayVault(
	ctx context.Context,
	gatewayID string,
	vaultID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayVaultResponse, error) {
	return a.SDK.AIGatewayVaults.DeleteAiGatewayVault(ctx, gatewayID, vaultID, opts...)
}
