package state

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util/pagination"
)

// ListAIGatewayNodes lists all data plane nodes for an AI Gateway.
func (c *Client) ListAIGatewayNodes(ctx context.Context, gatewayID string) ([]AIGatewayNode, error) {
	if err := ValidateAPIClient(c.aiGatewayNodesAPI, "AI Gateway Nodes API"); err != nil {
		return nil, err
	}

	var allData []kkComps.AIGatewayDataPlaneNode
	var pageAfter *string
	pageSize := int64(100)

	for {
		req := kkOps.ListAiGatewayNodesRequest{
			GatewayID: gatewayID,
			PageSize:  &pageSize,
			PageAfter: pageAfter,
		}

		resp, err := c.aiGatewayNodesAPI.ListAiGatewayNodes(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway Nodes", nil)
		}

		if resp == nil || resp.ListAIGatewayDataPlaneNodesResponse == nil {
			return []AIGatewayNode{}, nil
		}

		allData = append(allData, resp.ListAIGatewayDataPlaneNodesResponse.Data...)

		nextCursor := pagination.ExtractPageAfterCursor(resp.ListAIGatewayDataPlaneNodesResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	nodes := make([]AIGatewayNode, 0, len(allData))
	for _, node := range allData {
		nodes = append(nodes, AIGatewayNode{
			AIGatewayDataPlaneNode: node,
			NormalizedLabels:       map[string]string{},
		})
	}
	return nodes, nil
}

// GetAIGatewayNode fetches an AI Gateway data plane node by ID.
func (c *Client) GetAIGatewayNode(
	ctx context.Context,
	gatewayID string,
	dataPlaneNodeID string,
) (*AIGatewayNode, error) {
	if err := ValidateAPIClient(c.aiGatewayNodesAPI, "AI Gateway Nodes API"); err != nil {
		return nil, err
	}

	resp, err := c.aiGatewayNodesAPI.GetAiGatewayNode(ctx, gatewayID, dataPlaneNodeID)
	if err != nil {
		return nil, WrapAPIError(err, "get AI Gateway Node by ID", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayNode),
			ResourceName: dataPlaneNodeID,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayDataPlaneNode == nil {
		return nil, nil
	}

	return &AIGatewayNode{
		AIGatewayDataPlaneNode: *resp.AIGatewayDataPlaneNode,
		NormalizedLabels:       map[string]string{},
	}, nil
}

// UpsertAIGatewayNode creates or updates a data plane node under an AI Gateway.
func (c *Client) UpsertAIGatewayNode(
	ctx context.Context,
	gatewayID string,
	dataPlaneNodeID string,
	req AIGatewayNodeRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayNodesAPI, "AI Gateway Nodes API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayNodesAPI.UpsertAiGatewayNode(ctx, gatewayID, dataPlaneNodeID, req.Payload)
	if err != nil {
		return "", WrapAPIError(err, "upsert AI Gateway Node", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayNode),
			ResourceName: dataPlaneNodeID,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil {
		return "", fmt.Errorf("upsert AI Gateway Node response missing data")
	}
	if resp.ID != "" {
		return resp.ID, nil
	}
	return dataPlaneNodeID, nil
}

// DeleteAIGatewayNode deletes an AI Gateway data plane node by ID.
func (c *Client) DeleteAIGatewayNode(ctx context.Context, gatewayID string, dataPlaneNodeID string) error {
	if err := ValidateAPIClient(c.aiGatewayNodesAPI, "AI Gateway Nodes API"); err != nil {
		return err
	}

	if err := c.aiGatewayNodesAPI.DeleteAiGatewayNode(ctx, gatewayID, dataPlaneNodeID); err != nil {
		return WrapAPIError(err, "delete AI Gateway Node", nil)
	}

	return nil
}
