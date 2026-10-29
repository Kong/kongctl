package state

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type Feature struct {
	kkComps.Feature
	NormalizedLabels map[string]string
}

// ListFeatures includes unmanaged features so global key collisions cannot cause
// implicit adoption or cross-namespace mutations during planning.
func (c *Client) ListFeatures(ctx context.Context) ([]Feature, error) {
	values, err := helpers.ListAllFeatures(ctx, c.featuresAPI, 100)
	if err != nil {
		return nil, WrapAPIError(err, "list features", nil)
	}
	result := make([]Feature, 0, len(values))
	for _, v := range values {
		result = append(result, Feature{Feature: v, NormalizedLabels: v.Labels})
	}
	return result, nil
}

func (c *Client) GetFeatureByID(ctx context.Context, id string) (*Feature, error) {
	if c.featuresAPI == nil {
		return nil, fmt.Errorf("features API not configured")
	}
	resp, err := c.featuresAPI.GetFeature(ctx, id)
	if err != nil {
		return nil, WrapAPIError(err, "get feature", nil)
	}
	if resp == nil || resp.Feature == nil {
		return nil, fmt.Errorf("get feature response missing data")
	}
	return &Feature{Feature: *resp.Feature, NormalizedLabels: resp.Feature.Labels}, nil
}

func (c *Client) CreateFeature(
	ctx context.Context,
	req kkComps.CreateFeatureRequest,
	_ string,
) (*kkComps.Feature, error) {
	if c.featuresAPI == nil {
		return nil, fmt.Errorf("features API not configured")
	}
	if err := helpers.ValidateFeatureCreate(req); err != nil {
		return nil, err
	}
	resp, err := c.featuresAPI.CreateFeature(ctx, req)
	if err != nil {
		return nil, WrapAPIError(err, "create feature", nil)
	}
	if resp == nil || resp.Feature == nil {
		return nil, fmt.Errorf("create feature response missing data")
	}
	return resp.Feature, nil
}

func (c *Client) UpdateFeature(
	ctx context.Context,
	id string,
	req helpers.FeatureUpdateRequest,
	_ string,
) (*kkComps.Feature, error) {
	if c.featuresAPI == nil {
		return nil, fmt.Errorf("features API not configured")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	resp, err := c.featuresAPI.UpdateFeature(ctx, id, req)
	if err != nil {
		return nil, WrapAPIError(err, "update feature", nil)
	}
	if resp == nil || resp.Feature == nil {
		return nil, fmt.Errorf("update feature response missing data")
	}
	return resp.Feature, nil
}

func (c *Client) DeleteFeature(ctx context.Context, id string) error {
	if err := c.ValidateBillingDeletion(ctx, resources.ResourceTypeFeature, id); err != nil {
		return err
	}
	if c.featuresAPI == nil {
		return fmt.Errorf("features API not configured")
	}
	_, err := c.featuresAPI.DeleteFeature(ctx, id)
	if err != nil {
		return WrapAPIError(err, "delete feature", nil)
	}
	return nil
}
