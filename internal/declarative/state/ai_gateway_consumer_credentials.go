package state

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util/pagination"
)

// ListAIGatewayConsumerCredentials lists all Credentials for an AI Gateway Consumer.
func (c *Client) ListAIGatewayConsumerCredentials(
	ctx context.Context,
	gatewayID string,
	consumerID string,
) ([]AIGatewayConsumerCredential, error) {
	if err := ValidateAPIClient(c.aiGatewayConsumersAPI, "AI Gateway Consumers API"); err != nil {
		return nil, err
	}

	var allData []kkComps.AIGatewayConsumerCredential
	var pageAfter *string
	pageSize := int64(100)

	for {
		req := kkOps.ListAiGatewayConsumerCredentialsRequest{
			GatewayID:  gatewayID,
			ConsumerID: consumerID,
			PageSize:   &pageSize,
			PageAfter:  pageAfter,
		}

		resp, err := c.aiGatewayConsumersAPI.ListAiGatewayConsumerCredentials(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway Consumer Credentials", nil)
		}

		if resp == nil || resp.ListAIGatewayConsumerCredentialsResponse == nil {
			break
		}

		allData = append(allData, resp.ListAIGatewayConsumerCredentialsResponse.Data...)

		nextCursor := pagination.ExtractPageAfterCursor(resp.ListAIGatewayConsumerCredentialsResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	credentials := make([]AIGatewayConsumerCredential, 0, len(allData))
	for _, credential := range allData {
		credentials = append(credentials, AIGatewayConsumerCredential{
			AIGatewayConsumerCredential: credential,
			NormalizedLabels:            normalizedAIGatewayConsumerCredentialLabels(credential),
		})
	}
	return credentials, nil
}

// GetAIGatewayConsumerCredential fetches an AI Gateway Consumer Credential by ID.
func (c *Client) GetAIGatewayConsumerCredential(
	ctx context.Context,
	gatewayID string,
	consumerID string,
	credentialID string,
) (*AIGatewayConsumerCredential, error) {
	if err := ValidateAPIClient(c.aiGatewayConsumersAPI, "AI Gateway Consumers API"); err != nil {
		return nil, err
	}

	resp, err := c.aiGatewayConsumersAPI.GetAiGatewayConsumerCredential(ctx, kkOps.GetAiGatewayConsumerCredentialRequest{
		GatewayID:    gatewayID,
		ConsumerID:   consumerID,
		CredentialID: credentialID,
	})
	if err != nil {
		return nil, WrapAPIError(err, "get AI Gateway Consumer Credential by ID", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConsumerCredential),
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayConsumerCredential == nil {
		return nil, nil
	}

	return &AIGatewayConsumerCredential{
		AIGatewayConsumerCredential: *resp.AIGatewayConsumerCredential,
		NormalizedLabels:            normalizedAIGatewayConsumerCredentialLabels(*resp.AIGatewayConsumerCredential),
	}, nil
}

// GetAIGatewayConsumerCredentialByName finds an AI Gateway Consumer Credential by name within a Consumer.
func (c *Client) GetAIGatewayConsumerCredentialByName(
	ctx context.Context,
	gatewayID string,
	consumerID string,
	name string,
) (*AIGatewayConsumerCredential, error) {
	credentials, err := c.ListAIGatewayConsumerCredentials(ctx, gatewayID, consumerID)
	if err != nil {
		return nil, WrapAPIError(err, "list AI Gateway Consumer Credentials to find by name", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConsumerCredential),
			ResourceName: name,
			UseEnhanced:  true,
		})
	}

	for i := range credentials {
		if credentials[i].Name == name {
			return &credentials[i], nil
		}
	}

	return nil, nil
}

// CreateAIGatewayConsumerCredential creates a new Credential under an AI Gateway Consumer.
func (c *Client) CreateAIGatewayConsumerCredential(
	ctx context.Context,
	gatewayID string,
	consumerID string,
	req kkComps.CreateAIGatewayConsumerCredentialRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayConsumersAPI, "AI Gateway Consumers API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayConsumersAPI.CreateAiGatewayConsumerCredential(
		ctx,
		kkOps.CreateAiGatewayConsumerCredentialRequest{
			GatewayID:                                gatewayID,
			ConsumerID:                               consumerID,
			CreateAIGatewayConsumerCredentialRequest: req,
		},
	)
	if err != nil {
		return "", WrapAPIError(err, "create AI Gateway Consumer Credential", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayConsumerCredential),
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayConsumerCredentialWithKey == nil {
		return "", fmt.Errorf("create AI Gateway Consumer Credential response missing data")
	}

	return resp.AIGatewayConsumerCredentialWithKey.ID, nil
}

// DeleteAIGatewayConsumerCredential deletes an AI Gateway Consumer Credential by ID.
func (c *Client) DeleteAIGatewayConsumerCredential(
	ctx context.Context,
	gatewayID string,
	consumerID string,
	credentialID string,
) error {
	if err := ValidateAPIClient(c.aiGatewayConsumersAPI, "AI Gateway Consumers API"); err != nil {
		return err
	}

	req := kkOps.DeleteAiGatewayConsumerCredentialRequest{
		GatewayID:    gatewayID,
		ConsumerID:   consumerID,
		CredentialID: credentialID,
	}
	_, err := c.aiGatewayConsumersAPI.DeleteAiGatewayConsumerCredential(ctx, req)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway Consumer Credential", nil)
	}

	return nil
}

func normalizedAIGatewayConsumerCredentialLabels(
	credential kkComps.AIGatewayConsumerCredential,
) map[string]string {
	normalized := credential.Labels
	if normalized == nil {
		normalized = make(map[string]string)
	}
	return normalized
}
