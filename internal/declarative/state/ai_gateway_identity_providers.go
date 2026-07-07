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

// NormalizeAIGatewayIdentityProvider converts the SDK union response into a stable internal representation.
func NormalizeAIGatewayIdentityProvider(provider kkComps.AIGatewayIdentityProvider) (AIGatewayIdentityProvider, error) {
	raw, err := aiGatewayIdentityProviderRawMap(provider)
	if err != nil {
		return AIGatewayIdentityProvider{}, err
	}

	labels := stringMapFromRaw(raw["labels"])
	normalizedLabels := labels
	if normalizedLabels == nil {
		normalizedLabels = make(map[string]string)
	}

	return AIGatewayIdentityProvider{
		ID:               stringFromRaw(raw["id"]),
		Name:             stringFromRaw(raw["name"]),
		Type:             stringFromRaw(raw["type"]),
		DisplayName:      stringFromRaw(raw["display_name"]),
		Labels:           labels,
		ManagedBy:        stringMapFromRaw(raw["managed_by"]),
		Config:           mapFromRaw(raw["config"]),
		CreatedAt:        timeStringFromRaw(raw["created_at"]),
		UpdatedAt:        timeStringFromRaw(raw["updated_at"]),
		Raw:              raw,
		NormalizedLabels: normalizedLabels,
	}, nil
}

func (c *Client) ListAIGatewayIdentityProviders(
	ctx context.Context,
	gatewayID string,
) ([]AIGatewayIdentityProvider, error) {
	if err := ValidateAPIClient(c.aiGatewayIdentityProvidersAPI, "AI Gateway Identity Providers API"); err != nil {
		return nil, err
	}

	const defaultPageSize int64 = 100
	pageSize := defaultPageSize
	var pageAfter *string
	var all []AIGatewayIdentityProvider

	for {
		req := kkOps.ListAiGatewayIdentityProvidersRequest{
			GatewayID: gatewayID,
			PageSize:  &pageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		resp, err := c.aiGatewayIdentityProvidersAPI.ListAiGatewayIdentityProviders(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway Identity Providers", nil)
		}
		if resp == nil || resp.ListAIGatewayIdentityProvidersResponse == nil {
			return all, nil
		}

		for _, provider := range resp.ListAIGatewayIdentityProvidersResponse.Data {
			normalized, err := NormalizeAIGatewayIdentityProvider(provider)
			if err != nil {
				return nil, fmt.Errorf("normalize AI Gateway Identity Provider: %w", err)
			}
			all = append(all, normalized)
		}

		nextCursor := pagination.ExtractPageAfterCursor(resp.ListAIGatewayIdentityProvidersResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return all, nil
}

func (c *Client) GetAIGatewayIdentityProvider(
	ctx context.Context,
	gatewayID string,
	providerID string,
) (*AIGatewayIdentityProvider, error) {
	if err := ValidateAPIClient(c.aiGatewayIdentityProvidersAPI, "AI Gateway Identity Providers API"); err != nil {
		return nil, err
	}

	resp, err := c.aiGatewayIdentityProvidersAPI.GetAiGatewayIdentityProvider(ctx, gatewayID, providerID)
	if err != nil {
		return nil, WrapAPIError(err, "get AI Gateway Identity Provider", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayIdentityProvider),
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.AIGatewayIdentityProvider == nil {
		return nil, nil
	}

	normalized, err := NormalizeAIGatewayIdentityProvider(*resp.AIGatewayIdentityProvider)
	if err != nil {
		return nil, fmt.Errorf("normalize AI Gateway Identity Provider: %w", err)
	}
	return &normalized, nil
}

func (c *Client) GetAIGatewayIdentityProviderByName(
	ctx context.Context,
	gatewayID string,
	name string,
) (*AIGatewayIdentityProvider, error) {
	providers, err := c.ListAIGatewayIdentityProviders(ctx, gatewayID)
	if err != nil {
		return nil, WrapAPIError(err, "list AI Gateway Identity Providers to find by name", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayIdentityProvider),
			ResourceName: name,
			UseEnhanced:  true,
		})
	}
	for i := range providers {
		if providers[i].Name == name {
			return &providers[i], nil
		}
	}
	return nil, nil
}

func (c *Client) CreateAIGatewayIdentityProvider(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewayIdentityProviderRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayIdentityProvidersAPI, "AI Gateway Identity Providers API"); err != nil {
		return "", err
	}

	resourceName := aiGatewayIdentityProviderCreateRequestName(req)
	resp, err := c.aiGatewayIdentityProvidersAPI.CreateAiGatewayIdentityProvider(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create AI Gateway Identity Provider", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayIdentityProvider),
			ResourceName: resourceName,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.AIGatewayIdentityProvider == nil {
		return "", fmt.Errorf("create AI Gateway Identity Provider response missing data")
	}

	normalized, err := NormalizeAIGatewayIdentityProvider(*resp.AIGatewayIdentityProvider)
	if err != nil {
		return "", fmt.Errorf("normalize AI Gateway Identity Provider: %w", err)
	}
	return normalized.ID, nil
}

func (c *Client) UpdateAIGatewayIdentityProvider(
	ctx context.Context,
	gatewayID string,
	providerID string,
	req kkComps.UpdateAIGatewayIdentityProviderRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayIdentityProvidersAPI, "AI Gateway Identity Providers API"); err != nil {
		return "", err
	}

	resourceName := aiGatewayIdentityProviderUpdateRequestName(req)
	resp, err := c.aiGatewayIdentityProvidersAPI.UpdateAiGatewayIdentityProvider(
		ctx,
		kkOps.UpdateAiGatewayIdentityProviderRequest{
			GatewayID:                              gatewayID,
			IdentityProviderIDOrName:               providerID,
			UpdateAIGatewayIdentityProviderRequest: req,
		},
	)
	if err != nil {
		return "", WrapAPIError(err, "update AI Gateway Identity Provider", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayIdentityProvider),
			ResourceName: resourceName,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.AIGatewayIdentityProvider == nil {
		return "", fmt.Errorf("update AI Gateway Identity Provider response missing data")
	}

	normalized, err := NormalizeAIGatewayIdentityProvider(*resp.AIGatewayIdentityProvider)
	if err != nil {
		return "", fmt.Errorf("normalize AI Gateway Identity Provider: %w", err)
	}
	return normalized.ID, nil
}

func (c *Client) DeleteAIGatewayIdentityProvider(ctx context.Context, gatewayID string, providerID string) error {
	if err := ValidateAPIClient(c.aiGatewayIdentityProvidersAPI, "AI Gateway Identity Providers API"); err != nil {
		return err
	}

	_, err := c.aiGatewayIdentityProvidersAPI.DeleteAiGatewayIdentityProvider(ctx, gatewayID, providerID)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway Identity Provider", nil)
	}
	return nil
}

func aiGatewayIdentityProviderRawMap(provider kkComps.AIGatewayIdentityProvider) (map[string]any, error) {
	data, err := json.Marshal(provider)
	if err != nil {
		return nil, fmt.Errorf("marshal AI Gateway Identity Provider: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal AI Gateway Identity Provider: %w", err)
	}
	return raw, nil
}

func aiGatewayIdentityProviderCreateRequestName(req kkComps.CreateAIGatewayIdentityProviderRequest) string {
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

func aiGatewayIdentityProviderUpdateRequestName(req kkComps.UpdateAIGatewayIdentityProviderRequest) string {
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
