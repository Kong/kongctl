package state

import (
	"context"
	"fmt"
	"log/slog"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/log"
	"github.com/kong/kongctl/internal/util/pagination"
)

func (c *Client) ListAIGatewayConfigStores(
	ctx context.Context,
	gatewayID string,
) ([]AIGatewayConfigStore, error) {
	logAIGatewayConfigStoreOperation(ctx, "listing AI Gateway Config Stores", gatewayID, "")
	if err := ValidateAPIClient(c.aiGatewayConfigStoresAPI, "AI Gateway Config Stores API"); err != nil {
		return nil, err
	}

	var stores []AIGatewayConfigStore
	var pageAfter *string
	pageSize := int64(100)
	for {
		resp, err := c.aiGatewayConfigStoresAPI.ListAiGatewayConfigStores(ctx, kkOps.ListAiGatewayConfigStoresRequest{
			GatewayID: gatewayID,
			PageSize:  &pageSize,
			PageAfter: pageAfter,
		})
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway Config Stores", nil)
		}
		if resp == nil || resp.ListAIGatewayConfigStoresResponse == nil {
			break
		}
		for _, store := range resp.ListAIGatewayConfigStoresResponse.Data {
			stores = append(stores, AIGatewayConfigStore{AIGatewayConfigStore: store})
		}
		next := pagination.ExtractPageAfterCursor(resp.ListAIGatewayConfigStoresResponse.Meta.Page.Next)
		if next == "" {
			break
		}
		pageAfter = &next
	}
	return stores, nil
}

func (c *Client) GetAIGatewayConfigStore(
	ctx context.Context,
	gatewayID string,
	configStoreIDOrName string,
) (*AIGatewayConfigStore, error) {
	if err := ValidateAPIClient(c.aiGatewayConfigStoresAPI, "AI Gateway Config Stores API"); err != nil {
		return nil, err
	}
	resp, err := c.aiGatewayConfigStoresAPI.GetAiGatewayConfigStore(ctx, gatewayID, configStoreIDOrName)
	if err != nil {
		return nil, WrapAPIError(err, "get AI Gateway Config Store", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConfigStore),
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.AIGatewayConfigStore == nil {
		return nil, nil
	}
	return &AIGatewayConfigStore{AIGatewayConfigStore: *resp.AIGatewayConfigStore}, nil
}

func (c *Client) CreateAIGatewayConfigStore(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewayConfigStoreRequest,
	namespace string,
) (string, error) {
	logAIGatewayConfigStoreOperation(ctx, "creating AI Gateway Config Store", gatewayID, req.Name)
	if err := ValidateAPIClient(c.aiGatewayConfigStoresAPI, "AI Gateway Config Stores API"); err != nil {
		return "", err
	}
	resp, err := c.aiGatewayConfigStoresAPI.CreateAiGatewayConfigStore(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create AI Gateway Config Store", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConfigStore),
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.AIGatewayConfigStore == nil {
		return "", fmt.Errorf("create AI Gateway Config Store response missing data")
	}
	return resp.AIGatewayConfigStore.ID, nil
}

func (c *Client) UpdateAIGatewayConfigStore(
	ctx context.Context,
	gatewayID string,
	configStoreID string,
	req kkComps.UpdateAIGatewayConfigStoreRequest,
	namespace string,
) (string, error) {
	logAIGatewayConfigStoreOperation(ctx, "updating AI Gateway Config Store", gatewayID, configStoreID)
	if err := ValidateAPIClient(c.aiGatewayConfigStoresAPI, "AI Gateway Config Stores API"); err != nil {
		return "", err
	}
	resp, err := c.aiGatewayConfigStoresAPI.UpdateAiGatewayConfigStore(ctx, kkOps.UpdateAiGatewayConfigStoreRequest{
		GatewayID:                         gatewayID,
		ConfigStoreIDOrName:               configStoreID,
		UpdateAIGatewayConfigStoreRequest: req,
	})
	if err != nil {
		return "", WrapAPIError(err, "update AI Gateway Config Store", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConfigStore),
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.AIGatewayConfigStore == nil {
		return "", fmt.Errorf("update AI Gateway Config Store response missing data")
	}
	return resp.AIGatewayConfigStore.ID, nil
}

func (c *Client) DeleteAIGatewayConfigStore(ctx context.Context, gatewayID string, configStoreID string) error {
	logAIGatewayConfigStoreOperation(ctx, "deleting AI Gateway Config Store", gatewayID, configStoreID)
	if err := ValidateAPIClient(c.aiGatewayConfigStoresAPI, "AI Gateway Config Stores API"); err != nil {
		return err
	}
	_, err := c.aiGatewayConfigStoresAPI.DeleteAiGatewayConfigStore(ctx, kkOps.DeleteAiGatewayConfigStoreRequest{
		GatewayID:           gatewayID,
		ConfigStoreIDOrName: configStoreID,
	})
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway Config Store", nil)
	}
	return nil
}

func logAIGatewayConfigStoreOperation(ctx context.Context, message string, gatewayID string, store string) {
	logger, ok := ctx.Value(log.LoggerKey).(*slog.Logger)
	if !ok || logger == nil {
		return
	}
	logger.Debug(message, slog.String("gateway_id", gatewayID), slog.String("config_store", store))
}
