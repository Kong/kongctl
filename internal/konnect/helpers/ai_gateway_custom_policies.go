package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayCustomPoliciesAPI defines the interface for AI Gateway Custom Policy operations needed by kongctl.
type AIGatewayCustomPoliciesAPI interface {
	ListAiGatewayCustomPolicies(
		ctx context.Context,
		request kkOps.ListAiGatewayCustomPoliciesRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayCustomPoliciesResponse, error)
	CreateAiGatewayCustomPolicy(
		ctx context.Context,
		gatewayID string,
		request kkComps.CreateAIGatewayCustomPolicyRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayCustomPolicyResponse, error)
	GetAiGatewayCustomPolicy(
		ctx context.Context,
		gatewayID string,
		policyID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayCustomPolicyResponse, error)
	UpdateAiGatewayCustomPolicy(
		ctx context.Context,
		request kkOps.UpdateAiGatewayCustomPolicyRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayCustomPolicyResponse, error)
	DeleteAiGatewayCustomPolicy(
		ctx context.Context,
		gatewayID string,
		policyID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayCustomPolicyResponse, error)
}

// AIGatewayCustomPoliciesAPIImpl provides the real SDK implementation.
type AIGatewayCustomPoliciesAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayCustomPoliciesAPIImpl) ListAiGatewayCustomPolicies(
	ctx context.Context,
	request kkOps.ListAiGatewayCustomPoliciesRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayCustomPoliciesResponse, error) {
	return a.SDK.AIGatewayCustomPolicies.ListAiGatewayCustomPolicies(ctx, request, opts...)
}

func (a *AIGatewayCustomPoliciesAPIImpl) CreateAiGatewayCustomPolicy(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateAIGatewayCustomPolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayCustomPolicyResponse, error) {
	return a.SDK.AIGatewayCustomPolicies.CreateAiGatewayCustomPolicy(ctx, gatewayID, request, opts...)
}

func (a *AIGatewayCustomPoliciesAPIImpl) GetAiGatewayCustomPolicy(
	ctx context.Context,
	gatewayID string,
	policyID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayCustomPolicyResponse, error) {
	return a.SDK.AIGatewayCustomPolicies.GetAiGatewayCustomPolicy(ctx, gatewayID, policyID, opts...)
}

func (a *AIGatewayCustomPoliciesAPIImpl) UpdateAiGatewayCustomPolicy(
	ctx context.Context,
	request kkOps.UpdateAiGatewayCustomPolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayCustomPolicyResponse, error) {
	return a.SDK.AIGatewayCustomPolicies.UpdateAiGatewayCustomPolicy(ctx, request, opts...)
}

func (a *AIGatewayCustomPoliciesAPIImpl) DeleteAiGatewayCustomPolicy(
	ctx context.Context,
	gatewayID string,
	policyID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayCustomPolicyResponse, error) {
	return a.SDK.AIGatewayCustomPolicies.DeleteAiGatewayCustomPolicy(ctx, gatewayID, policyID, opts...)
}
