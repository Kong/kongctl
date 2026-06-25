package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
		kkSDK.WithClient(&aiGatewayModelCompatibilityHTTPClient{base: a.HTTPClient}),
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

type aiGatewayModelCompatibilityHTTPClient struct {
	base kkSDK.HTTPClient
}

func (c *aiGatewayModelCompatibilityHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if c == nil || c.base == nil {
		return nil, fmt.Errorf("http client is not configured")
	}

	if err := addTargetsToAIGatewayModelRequest(req); err != nil {
		return nil, err
	}

	return c.base.Do(req)
}

func addTargetsToAIGatewayModelRequest(req *http.Request) error {
	if !isAIGatewayModelMutationRequest(req) {
		return nil
	}

	body, err := readAndRestoreRequestBody(req)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to decode AI Gateway model request: %w", err)
	}
	if _, ok := payload["targets"]; ok {
		return nil
	}

	targetModels, ok := payload["target_models"]
	if !ok || len(bytes.TrimSpace(targetModels)) == 0 || bytes.Equal(bytes.TrimSpace(targetModels), []byte("null")) {
		return nil
	}
	payload["targets"] = targetModels

	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode AI Gateway model request: %w", err)
	}
	restoreRequestBody(req, encoded)
	return nil
}

func isAIGatewayModelMutationRequest(req *http.Request) bool {
	if req == nil || req.URL == nil {
		return false
	}
	if req.Method != http.MethodPost && req.Method != http.MethodPut {
		return false
	}

	path := strings.Trim(req.URL.Path, "/")
	segments := strings.Split(path, "/")
	if len(segments) != 4 && len(segments) != 5 {
		return false
	}
	if segments[0] != "v1" || segments[1] != "ai-gateways" || segments[2] == "" || segments[3] != "models" {
		return false
	}
	if req.Method == http.MethodPost {
		return len(segments) == 4
	}
	return len(segments) == 5 && segments[4] != ""
}
