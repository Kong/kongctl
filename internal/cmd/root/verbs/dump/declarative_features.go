package dump

import (
	"cmp"
	"context"
	"slices"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	decllabels "github.com/kong/kongctl/internal/declarative/labels"
	declresources "github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

func collectDeclarativeFeatures(
	ctx context.Context,
	api helpers.FeaturesAPI,
	pageSize int64,
	filter filterOptions,
) ([]declresources.FeatureResource, error) {
	values, err := helpers.ListAllFeatures(ctx, api, pageSize)
	if err != nil {
		return nil, err
	}
	result := make([]declresources.FeatureResource, 0)
	for _, m := range filterByNameOrID(values, filter, func(m kkComps.Feature) (string, string) { return m.Name, m.ID }) {
		result = append(result, mapFeatureToDeclarativeResource(m))
	}
	slices.SortFunc(result, func(a, b declresources.FeatureResource) int { return cmp.Compare(a.Key, b.Key) })
	return result, nil
}

func mapFeatureToDeclarativeResource(m kkComps.Feature) declresources.FeatureResource {
	return declresources.FeatureResource{
		BaseResource: declresources.BaseResource{Ref: m.Key, Kongctl: kongctlMetaFromLabels(m.Labels)},
		ID:           m.ID,
		Name:         m.Name,
		Key:          m.Key,
		Description:  m.Description,
		Labels:       decllabels.GetUserLabels(m.Labels),
		Meter:        featureDumpMeter(m),
		UnitCost:     helpers.FeatureCostConfiguration(m.UnitCost),
		UnitCostSet:  true,
	}
}

func featureDumpMeter(m kkComps.Feature) *kkComps.CreateFeatureRequestMeterReference {
	if m.Meter == nil {
		return nil
	}
	return &kkComps.CreateFeatureRequestMeterReference{ID: m.Meter.ID, Filters: m.Meter.Filters}
}
