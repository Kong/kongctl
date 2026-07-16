package state

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util/pagination"
)

// ListAIGatewayConsumers lists all Consumers for an AI Gateway.
func (c *Client) ListAIGatewayConsumers(ctx context.Context, gatewayID string) ([]AIGatewayConsumer, error) {
	if err := ValidateAPIClient(c.aiGatewayConsumersAPI, "AI Gateway Consumers API"); err != nil {
		return nil, err
	}

	var allData []kkComps.AIGatewayConsumer
	var pageAfter *string
	pageSize := int64(100)

	for {
		req := kkOps.ListAiGatewayConsumersRequest{
			GatewayID: gatewayID,
			PageSize:  &pageSize,
			PageAfter: pageAfter,
		}

		resp, err := c.aiGatewayConsumersAPI.ListAiGatewayConsumers(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway Consumers", nil)
		}

		if resp == nil || resp.ListAIGatewayConsumersResponse == nil {
			break
		}

		allData = append(allData, resp.ListAIGatewayConsumersResponse.Data...)

		nextCursor := pagination.ExtractPageAfterCursor(resp.ListAIGatewayConsumersResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	consumers := make([]AIGatewayConsumer, 0, len(allData))
	for _, consumer := range allData {
		consumers = append(consumers, AIGatewayConsumer{
			AIGatewayConsumer: consumer,
			NormalizedLabels:  normalizedAIGatewayConsumerLabels(consumer),
		})
	}
	return consumers, nil
}

// GetAIGatewayConsumer fetches an AI Gateway Consumer by ID or name.
func (c *Client) GetAIGatewayConsumer(
	ctx context.Context,
	gatewayID string,
	consumerID string,
) (*AIGatewayConsumer, error) {
	if err := ValidateAPIClient(c.aiGatewayConsumersAPI, "AI Gateway Consumers API"); err != nil {
		return nil, err
	}

	resp, err := c.aiGatewayConsumersAPI.GetAiGatewayConsumer(ctx, gatewayID, consumerID)
	if err != nil {
		return nil, WrapAPIError(err, "get AI Gateway Consumer by ID", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConsumer),
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayConsumer == nil {
		return nil, nil
	}

	return &AIGatewayConsumer{
		AIGatewayConsumer: *resp.AIGatewayConsumer,
		NormalizedLabels:  normalizedAIGatewayConsumerLabels(*resp.AIGatewayConsumer),
	}, nil
}

// GetAIGatewayConsumerByName finds an AI Gateway Consumer by name within a gateway.
func (c *Client) GetAIGatewayConsumerByName(
	ctx context.Context,
	gatewayID string,
	name string,
) (*AIGatewayConsumer, error) {
	consumers, err := c.ListAIGatewayConsumers(ctx, gatewayID)
	if err != nil {
		return nil, WrapAPIError(err, "list AI Gateway Consumers to find by name", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConsumer),
			ResourceName: name,
			UseEnhanced:  true,
		})
	}

	for i := range consumers {
		if consumers[i].Name == name {
			return &consumers[i], nil
		}
	}

	return nil, nil
}

// CreateAIGatewayConsumer creates a new Consumer under an AI Gateway.
func (c *Client) CreateAIGatewayConsumer(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewayConsumerRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayConsumersAPI, "AI Gateway Consumers API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayConsumersAPI.CreateAiGatewayConsumer(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create AI Gateway Consumer", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConsumer),
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayConsumer == nil {
		return "", fmt.Errorf("create AI Gateway Consumer response missing data")
	}

	return resp.AIGatewayConsumer.ID, nil
}

// UpdateAIGatewayConsumer updates an existing Consumer under an AI Gateway.
func (c *Client) UpdateAIGatewayConsumer(
	ctx context.Context,
	gatewayID string,
	consumerID string,
	req kkComps.UpdateAIGatewayConsumerRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayConsumersAPI, "AI Gateway Consumers API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayConsumersAPI.UpdateAiGatewayConsumer(ctx, kkOps.UpdateAiGatewayConsumerRequest{
		GatewayID:                      gatewayID,
		ConsumerIDOrName:               consumerID,
		UpdateAIGatewayConsumerRequest: req,
	})
	if err != nil {
		return "", WrapAPIError(err, "update AI Gateway Consumer", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConsumer),
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayConsumer == nil {
		return "", fmt.Errorf("update AI Gateway Consumer response missing data")
	}

	return resp.AIGatewayConsumer.ID, nil
}

// DeleteAIGatewayConsumer deletes an AI Gateway Consumer by ID.
func (c *Client) DeleteAIGatewayConsumer(ctx context.Context, gatewayID string, consumerID string) error {
	if err := ValidateAPIClient(c.aiGatewayConsumersAPI, "AI Gateway Consumers API"); err != nil {
		return err
	}

	_, err := c.aiGatewayConsumersAPI.DeleteAiGatewayConsumer(ctx, gatewayID, consumerID)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway Consumer", nil)
	}

	return nil
}

func normalizedAIGatewayConsumerLabels(consumer kkComps.AIGatewayConsumer) map[string]string {
	normalized := consumer.Labels
	if normalized == nil {
		normalized = make(map[string]string)
	}
	return normalized
}
