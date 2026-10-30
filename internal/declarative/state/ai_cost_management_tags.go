package state

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func (c *Client) GetAICostManagementTags(ctx context.Context) (*kkComps.CostManagementTags, error) {
	if err := ValidateAPIClient(c.aiCostManagementTagsAPI, "AI cost management tags API"); err != nil {
		return nil, err
	}
	resp, err := c.aiCostManagementTagsAPI.GetCostManagementTags(ctx)
	if err != nil {
		return nil, WrapAPIError(err, "get AI cost management tags", nil)
	}
	if resp == nil || resp.CostManagementTags == nil {
		return nil, fmt.Errorf("AI cost management tags response missing data")
	}
	return resp.CostManagementTags, nil
}

func (c *Client) UpdateAICostManagementTags(ctx context.Context, req kkComps.UpdateCostManagementTags) error {
	if err := ValidateAPIClient(c.aiCostManagementTagsAPI, "AI cost management tags API"); err != nil {
		return err
	}
	_, err := c.aiCostManagementTagsAPI.UpdateCostManagementTags(ctx, req)
	if err != nil {
		return WrapAPIError(err, "update AI cost management tags", nil)
	}
	return nil
}
