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

// NormalizeAIGatewayAuthStrategy converts the SDK union response into a stable internal representation.
func NormalizeAIGatewayAuthStrategy(provider kkComps.AIGatewayAuthStrategy) (AIGatewayAuthStrategy, error) {
	raw, err := aiGatewayAuthStrategyRawMap(provider)
	if err != nil {
		return AIGatewayAuthStrategy{}, err
	}

	labels := stringMapFromRaw(raw["labels"])
	normalizedLabels := labels
	if normalizedLabels == nil {
		normalizedLabels = make(map[string]string)
	}

	return AIGatewayAuthStrategy{
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

func (c *Client) ListAIGatewayAuthStrategies(
	ctx context.Context,
	gatewayID string,
) ([]AIGatewayAuthStrategy, error) {
	if err := ValidateAPIClient(c.aiGatewayAuthStrategiesAPI, "AI Gateway Auth Strategies API"); err != nil {
		return nil, err
	}

	const defaultPageSize int64 = 100
	pageSize := defaultPageSize
	var pageAfter *string
	var all []AIGatewayAuthStrategy

	for {
		req := kkOps.ListAiGatewayAuthStrategiesRequest{
			GatewayID: gatewayID,
			PageSize:  &pageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		resp, err := c.aiGatewayAuthStrategiesAPI.ListAiGatewayAuthStrategies(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway Auth Strategies", nil)
		}
		if resp == nil || resp.ListAIGatewayAuthStrategiesResponse == nil {
			return all, nil
		}

		for _, provider := range resp.ListAIGatewayAuthStrategiesResponse.Data {
			normalized, err := NormalizeAIGatewayAuthStrategy(provider)
			if err != nil {
				return nil, fmt.Errorf("normalize AI Gateway Auth Strategy: %w", err)
			}
			all = append(all, normalized)
		}

		nextCursor := pagination.ExtractPageAfterCursor(resp.ListAIGatewayAuthStrategiesResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return all, nil
}

func (c *Client) GetAIGatewayAuthStrategy(
	ctx context.Context,
	gatewayID string,
	providerID string,
) (*AIGatewayAuthStrategy, error) {
	if err := ValidateAPIClient(c.aiGatewayAuthStrategiesAPI, "AI Gateway Auth Strategies API"); err != nil {
		return nil, err
	}

	resp, err := c.aiGatewayAuthStrategiesAPI.GetAiGatewayAuthStrategy(ctx, gatewayID, providerID)
	if err != nil {
		return nil, WrapAPIError(err, "get AI Gateway Auth Strategy", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayAuthStrategy),
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.AIGatewayAuthStrategy == nil {
		return nil, nil
	}

	normalized, err := NormalizeAIGatewayAuthStrategy(*resp.AIGatewayAuthStrategy)
	if err != nil {
		return nil, fmt.Errorf("normalize AI Gateway Auth Strategy: %w", err)
	}
	return &normalized, nil
}

func (c *Client) GetAIGatewayAuthStrategyByName(
	ctx context.Context,
	gatewayID string,
	name string,
) (*AIGatewayAuthStrategy, error) {
	providers, err := c.ListAIGatewayAuthStrategies(ctx, gatewayID)
	if err != nil {
		return nil, WrapAPIError(err, "list AI Gateway Auth Strategies to find by name", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayAuthStrategy),
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

func (c *Client) CreateAIGatewayAuthStrategy(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewayAuthStrategyRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayAuthStrategiesAPI, "AI Gateway Auth Strategies API"); err != nil {
		return "", err
	}

	resourceName := aiGatewayAuthStrategyCreateRequestName(req)
	resp, err := c.aiGatewayAuthStrategiesAPI.CreateAiGatewayAuthStrategy(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create AI Gateway Auth Strategy", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayAuthStrategy),
			ResourceName: resourceName,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.AIGatewayAuthStrategy == nil {
		return "", fmt.Errorf("create AI Gateway Auth Strategy response missing data")
	}

	normalized, err := NormalizeAIGatewayAuthStrategy(*resp.AIGatewayAuthStrategy)
	if err != nil {
		return "", fmt.Errorf("normalize AI Gateway Auth Strategy: %w", err)
	}
	return normalized.ID, nil
}

func (c *Client) UpdateAIGatewayAuthStrategy(
	ctx context.Context,
	gatewayID string,
	providerID string,
	req kkComps.UpdateAIGatewayAuthStrategyRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayAuthStrategiesAPI, "AI Gateway Auth Strategies API"); err != nil {
		return "", err
	}

	resourceName := aiGatewayAuthStrategyUpdateRequestName(req)
	resp, err := c.aiGatewayAuthStrategiesAPI.UpdateAiGatewayAuthStrategy(
		ctx,
		kkOps.UpdateAiGatewayAuthStrategyRequest{
			GatewayID:                          gatewayID,
			AuthStrategyIDOrName:               providerID,
			UpdateAIGatewayAuthStrategyRequest: req,
		},
	)
	if err != nil {
		return "", WrapAPIError(err, "update AI Gateway Auth Strategy", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayAuthStrategy),
			ResourceName: resourceName,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.AIGatewayAuthStrategy == nil {
		return "", fmt.Errorf("update AI Gateway Auth Strategy response missing data")
	}

	normalized, err := NormalizeAIGatewayAuthStrategy(*resp.AIGatewayAuthStrategy)
	if err != nil {
		return "", fmt.Errorf("normalize AI Gateway Auth Strategy: %w", err)
	}
	return normalized.ID, nil
}

func (c *Client) DeleteAIGatewayAuthStrategy(ctx context.Context, gatewayID string, providerID string) error {
	if err := ValidateAPIClient(c.aiGatewayAuthStrategiesAPI, "AI Gateway Auth Strategies API"); err != nil {
		return err
	}

	_, err := c.aiGatewayAuthStrategiesAPI.DeleteAiGatewayAuthStrategy(ctx, gatewayID, providerID)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway Auth Strategy", nil)
	}
	return nil
}

func aiGatewayAuthStrategyRawMap(provider kkComps.AIGatewayAuthStrategy) (map[string]any, error) {
	data, err := json.Marshal(provider)
	if err != nil {
		return nil, fmt.Errorf("marshal AI Gateway Auth Strategy: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal AI Gateway Auth Strategy: %w", err)
	}
	return raw, nil
}

func aiGatewayAuthStrategyCreateRequestName(req kkComps.CreateAIGatewayAuthStrategyRequest) string {
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

func aiGatewayAuthStrategyUpdateRequestName(req kkComps.UpdateAIGatewayAuthStrategyRequest) string {
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
