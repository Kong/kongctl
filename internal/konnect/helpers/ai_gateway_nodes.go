package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/konnect/apiutil"
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
	UpsertAiGatewayNode(
		ctx context.Context,
		gatewayID string,
		dataPlaneNodeID string,
		request map[string]any,
		opts ...kkOps.Option,
	) (*kkComps.AIGatewayDataPlaneNode, error)
	DeleteAiGatewayNode(
		ctx context.Context,
		gatewayID string,
		dataPlaneNodeID string,
		opts ...kkOps.Option,
	) error
}

// AIGatewayNodesAPIImpl provides the real SDK implementation.
type AIGatewayNodesAPIImpl struct {
	SDK         *kkSDK.SDK
	BaseURL     string
	Token       string
	TokenSource apiutil.TokenSource
	HTTPClient  kkSDK.HTTPClient
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

func (a *AIGatewayNodesAPIImpl) UpsertAiGatewayNode(
	ctx context.Context,
	gatewayID string,
	dataPlaneNodeID string,
	request map[string]any,
	_ ...kkOps.Option,
) (*kkComps.AIGatewayDataPlaneNode, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to encode AI Gateway Node request: %w", err)
	}

	result, err := a.request(ctx, http.MethodPut, aiGatewayNodePath(gatewayID, dataPlaneNodeID), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf(
			"AI Gateway Node upsert failed with status %d: %s",
			result.StatusCode,
			strings.TrimSpace(string(result.Body)),
		)
	}

	if len(bytes.TrimSpace(result.Body)) == 0 {
		return &kkComps.AIGatewayDataPlaneNode{ID: dataPlaneNodeID}, nil
	}

	var node kkComps.AIGatewayDataPlaneNode
	if err := json.Unmarshal(result.Body, &node); err != nil {
		return nil, fmt.Errorf("failed to decode AI Gateway Node response: %w", err)
	}
	if node.ID == "" {
		node.ID = dataPlaneNodeID
	}
	return &node, nil
}

func (a *AIGatewayNodesAPIImpl) DeleteAiGatewayNode(
	ctx context.Context,
	gatewayID string,
	dataPlaneNodeID string,
	_ ...kkOps.Option,
) error {
	result, err := a.request(ctx, http.MethodDelete, aiGatewayNodePath(gatewayID, dataPlaneNodeID), nil)
	if err != nil {
		return err
	}
	if result.StatusCode == http.StatusOK ||
		result.StatusCode == http.StatusAccepted ||
		result.StatusCode == http.StatusNoContent {
		return nil
	}
	return fmt.Errorf(
		"AI Gateway Node delete failed with status %d: %s",
		result.StatusCode,
		strings.TrimSpace(string(result.Body)),
	)
}

func (a *AIGatewayNodesAPIImpl) request(
	ctx context.Context,
	method string,
	path string,
	body io.Reader,
) (*apiutil.Result, error) {
	headers := map[string]string{
		httpHeaderContentType: contentTypeJSON,
	}
	client := a.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	if a.TokenSource != nil {
		return apiutil.RequestWithTokenSource(ctx, client, method, a.BaseURL, path, a.TokenSource, headers, body)
	}
	return apiutil.Request(ctx, client, method, a.BaseURL, path, a.Token, headers, body)
}

func aiGatewayNodePath(gatewayID string, dataPlaneNodeID string) string {
	return fmt.Sprintf(
		"/v1/ai-gateways/%s/nodes/%s",
		url.PathEscape(gatewayID),
		url.PathEscape(dataPlaneNodeID),
	)
}
