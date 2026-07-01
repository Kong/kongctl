package state

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util/pagination"
)

// ListAIGatewayConsumerGroups lists all Consumer Groups for an AI Gateway.
func (c *Client) ListAIGatewayConsumerGroups(ctx context.Context, gatewayID string) ([]AIGatewayConsumerGroup, error) {
	if err := ValidateAPIClient(c.aiGatewayConsumerGroupsAPI, "AI Gateway Consumer Groups API"); err != nil {
		return nil, err
	}

	var allData []kkComps.AIGatewayConsumerGroup
	var pageAfter *string
	pageSize := int64(100)

	for {
		req := kkOps.ListAiGatewayConsumerGroupsRequest{
			GatewayID: gatewayID,
			PageSize:  &pageSize,
			PageAfter: pageAfter,
		}

		resp, err := c.aiGatewayConsumerGroupsAPI.ListAiGatewayConsumerGroups(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway Consumer Groups", nil)
		}

		if resp == nil || resp.ListAIGatewayConsumerGroupsResponse == nil {
			break
		}

		allData = append(allData, resp.ListAIGatewayConsumerGroupsResponse.Data...)

		nextCursor := pagination.ExtractPageAfterCursor(resp.ListAIGatewayConsumerGroupsResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	groups := make([]AIGatewayConsumerGroup, 0, len(allData))
	for _, group := range allData {
		groups = append(groups, AIGatewayConsumerGroup{
			AIGatewayConsumerGroup: group,
			NormalizedLabels:       normalizedAIGatewayConsumerGroupLabels(group),
		})
	}
	return groups, nil
}

// GetAIGatewayConsumerGroup fetches an AI Gateway Consumer Group by ID or name.
func (c *Client) GetAIGatewayConsumerGroup(
	ctx context.Context,
	gatewayID string,
	consumerGroupID string,
) (*AIGatewayConsumerGroup, error) {
	if err := ValidateAPIClient(c.aiGatewayConsumerGroupsAPI, "AI Gateway Consumer Groups API"); err != nil {
		return nil, err
	}

	resp, err := c.aiGatewayConsumerGroupsAPI.GetAiGatewayConsumerGroup(ctx, gatewayID, consumerGroupID)
	if err != nil {
		return nil, WrapAPIError(err, "get AI Gateway Consumer Group by ID", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConsumerGroup),
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayConsumerGroup == nil {
		return nil, nil
	}

	return &AIGatewayConsumerGroup{
		AIGatewayConsumerGroup: *resp.AIGatewayConsumerGroup,
		NormalizedLabels:       normalizedAIGatewayConsumerGroupLabels(*resp.AIGatewayConsumerGroup),
	}, nil
}

// GetAIGatewayConsumerGroupByName finds an AI Gateway Consumer Group by name within a gateway.
func (c *Client) GetAIGatewayConsumerGroupByName(
	ctx context.Context,
	gatewayID string,
	name string,
) (*AIGatewayConsumerGroup, error) {
	groups, err := c.ListAIGatewayConsumerGroups(ctx, gatewayID)
	if err != nil {
		return nil, WrapAPIError(err, "list AI Gateway Consumer Groups to find by name", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConsumerGroup),
			ResourceName: name,
			UseEnhanced:  true,
		})
	}

	for i := range groups {
		if groups[i].Name == name {
			return &groups[i], nil
		}
	}

	return nil, nil
}

// CreateAIGatewayConsumerGroup creates a new Consumer Group under an AI Gateway.
func (c *Client) CreateAIGatewayConsumerGroup(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewayConsumerGroupRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayConsumerGroupsAPI, "AI Gateway Consumer Groups API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayConsumerGroupsAPI.CreateAiGatewayConsumerGroup(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create AI Gateway Consumer Group", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConsumerGroup),
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayConsumerGroup == nil {
		return "", fmt.Errorf("create AI Gateway Consumer Group response missing data")
	}

	return resp.AIGatewayConsumerGroup.ID, nil
}

// UpdateAIGatewayConsumerGroup updates an existing Consumer Group under an AI Gateway.
func (c *Client) UpdateAIGatewayConsumerGroup(
	ctx context.Context,
	gatewayID string,
	consumerGroupID string,
	req kkComps.UpdateAIGatewayConsumerGroupRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayConsumerGroupsAPI, "AI Gateway Consumer Groups API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayConsumerGroupsAPI.UpdateAiGatewayConsumerGroup(ctx, kkOps.UpdateAiGatewayConsumerGroupRequest{
		GatewayID:                           gatewayID,
		ConsumerGroupIDOrName:               consumerGroupID,
		UpdateAIGatewayConsumerGroupRequest: req,
	})
	if err != nil {
		return "", WrapAPIError(err, "update AI Gateway Consumer Group", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConsumerGroup),
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayConsumerGroup == nil {
		return "", fmt.Errorf("update AI Gateway Consumer Group response missing data")
	}

	return resp.AIGatewayConsumerGroup.ID, nil
}

// DeleteAIGatewayConsumerGroup deletes an AI Gateway Consumer Group by ID.
func (c *Client) DeleteAIGatewayConsumerGroup(ctx context.Context, gatewayID string, consumerGroupID string) error {
	if err := ValidateAPIClient(c.aiGatewayConsumerGroupsAPI, "AI Gateway Consumer Groups API"); err != nil {
		return err
	}

	_, err := c.aiGatewayConsumerGroupsAPI.DeleteAiGatewayConsumerGroup(ctx, gatewayID, consumerGroupID)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway Consumer Group", nil)
	}

	return nil
}

func normalizedAIGatewayConsumerGroupLabels(group kkComps.AIGatewayConsumerGroup) map[string]string {
	normalized := group.Labels
	if normalized == nil {
		normalized = make(map[string]string)
	}
	return normalized
}
