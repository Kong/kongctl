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

// ListAIGatewayVaults lists all Vaults for an AI Gateway.
func (c *Client) ListAIGatewayVaults(ctx context.Context, gatewayID string) ([]AIGatewayVault, error) {
	if err := ValidateAPIClient(c.aiGatewayVaultsAPI, "AI Gateway Vaults API"); err != nil {
		return nil, err
	}

	var allData []kkComps.AIGatewayVault
	var pageAfter *string
	pageSize := int64(100)

	for {
		req := kkOps.ListAiGatewayVaultsRequest{
			GatewayID: gatewayID,
			PageSize:  &pageSize,
			PageAfter: pageAfter,
		}

		resp, err := c.aiGatewayVaultsAPI.ListAiGatewayVaults(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway Vaults", nil)
		}

		if resp == nil || resp.ListAIGatewayVaultsResponse == nil {
			break
		}

		allData = append(allData, resp.ListAIGatewayVaultsResponse.Data...)

		nextCursor := pagination.ExtractPageAfterCursor(resp.ListAIGatewayVaultsResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	vaults := make([]AIGatewayVault, 0, len(allData))
	for _, vault := range allData {
		vaults = append(vaults, AIGatewayVault{
			AIGatewayVault:   vault,
			NormalizedLabels: normalizedAIGatewayVaultLabels(vault),
		})
	}
	return vaults, nil
}

// GetAIGatewayVault fetches an AI Gateway Vault by ID or name.
func (c *Client) GetAIGatewayVault(
	ctx context.Context,
	gatewayID string,
	vaultID string,
) (*AIGatewayVault, error) {
	if err := ValidateAPIClient(c.aiGatewayVaultsAPI, "AI Gateway Vaults API"); err != nil {
		return nil, err
	}

	resp, err := c.aiGatewayVaultsAPI.GetAiGatewayVault(ctx, gatewayID, vaultID)
	if err != nil {
		return nil, WrapAPIError(err, "get AI Gateway Vault by ID", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayVault),
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayVault == nil {
		return nil, nil
	}

	return &AIGatewayVault{
		AIGatewayVault:   *resp.AIGatewayVault,
		NormalizedLabels: normalizedAIGatewayVaultLabels(*resp.AIGatewayVault),
	}, nil
}

// GetAIGatewayVaultByName finds an AI Gateway Vault by name within a gateway.
func (c *Client) GetAIGatewayVaultByName(
	ctx context.Context,
	gatewayID string,
	name string,
) (*AIGatewayVault, error) {
	vaults, err := c.ListAIGatewayVaults(ctx, gatewayID)
	if err != nil {
		return nil, WrapAPIError(err, "list AI Gateway Vaults to find by name", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayVault),
			ResourceName: name,
			UseEnhanced:  true,
		})
	}

	for i := range vaults {
		if resources.AIGatewayVaultName(vaults[i].AIGatewayVault) == name {
			return &vaults[i], nil
		}
	}

	return nil, nil
}

// CreateAIGatewayVault creates a new Vault under an AI Gateway.
func (c *Client) CreateAIGatewayVault(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewayVaultRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayVaultsAPI, "AI Gateway Vaults API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayVaultsAPI.CreateAiGatewayVault(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create AI Gateway Vault", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayVault),
			ResourceName: aiGatewayVaultCreateRequestName(req),
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayVault == nil {
		return "", fmt.Errorf("create AI Gateway Vault response missing data")
	}

	return resources.AIGatewayVaultID(*resp.AIGatewayVault), nil
}

// UpdateAIGatewayVault updates an existing Vault under an AI Gateway.
func (c *Client) UpdateAIGatewayVault(
	ctx context.Context,
	gatewayID string,
	vaultID string,
	req kkComps.UpdateAIGatewayVaultRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayVaultsAPI, "AI Gateway Vaults API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayVaultsAPI.UpdateAiGatewayVault(ctx, kkOps.UpdateAiGatewayVaultRequest{
		GatewayID:                   gatewayID,
		VaultID:                     vaultID,
		UpdateAIGatewayVaultRequest: req,
	})
	if err != nil {
		return "", WrapAPIError(err, "update AI Gateway Vault", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayVault),
			ResourceName: aiGatewayVaultUpdateRequestName(req),
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayVault == nil {
		return "", fmt.Errorf("update AI Gateway Vault response missing data")
	}

	return resources.AIGatewayVaultID(*resp.AIGatewayVault), nil
}

// DeleteAIGatewayVault deletes an AI Gateway Vault by ID.
func (c *Client) DeleteAIGatewayVault(ctx context.Context, gatewayID string, vaultID string) error {
	if err := ValidateAPIClient(c.aiGatewayVaultsAPI, "AI Gateway Vaults API"); err != nil {
		return err
	}

	_, err := c.aiGatewayVaultsAPI.DeleteAiGatewayVault(ctx, gatewayID, vaultID)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway Vault", nil)
	}

	return nil
}

func normalizedAIGatewayVaultLabels(vault kkComps.AIGatewayVault) map[string]string {
	normalized := resources.AIGatewayVaultLabels(vault)
	if normalized == nil {
		normalized = make(map[string]string)
	}
	return normalized
}

func aiGatewayVaultCreateRequestName(req kkComps.CreateAIGatewayVaultRequest) string {
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

func aiGatewayVaultUpdateRequestName(req kkComps.UpdateAIGatewayVaultRequest) string {
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
