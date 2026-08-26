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

func (c *Client) ListAIGatewayConfigStoreSecrets(
	ctx context.Context,
	gatewayID string,
	configStoreIDOrName string,
) ([]AIGatewayConfigStoreSecret, error) {
	logAIGatewayConfigStoreSecretOperation(
		ctx, "listing AI Gateway Config Store secrets", gatewayID, configStoreIDOrName, "",
	)
	if err := ValidateAPIClient(c.aiGatewayConfigStoresAPI, "AI Gateway Config Stores API"); err != nil {
		return nil, err
	}

	var secrets []AIGatewayConfigStoreSecret
	var pageAfter *string
	pageSize := int64(100)
	for {
		resp, err := c.aiGatewayConfigStoresAPI.ListAiGatewayConfigStoreSecrets(
			ctx,
			kkOps.ListAiGatewayConfigStoreSecretsRequest{
				GatewayID:           gatewayID,
				ConfigStoreIDOrName: configStoreIDOrName,
				PageSize:            &pageSize,
				PageAfter:           pageAfter,
			},
		)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway Config Store secrets", nil)
		}
		if resp == nil || resp.ListAIGatewayConfigStoreSecretsResponse == nil {
			break
		}
		for _, secret := range resp.ListAIGatewayConfigStoreSecretsResponse.Data {
			secrets = append(secrets, AIGatewayConfigStoreSecret{AIGatewayConfigStoreSecret: secret})
		}
		next := pagination.ExtractPageAfterCursor(resp.ListAIGatewayConfigStoreSecretsResponse.Meta.Page.Next)
		if next == "" {
			break
		}
		pageAfter = &next
	}
	return secrets, nil
}

func (c *Client) GetAIGatewayConfigStoreSecret(
	ctx context.Context,
	gatewayID string,
	configStoreIDOrName string,
	key string,
) (*AIGatewayConfigStoreSecret, error) {
	if err := ValidateAPIClient(c.aiGatewayConfigStoresAPI, "AI Gateway Config Stores API"); err != nil {
		return nil, err
	}
	resp, err := c.aiGatewayConfigStoresAPI.GetAiGatewayConfigStoreSecret(
		ctx,
		kkOps.GetAiGatewayConfigStoreSecretRequest{
			GatewayID:           gatewayID,
			ConfigStoreIDOrName: configStoreIDOrName,
			Key:                 key,
		},
	)
	if err != nil {
		return nil, WrapAPIError(err, "get AI Gateway Config Store secret", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConfigStoreSecret),
			ResourceName: key,
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.AIGatewayConfigStoreSecret == nil {
		return nil, nil
	}
	return &AIGatewayConfigStoreSecret{AIGatewayConfigStoreSecret: *resp.AIGatewayConfigStoreSecret}, nil
}

func (c *Client) CreateAIGatewayConfigStoreSecret(
	ctx context.Context,
	gatewayID string,
	configStoreIDOrName string,
	req kkComps.CreateAIGatewayConfigStoreSecretRequest,
) (string, error) {
	logAIGatewayConfigStoreSecretOperation(
		ctx, "creating AI Gateway Config Store secret", gatewayID, configStoreIDOrName, req.Key,
	)
	if err := ValidateAPIClient(c.aiGatewayConfigStoresAPI, "AI Gateway Config Stores API"); err != nil {
		return "", err
	}
	resp, err := c.aiGatewayConfigStoresAPI.CreateAiGatewayConfigStoreSecret(
		ctx,
		kkOps.CreateAiGatewayConfigStoreSecretRequest{
			GatewayID:                               gatewayID,
			ConfigStoreIDOrName:                     configStoreIDOrName,
			CreateAIGatewayConfigStoreSecretRequest: req,
		},
	)
	if err != nil {
		return "", WrapAPIError(err, "create AI Gateway Config Store secret", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConfigStoreSecret),
			ResourceName: req.Key,
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.AIGatewayConfigStoreSecret == nil {
		return "", fmt.Errorf("create AI Gateway Config Store secret response missing data")
	}
	return resp.AIGatewayConfigStoreSecret.Key, nil
}

func (c *Client) UpdateAIGatewayConfigStoreSecret(
	ctx context.Context,
	gatewayID string,
	configStoreIDOrName string,
	key string,
	req kkComps.UpdateAIGatewayConfigStoreSecretRequest,
) (string, error) {
	logAIGatewayConfigStoreSecretOperation(
		ctx, "updating AI Gateway Config Store secret", gatewayID, configStoreIDOrName, key,
	)
	if err := ValidateAPIClient(c.aiGatewayConfigStoresAPI, "AI Gateway Config Stores API"); err != nil {
		return "", err
	}
	resp, err := c.aiGatewayConfigStoresAPI.UpdateAiGatewayConfigStoreSecret(
		ctx,
		kkOps.UpdateAiGatewayConfigStoreSecretRequest{
			GatewayID:                               gatewayID,
			ConfigStoreIDOrName:                     configStoreIDOrName,
			Key:                                     key,
			UpdateAIGatewayConfigStoreSecretRequest: req,
		},
	)
	if err != nil {
		return "", WrapAPIError(err, "update AI Gateway Config Store secret", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConfigStoreSecret),
			ResourceName: key,
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.AIGatewayConfigStoreSecret == nil {
		return "", fmt.Errorf("update AI Gateway Config Store secret response missing data")
	}
	return resp.AIGatewayConfigStoreSecret.Key, nil
}

func (c *Client) DeleteAIGatewayConfigStoreSecret(
	ctx context.Context,
	gatewayID string,
	configStoreIDOrName string,
	key string,
) error {
	logAIGatewayConfigStoreSecretOperation(
		ctx, "deleting AI Gateway Config Store secret", gatewayID, configStoreIDOrName, key,
	)
	if err := ValidateAPIClient(c.aiGatewayConfigStoresAPI, "AI Gateway Config Stores API"); err != nil {
		return err
	}
	_, err := c.aiGatewayConfigStoresAPI.DeleteAiGatewayConfigStoreSecret(
		ctx,
		kkOps.DeleteAiGatewayConfigStoreSecretRequest{
			GatewayID:           gatewayID,
			ConfigStoreIDOrName: configStoreIDOrName,
			Key:                 key,
		},
	)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway Config Store secret", nil)
	}
	return nil
}

func logAIGatewayConfigStoreSecretOperation(
	ctx context.Context,
	message string,
	gatewayID string,
	configStore string,
	key string,
) {
	logger, ok := ctx.Value(log.LoggerKey).(*slog.Logger)
	if !ok || logger == nil {
		return
	}
	logger.Debug(
		message,
		slog.String("gateway_id", gatewayID),
		slog.String("config_store", configStore),
		slog.String("key", key),
	)
}
