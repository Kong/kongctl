package helpers

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// FeaturesAPI is the SDK contract shared by imperative and declarative features.
type FeaturesAPI interface {
	ListFeatures(context.Context, kkOps.ListFeaturesRequest, ...kkOps.Option) (*kkOps.ListFeaturesResponse, error)
	CreateFeature(context.Context, kkComps.CreateFeatureRequest, ...kkOps.Option) (*kkOps.CreateFeatureResponse, error)
	GetFeature(context.Context, string, ...kkOps.Option) (*kkOps.GetFeatureResponse, error)
	UpdateFeature(context.Context, string, FeatureUpdateRequest, ...kkOps.Option) (*kkOps.UpdateFeatureResponse, error)
	DeleteFeature(context.Context, string, ...kkOps.Option) (*kkOps.DeleteFeatureResponse, error)
	QueryFeatureCost(
		context.Context,
		string,
		*kkComps.MeterQueryRequest,
		...kkOps.Option,
	) (*kkOps.QueryFeatureCostResponse, error)
}

// ListAllFeatures follows API pagination before callers filter ownership or identity.
func ListAllFeatures(ctx context.Context, api FeaturesAPI, pageSize int64) ([]kkComps.Feature, error) {
	if api == nil {
		return nil, fmt.Errorf("features API client is not configured")
	}
	if pageSize < 1 || pageSize > 1000 {
		return nil, fmt.Errorf("feature page size must be between 1 and 1000")
	}
	result := make([]kkComps.Feature, 0)
	for page := int64(1); page <= 10000; page++ {
		resp, err := api.ListFeatures(
			ctx,
			kkOps.ListFeaturesRequest{Page: &kkComps.PagePaginationQuery{Size: &pageSize, Number: &page}},
		)
		if err != nil {
			return nil, fmt.Errorf("list features: %w", err)
		}
		if resp == nil || resp.FeaturePagePaginatedResponse == nil {
			return nil, fmt.Errorf("list features response missing data")
		}
		body := resp.FeaturePagePaginatedResponse
		result = append(result, body.Data...)
		if len(body.Data) == 0 || float64(len(result)) >= body.Meta.Page.Total {
			return result, nil
		}
	}
	return nil, fmt.Errorf("feature pagination exceeded safety limit of 10000 pages")
}

// ResolveFeature accepts API keys or ULIDs, never display names or local refs.
func ResolveFeature(ctx context.Context, api FeaturesAPI, identifier string, pageSize int64) (*kkComps.Feature, error) {
	if api == nil {
		return nil, fmt.Errorf("features API client is not configured")
	}
	if IsMeterULID(identifier) {
		resp, err := api.GetFeature(ctx, identifier)
		if err != nil {
			return nil, fmt.Errorf("get feature %q: %w", identifier, err)
		}
		if resp == nil || resp.Feature == nil {
			return nil, fmt.Errorf("get feature response missing data")
		}
		return resp.Feature, nil
	}
	values, err := ListAllFeatures(ctx, api, pageSize)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if value.Key == identifier {
			return &value, nil
		}
	}
	return nil, fmt.Errorf("feature key %q not found; use its API key or ULID", identifier)
}
