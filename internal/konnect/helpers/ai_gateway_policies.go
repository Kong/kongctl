package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayPoliciesAPI defines the interface for AI Gateway Policy operations needed by kongctl.
type AIGatewayPoliciesAPI interface {
	ListAiGatewayPolicies(
		ctx context.Context,
		request kkOps.ListAiGatewayPoliciesRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayPoliciesResponse, error)
	CreateAiGatewayPolicy(
		ctx context.Context,
		gatewayID string,
		request kkComps.CreateAIGatewayPolicyRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayPolicyResponse, error)
	GetAiGatewayPolicy(
		ctx context.Context,
		gatewayID string,
		policyID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayPolicyResponse, error)
	UpdateAiGatewayPolicy(
		ctx context.Context,
		request kkOps.UpdateAiGatewayPolicyRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayPolicyResponse, error)
	DeleteAiGatewayPolicy(
		ctx context.Context,
		gatewayID string,
		policyID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayPolicyResponse, error)
}

// AIGatewayPoliciesAPIImpl provides the real SDK implementation.
type AIGatewayPoliciesAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayPoliciesAPIImpl) ListAiGatewayPolicies(
	ctx context.Context,
	request kkOps.ListAiGatewayPoliciesRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayPoliciesResponse, error) {
	return a.SDK.AIGatewayPolicies.ListAiGatewayPolicies(ctx, request, opts...)
}

func (a *AIGatewayPoliciesAPIImpl) CreateAiGatewayPolicy(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateAIGatewayPolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayPolicyResponse, error) {
	return a.SDK.AIGatewayPolicies.CreateAiGatewayPolicy(ctx, gatewayID, request, opts...)
}

func (a *AIGatewayPoliciesAPIImpl) GetAiGatewayPolicy(
	ctx context.Context,
	gatewayID string,
	policyID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayPolicyResponse, error) {
	return a.SDK.AIGatewayPolicies.GetAiGatewayPolicy(ctx, gatewayID, policyID, opts...)
}

func (a *AIGatewayPoliciesAPIImpl) UpdateAiGatewayPolicy(
	ctx context.Context,
	request kkOps.UpdateAiGatewayPolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayPolicyResponse, error) {
	return a.SDK.AIGatewayPolicies.UpdateAiGatewayPolicy(ctx, request, opts...)
}

func (a *AIGatewayPoliciesAPIImpl) DeleteAiGatewayPolicy(
	ctx context.Context,
	gatewayID string,
	policyID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayPolicyResponse, error) {
	return a.SDK.AIGatewayPolicies.DeleteAiGatewayPolicy(ctx, gatewayID, policyID, opts...)
}
