package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayAgentsAPI defines the interface for AI Gateway Agent operations needed by kongctl.
type AIGatewayAgentsAPI interface {
	ListAiGatewayAgents(
		ctx context.Context,
		request kkOps.ListAiGatewayAgentsRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayAgentsResponse, error)
	CreateAiGatewayAgent(
		ctx context.Context,
		gatewayID string,
		request kkComps.CreateAIGatewayAgentRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayAgentResponse, error)
	GetAiGatewayAgent(
		ctx context.Context,
		gatewayID string,
		agentID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayAgentResponse, error)
	UpdateAiGatewayAgent(
		ctx context.Context,
		request kkOps.UpdateAiGatewayAgentRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayAgentResponse, error)
	DeleteAiGatewayAgent(
		ctx context.Context,
		gatewayID string,
		agentID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayAgentResponse, error)
}

// AIGatewayAgentsAPIImpl provides the real SDK implementation.
type AIGatewayAgentsAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayAgentsAPIImpl) ListAiGatewayAgents(
	ctx context.Context,
	request kkOps.ListAiGatewayAgentsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayAgentsResponse, error) {
	return a.SDK.AIGatewayAgents.ListAiGatewayAgents(ctx, request, opts...)
}

func (a *AIGatewayAgentsAPIImpl) CreateAiGatewayAgent(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateAIGatewayAgentRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayAgentResponse, error) {
	return a.SDK.AIGatewayAgents.CreateAiGatewayAgent(ctx, gatewayID, request, opts...)
}

func (a *AIGatewayAgentsAPIImpl) GetAiGatewayAgent(
	ctx context.Context,
	gatewayID string,
	agentID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayAgentResponse, error) {
	return a.SDK.AIGatewayAgents.GetAiGatewayAgent(ctx, gatewayID, agentID, opts...)
}

func (a *AIGatewayAgentsAPIImpl) UpdateAiGatewayAgent(
	ctx context.Context,
	request kkOps.UpdateAiGatewayAgentRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayAgentResponse, error) {
	return a.SDK.AIGatewayAgents.UpdateAiGatewayAgent(ctx, request, opts...)
}

func (a *AIGatewayAgentsAPIImpl) DeleteAiGatewayAgent(
	ctx context.Context,
	gatewayID string,
	agentID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayAgentResponse, error) {
	return a.SDK.AIGatewayAgents.DeleteAiGatewayAgent(ctx, gatewayID, agentID, opts...)
}
