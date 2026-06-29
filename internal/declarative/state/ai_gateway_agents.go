package state

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util/pagination"
)

// ListAIGatewayAgents lists all Agents for an AI Gateway.
func (c *Client) ListAIGatewayAgents(ctx context.Context, gatewayID string) ([]AIGatewayAgent, error) {
	if err := ValidateAPIClient(c.aiGatewayAgentsAPI, "AI Gateway Agents API"); err != nil {
		return nil, err
	}

	var allData []kkComps.AIGatewayAgent
	var pageAfter *string
	pageSize := int64(100)

	for {
		req := kkOps.ListAiGatewayAgentsRequest{
			GatewayID: gatewayID,
			PageSize:  &pageSize,
			PageAfter: pageAfter,
		}

		resp, err := c.aiGatewayAgentsAPI.ListAiGatewayAgents(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway Agents", nil)
		}

		if resp == nil || resp.ListAIGatewayAgentsResponse == nil {
			return []AIGatewayAgent{}, nil
		}

		allData = append(allData, resp.ListAIGatewayAgentsResponse.Data...)

		nextCursor := pagination.ExtractPageAfterCursor(resp.ListAIGatewayAgentsResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	agents := make([]AIGatewayAgent, 0, len(allData))
	for _, agent := range allData {
		agents = append(agents, AIGatewayAgent{
			AIGatewayAgent:   agent,
			NormalizedLabels: normalizedAIGatewayAgentLabels(agent),
		})
	}
	return agents, nil
}

// GetAIGatewayAgent fetches an AI Gateway Agent by ID or name.
func (c *Client) GetAIGatewayAgent(
	ctx context.Context,
	gatewayID string,
	agentID string,
) (*AIGatewayAgent, error) {
	if err := ValidateAPIClient(c.aiGatewayAgentsAPI, "AI Gateway Agents API"); err != nil {
		return nil, err
	}

	resp, err := c.aiGatewayAgentsAPI.GetAiGatewayAgent(ctx, gatewayID, agentID)
	if err != nil {
		return nil, WrapAPIError(err, "get AI Gateway Agent by ID", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayAgent),
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayAgent == nil {
		return nil, nil
	}

	return &AIGatewayAgent{
		AIGatewayAgent:   *resp.AIGatewayAgent,
		NormalizedLabels: normalizedAIGatewayAgentLabels(*resp.AIGatewayAgent),
	}, nil
}

// GetAIGatewayAgentByName finds an AI Gateway Agent by name within a gateway.
func (c *Client) GetAIGatewayAgentByName(
	ctx context.Context,
	gatewayID string,
	name string,
) (*AIGatewayAgent, error) {
	agents, err := c.ListAIGatewayAgents(ctx, gatewayID)
	if err != nil {
		return nil, WrapAPIError(err, "list AI Gateway Agents to find by name", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayAgent),
			ResourceName: name,
			UseEnhanced:  true,
		})
	}

	for i := range agents {
		if agents[i].Name == name {
			return &agents[i], nil
		}
	}

	return nil, nil
}

// CreateAIGatewayAgent creates a new Agent under an AI Gateway.
func (c *Client) CreateAIGatewayAgent(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewayAgentRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayAgentsAPI, "AI Gateway Agents API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayAgentsAPI.CreateAiGatewayAgent(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create AI Gateway Agent", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayAgent),
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayAgent == nil {
		return "", fmt.Errorf("create AI Gateway Agent response missing data")
	}

	return resp.AIGatewayAgent.ID, nil
}

// UpdateAIGatewayAgent updates an existing Agent under an AI Gateway.
func (c *Client) UpdateAIGatewayAgent(
	ctx context.Context,
	gatewayID string,
	agentID string,
	req kkComps.UpdateAIGatewayAgentRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayAgentsAPI, "AI Gateway Agents API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayAgentsAPI.UpdateAiGatewayAgent(ctx, kkOps.UpdateAiGatewayAgentRequest{
		GatewayID:                   gatewayID,
		AgentID:                     agentID,
		UpdateAIGatewayAgentRequest: req,
	})
	if err != nil {
		return "", WrapAPIError(err, "update AI Gateway Agent", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayAgent),
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayAgent == nil {
		return "", fmt.Errorf("update AI Gateway Agent response missing data")
	}

	return resp.AIGatewayAgent.ID, nil
}

// DeleteAIGatewayAgent deletes an AI Gateway Agent by ID.
func (c *Client) DeleteAIGatewayAgent(ctx context.Context, gatewayID string, agentID string) error {
	if err := ValidateAPIClient(c.aiGatewayAgentsAPI, "AI Gateway Agents API"); err != nil {
		return err
	}

	_, err := c.aiGatewayAgentsAPI.DeleteAiGatewayAgent(ctx, gatewayID, agentID)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway Agent", nil)
	}

	return nil
}

func normalizedAIGatewayAgentLabels(agent kkComps.AIGatewayAgent) map[string]string {
	normalized := agent.Labels
	if normalized == nil {
		normalized = make(map[string]string)
	}
	return normalized
}
