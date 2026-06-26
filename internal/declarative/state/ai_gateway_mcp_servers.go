package state

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util/pagination"
)

// ListAIGatewayMCPServers lists all MCP Servers for an AI Gateway.
func (c *Client) ListAIGatewayMCPServers(ctx context.Context, gatewayID string) ([]AIGatewayMCPServer, error) {
	if err := ValidateAPIClient(c.aiGatewayMCPServersAPI, "AI Gateway MCP Servers API"); err != nil {
		return nil, err
	}

	var allData []kkComps.AIGatewayMCPServer
	var pageAfter *string
	pageSize := int64(100)

	for {
		req := kkOps.ListAiGatewayMcpServersRequest{
			GatewayID: gatewayID,
			PageSize:  &pageSize,
			PageAfter: pageAfter,
		}

		resp, err := c.aiGatewayMCPServersAPI.ListAiGatewayMcpServers(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway MCP Servers", nil)
		}

		if resp == nil || resp.ListAIGatewayMCPServersResponse == nil {
			return []AIGatewayMCPServer{}, nil
		}

		allData = append(allData, resp.ListAIGatewayMCPServersResponse.Data...)

		nextCursor := pagination.ExtractPageAfterCursor(resp.ListAIGatewayMCPServersResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	servers := make([]AIGatewayMCPServer, 0, len(allData))
	for _, server := range allData {
		servers = append(servers, AIGatewayMCPServer{
			AIGatewayMCPServer: server,
			NormalizedLabels:   normalizedAIGatewayMCPServerLabels(server),
		})
	}
	return servers, nil
}

// GetAIGatewayMCPServer fetches an AI Gateway MCP Server by ID or name.
func (c *Client) GetAIGatewayMCPServer(
	ctx context.Context,
	gatewayID string,
	mcpServerID string,
) (*AIGatewayMCPServer, error) {
	if err := ValidateAPIClient(c.aiGatewayMCPServersAPI, "AI Gateway MCP Servers API"); err != nil {
		return nil, err
	}

	resp, err := c.aiGatewayMCPServersAPI.GetAiGatewayMcpServer(ctx, gatewayID, mcpServerID)
	if err != nil {
		return nil, WrapAPIError(err, "get AI Gateway MCP Server by ID", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayMCPServer),
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayMCPServer == nil {
		return nil, nil
	}

	return &AIGatewayMCPServer{
		AIGatewayMCPServer: *resp.AIGatewayMCPServer,
		NormalizedLabels:   normalizedAIGatewayMCPServerLabels(*resp.AIGatewayMCPServer),
	}, nil
}

// GetAIGatewayMCPServerByName finds an AI Gateway MCP Server by name within a gateway.
func (c *Client) GetAIGatewayMCPServerByName(
	ctx context.Context,
	gatewayID string,
	name string,
) (*AIGatewayMCPServer, error) {
	servers, err := c.ListAIGatewayMCPServers(ctx, gatewayID)
	if err != nil {
		return nil, WrapAPIError(err, "list AI Gateway MCP Servers to find by name", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayMCPServer),
			ResourceName: name,
			UseEnhanced:  true,
		})
	}

	for i := range servers {
		if resources.AIGatewayMCPServerName(servers[i].AIGatewayMCPServer) == name {
			return &servers[i], nil
		}
	}

	return nil, nil
}

// CreateAIGatewayMCPServer creates a new MCP Server under an AI Gateway.
func (c *Client) CreateAIGatewayMCPServer(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewayMCPServerRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayMCPServersAPI, "AI Gateway MCP Servers API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayMCPServersAPI.CreateAiGatewayMcpServer(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create AI Gateway MCP Server", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayMCPServer),
			ResourceName: aiGatewayMCPServerCreateRequestName(req),
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayMCPServer == nil {
		return "", fmt.Errorf("create AI Gateway MCP Server response missing data")
	}

	return resources.AIGatewayMCPServerID(*resp.AIGatewayMCPServer), nil
}

// UpdateAIGatewayMCPServer updates an existing MCP Server under an AI Gateway.
func (c *Client) UpdateAIGatewayMCPServer(
	ctx context.Context,
	gatewayID string,
	mcpServerID string,
	req kkComps.UpdateAIGatewayMCPServerRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayMCPServersAPI, "AI Gateway MCP Servers API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayMCPServersAPI.UpdateAiGatewayMcpServer(ctx, kkOps.UpdateAiGatewayMcpServerRequest{
		GatewayID:                       gatewayID,
		McpServerID:                     mcpServerID,
		UpdateAIGatewayMCPServerRequest: req,
	})
	if err != nil {
		return "", WrapAPIError(err, "update AI Gateway MCP Server", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayMCPServer),
			ResourceName: aiGatewayMCPServerUpdateRequestName(req),
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayMCPServer == nil {
		return "", fmt.Errorf("update AI Gateway MCP Server response missing data")
	}

	return resources.AIGatewayMCPServerID(*resp.AIGatewayMCPServer), nil
}

// DeleteAIGatewayMCPServer deletes an AI Gateway MCP Server by ID.
func (c *Client) DeleteAIGatewayMCPServer(ctx context.Context, gatewayID string, mcpServerID string) error {
	if err := ValidateAPIClient(c.aiGatewayMCPServersAPI, "AI Gateway MCP Servers API"); err != nil {
		return err
	}

	_, err := c.aiGatewayMCPServersAPI.DeleteAiGatewayMcpServer(ctx, gatewayID, mcpServerID)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway MCP Server", nil)
	}

	return nil
}

func normalizedAIGatewayMCPServerLabels(server kkComps.AIGatewayMCPServer) map[string]string {
	normalized := resources.AIGatewayMCPServerLabels(server)
	if normalized == nil {
		normalized = make(map[string]string)
	}
	return normalized
}

func aiGatewayMCPServerCreateRequestName(req kkComps.CreateAIGatewayMCPServerRequest) string {
	data, err := json.Marshal(req)
	if err != nil {
		return ""
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	return stringFromRaw(raw["name"])
}

func aiGatewayMCPServerUpdateRequestName(req kkComps.UpdateAIGatewayMCPServerRequest) string {
	data, err := json.Marshal(req)
	if err != nil {
		return ""
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	return stringFromRaw(raw["name"])
}
