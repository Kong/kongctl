package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"

	"github.com/kong/kongctl/internal/konnect/apiutil"
)

// AIGatewayModelAPI defines the interface for AI Gateway model operations needed by kongctl.
type AIGatewayModelAPI interface {
	ListAiGatewayModels(
		ctx context.Context,
		request kkOps.ListAiGatewayModelsRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayModelsResponse, error)
	CreateAiGatewayModel(
		ctx context.Context,
		gatewayID string,
		request kkComps.CreateAIGatewayModelRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayModelResponse, error)
	GetAiGatewayModel(
		ctx context.Context,
		gatewayID string,
		modelID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayModelResponse, error)
	UpdateAiGatewayModel(
		ctx context.Context,
		request kkOps.UpdateAiGatewayModelRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayModelResponse, error)
	DeleteAiGatewayModel(
		ctx context.Context,
		gatewayID string,
		modelID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayModelResponse, error)
}

// AIGatewayModelAPIImpl provides the real SDK implementation.
type AIGatewayModelAPIImpl struct {
	SDK         *kkSDK.SDK
	BaseURL     string
	Token       string
	TokenSource apiutil.TokenSource
	HTTPClient  kkSDK.HTTPClient
}

func (a *AIGatewayModelAPIImpl) ListAiGatewayModels(
	ctx context.Context,
	request kkOps.ListAiGatewayModelsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayModelsResponse, error) {
	return a.SDK.AIGatewayModels.ListAiGatewayModels(ctx, request, opts...)
}

func (a *AIGatewayModelAPIImpl) CreateAiGatewayModel(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateAIGatewayModelRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayModelResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	if a.BaseURL == "" || a.HTTPClient == nil {
		return a.SDK.AIGatewayModels.CreateAiGatewayModel(ctx, gatewayID, request, opts...)
	}

	sdk, err := a.modelCompatibilitySDK()
	if err != nil {
		return nil, err
	}
	return sdk.AIGatewayModels.CreateAiGatewayModel(ctx, gatewayID, request, opts...)
}

func (a *AIGatewayModelAPIImpl) GetAiGatewayModel(
	ctx context.Context,
	gatewayID string,
	modelID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayModelResponse, error) {
	return a.SDK.AIGatewayModels.GetAiGatewayModel(ctx, gatewayID, modelID, opts...)
}

func (a *AIGatewayModelAPIImpl) UpdateAiGatewayModel(
	ctx context.Context,
	request kkOps.UpdateAiGatewayModelRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayModelResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	if a.BaseURL == "" || a.HTTPClient == nil {
		return a.SDK.AIGatewayModels.UpdateAiGatewayModel(ctx, request, opts...)
	}

	sdk, err := a.modelCompatibilitySDK()
	if err != nil {
		return nil, err
	}
	return sdk.AIGatewayModels.UpdateAiGatewayModel(ctx, request, opts...)
}

func (a *AIGatewayModelAPIImpl) DeleteAiGatewayModel(
	ctx context.Context,
	gatewayID string,
	modelID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayModelResponse, error) {
	return a.SDK.AIGatewayModels.DeleteAiGatewayModel(ctx, gatewayID, modelID, opts...)
}

func (a *AIGatewayModelAPIImpl) modelCompatibilitySDK() (*kkSDK.SDK, error) {
	sdkOpts := []kkSDK.SDKOption{
		kkSDK.WithServerURL(a.BaseURL),
		kkSDK.WithClient(a.HTTPClient),
	}
	if a.TokenSource != nil {
		sdkOpts = append(sdkOpts, kkSDK.WithSecuritySource(func(ctx context.Context) (kkComps.Security, error) {
			token, err := a.TokenSource.Token(ctx)
			if err != nil {
				return kkComps.Security{}, err
			}
			return kkComps.Security{PersonalAccessToken: &token}, nil
		}))
	} else {
		sdkOpts = append(sdkOpts, kkSDK.WithSecurity(kkComps.Security{
			PersonalAccessToken: &a.Token,
		}))
	}

	sdk := kkSDK.New(sdkOpts...)
	if sdk == nil || sdk.AIGatewayModels == nil {
		return nil, fmt.Errorf("failed to initialize SDK for AI Gateway model request")
	}
	return sdk, nil
}
