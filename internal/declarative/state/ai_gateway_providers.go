package state

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util/pagination"
)

// NormalizeAIGatewayProvider converts the SDK union response into a stable internal representation.
func NormalizeAIGatewayProvider(provider kkComps.AIGatewayModelProvider) (AIGatewayProvider, error) {
	raw, err := aiGatewayProviderRawMap(provider)
	if err != nil {
		return AIGatewayProvider{}, err
	}

	labels := stringMapFromRaw(raw["labels"])
	normalizedLabels := labels
	if normalizedLabels == nil {
		normalizedLabels = make(map[string]string)
	}

	return AIGatewayProvider{
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

func (c *Client) ListAIGatewayProviders(ctx context.Context, gatewayID string) ([]AIGatewayProvider, error) {
	if err := ValidateAPIClient(c.aiGatewayProvidersAPI, "AI Gateway Model Providers API"); err != nil {
		return nil, err
	}

	const defaultPageSize int64 = 100
	pageSize := defaultPageSize
	var pageAfter *string
	var all []AIGatewayProvider

	for {
		req := kkOps.ListAiGatewayModelProvidersRequest{
			GatewayID: gatewayID,
			PageSize:  &pageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		resp, err := c.aiGatewayProvidersAPI.ListAiGatewayProviders(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway Model Providers", nil)
		}
		if resp == nil || resp.ListAIGatewayModelProvidersResponse == nil {
			return all, nil
		}

		for _, provider := range resp.ListAIGatewayModelProvidersResponse.Data {
			normalized, err := NormalizeAIGatewayProvider(provider)
			if err != nil {
				return nil, fmt.Errorf("normalize AI Gateway Model Provider: %w", err)
			}
			all = append(all, normalized)
		}

		nextCursor := pagination.ExtractPageAfterCursor(resp.ListAIGatewayModelProvidersResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return all, nil
}

func (c *Client) GetAIGatewayProvider(
	ctx context.Context,
	gatewayID string,
	providerID string,
) (*AIGatewayProvider, error) {
	if err := ValidateAPIClient(c.aiGatewayProvidersAPI, "AI Gateway Model Providers API"); err != nil {
		return nil, err
	}

	resp, err := c.aiGatewayProvidersAPI.GetAiGatewayProvider(ctx, gatewayID, providerID)
	if err != nil {
		return nil, WrapAPIError(err, "get AI Gateway Model Provider", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayProvider),
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.AIGatewayModelProvider == nil {
		return nil, nil
	}

	normalized, err := NormalizeAIGatewayProvider(*resp.AIGatewayModelProvider)
	if err != nil {
		return nil, fmt.Errorf("normalize AI Gateway Model Provider: %w", err)
	}
	return &normalized, nil
}

func (c *Client) GetAIGatewayProviderByName(
	ctx context.Context,
	gatewayID string,
	name string,
) (*AIGatewayProvider, error) {
	providers, err := c.ListAIGatewayProviders(ctx, gatewayID)
	if err != nil {
		return nil, WrapAPIError(err, "list AI Gateway Model Providers to find by name", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayProvider),
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

func (c *Client) CreateAIGatewayProvider(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewayModelProviderRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayProvidersAPI, "AI Gateway Model Providers API"); err != nil {
		return "", err
	}

	resourceName := aiGatewayProviderCreateRequestName(req)
	resp, err := c.aiGatewayProvidersAPI.CreateAiGatewayProvider(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create AI Gateway Model Provider", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayProvider),
			ResourceName: resourceName,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.AIGatewayModelProvider == nil {
		return "", fmt.Errorf("create AI Gateway Model Provider response missing data")
	}

	normalized, err := NormalizeAIGatewayProvider(*resp.AIGatewayModelProvider)
	if err != nil {
		return "", fmt.Errorf("normalize AI Gateway Model Provider: %w", err)
	}
	return normalized.ID, nil
}

func (c *Client) UpdateAIGatewayProvider(
	ctx context.Context,
	gatewayID string,
	providerID string,
	req kkComps.UpdateAIGatewayModelProviderRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayProvidersAPI, "AI Gateway Model Providers API"); err != nil {
		return "", err
	}

	resourceName := aiGatewayProviderUpdateRequestName(req)
	resp, err := c.aiGatewayProvidersAPI.UpdateAiGatewayProvider(ctx, kkOps.UpdateAiGatewayModelProviderRequest{
		GatewayID:                           gatewayID,
		ModelProviderIDOrName:               providerID,
		UpdateAIGatewayModelProviderRequest: req,
	})
	if err != nil {
		return "", WrapAPIError(err, "update AI Gateway Model Provider", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayProvider),
			ResourceName: resourceName,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}
	if resp == nil || resp.AIGatewayModelProvider == nil {
		return "", fmt.Errorf("update AI Gateway Model Provider response missing data")
	}

	normalized, err := NormalizeAIGatewayProvider(*resp.AIGatewayModelProvider)
	if err != nil {
		return "", fmt.Errorf("normalize AI Gateway Model Provider: %w", err)
	}
	return normalized.ID, nil
}

func (c *Client) DeleteAIGatewayProvider(ctx context.Context, gatewayID string, providerID string) error {
	if err := ValidateAPIClient(c.aiGatewayProvidersAPI, "AI Gateway Model Providers API"); err != nil {
		return err
	}

	_, err := c.aiGatewayProvidersAPI.DeleteAiGatewayProvider(ctx, gatewayID, providerID)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway Model Provider", nil)
	}
	return nil
}

func aiGatewayProviderRawMap(provider kkComps.AIGatewayModelProvider) (map[string]any, error) {
	data, err := json.Marshal(provider)
	if err != nil {
		return nil, fmt.Errorf("marshal AI Gateway Model Provider: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal AI Gateway Model Provider: %w", err)
	}
	return raw, nil
}

func aiGatewayProviderCreateRequestName(req kkComps.CreateAIGatewayModelProviderRequest) string {
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

func aiGatewayProviderUpdateRequestName(req kkComps.UpdateAIGatewayModelProviderRequest) string {
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

func stringFromRaw(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprint(value)
}

func timeStringFromRaw(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	if t, ok := value.(time.Time); ok && !t.IsZero() {
		return t.Format(time.RFC3339)
	}
	return stringFromRaw(value)
}

func mapFromRaw(value any) map[string]any {
	if value == nil {
		return nil
	}
	if m, ok := value.(map[string]any); ok {
		return m
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func stringMapFromRaw(value any) map[string]string {
	if value == nil {
		return nil
	}
	if m, ok := value.(map[string]string); ok {
		return m
	}
	raw, ok := value.(map[string]any)
	if !ok {
		data, err := json.Marshal(value)
		if err != nil {
			return nil
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil
		}
	}
	out := make(map[string]string, len(raw))
	for key, v := range raw {
		if s, ok := v.(string); ok {
			out[key] = s
		}
	}
	return out
}
