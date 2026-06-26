package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayMCPServersAPI defines the interface for AI Gateway MCP Server operations needed by kongctl.
type AIGatewayMCPServersAPI interface {
	ListAiGatewayMcpServers(
		ctx context.Context,
		request kkOps.ListAiGatewayMcpServersRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayMcpServersResponse, error)
	CreateAiGatewayMcpServer(
		ctx context.Context,
		gatewayID string,
		request kkComps.CreateAIGatewayMCPServerRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayMcpServerResponse, error)
	GetAiGatewayMcpServer(
		ctx context.Context,
		gatewayID string,
		mcpServerID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayMcpServerResponse, error)
	UpdateAiGatewayMcpServer(
		ctx context.Context,
		request kkOps.UpdateAiGatewayMcpServerRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayMcpServerResponse, error)
	DeleteAiGatewayMcpServer(
		ctx context.Context,
		gatewayID string,
		mcpServerID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayMcpServerResponse, error)
}

// AIGatewayMCPServersAPIImpl provides the real SDK implementation.
type AIGatewayMCPServersAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayMCPServersAPIImpl) ListAiGatewayMcpServers(
	ctx context.Context,
	request kkOps.ListAiGatewayMcpServersRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayMcpServersResponse, error) {
	return a.SDK.AIGatewayMCPServers.ListAiGatewayMcpServers(ctx, request, opts...)
}

func (a *AIGatewayMCPServersAPIImpl) CreateAiGatewayMcpServer(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateAIGatewayMCPServerRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayMcpServerResponse, error) {
	return a.SDK.AIGatewayMCPServers.CreateAiGatewayMcpServer(ctx, gatewayID, request, opts...)
}

func (a *AIGatewayMCPServersAPIImpl) GetAiGatewayMcpServer(
	ctx context.Context,
	gatewayID string,
	mcpServerID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayMcpServerResponse, error) {
	return a.SDK.AIGatewayMCPServers.GetAiGatewayMcpServer(ctx, gatewayID, mcpServerID, opts...)
}

func (a *AIGatewayMCPServersAPIImpl) UpdateAiGatewayMcpServer(
	ctx context.Context,
	request kkOps.UpdateAiGatewayMcpServerRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayMcpServerResponse, error) {
	return a.SDK.AIGatewayMCPServers.UpdateAiGatewayMcpServer(ctx, request, opts...)
}

func (a *AIGatewayMCPServersAPIImpl) DeleteAiGatewayMcpServer(
	ctx context.Context,
	gatewayID string,
	mcpServerID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayMcpServerResponse, error) {
	return a.SDK.AIGatewayMCPServers.DeleteAiGatewayMcpServer(ctx, gatewayID, mcpServerID, opts...)
}
