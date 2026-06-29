package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayNodesAPI defines the interface for AI Gateway Node operations needed by kongctl.
type AIGatewayNodesAPI interface {
	ListAiGatewayNodes(
		ctx context.Context,
		request kkOps.ListAiGatewayNodesRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayNodesResponse, error)
	GetAiGatewayNode(
		ctx context.Context,
		gatewayID string,
		dataPlaneNodeID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayNodeResponse, error)
}

// AIGatewayNodesAPIImpl provides the real SDK implementation.
type AIGatewayNodesAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayNodesAPIImpl) ListAiGatewayNodes(
	ctx context.Context,
	request kkOps.ListAiGatewayNodesRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayNodesResponse, error) {
	return a.SDK.AIGatewayNodes.ListAiGatewayNodes(ctx, request, opts...)
}

func (a *AIGatewayNodesAPIImpl) GetAiGatewayNode(
	ctx context.Context,
	gatewayID string,
	dataPlaneNodeID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayNodeResponse, error) {
	return a.SDK.AIGatewayNodes.GetAiGatewayNode(ctx, gatewayID, dataPlaneNodeID, opts...)
}
