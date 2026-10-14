package dump

import (
	"cmp"
	"context"
	"slices"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	decllabels "github.com/kong/kongctl/internal/declarative/labels"
	declresources "github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

func collectDeclarativeMeters(
	ctx context.Context,
	api helpers.MetersAPI,
	pageSize int64,
	filter filterOptions,
) ([]declresources.MeterResource, error) {
	values, err := helpers.ListAllMeters(ctx, api, pageSize)
	if err != nil {
		return nil, err
	}
	result := make([]declresources.MeterResource, 0)
	for _, m := range filterByNameOrID(values, filter, func(m kkComps.Meter) (string, string) { return m.Name, m.ID }) {
		result = append(result, mapMeterToDeclarativeResource(m))
	}
	slices.SortFunc(result, func(a, b declresources.MeterResource) int { return cmp.Compare(a.Key, b.Key) })
	return result, nil
}

func mapMeterToDeclarativeResource(m kkComps.Meter) declresources.MeterResource {
	return declresources.MeterResource{
		BaseResource:  declresources.BaseResource{Ref: m.Key, Kongctl: kongctlMetaFromLabels(m.Labels)},
		ID:            m.ID,
		Name:          m.Name,
		Key:           m.Key,
		Aggregation:   kkComps.Aggregation(m.Aggregation),
		EventType:     m.EventType,
		Description:   m.Description,
		ValueProperty: m.ValueProperty,
		EventsFrom:    meterEventsFrom(m),
		Dimensions:    m.Dimensions,
		Labels:        decllabels.GetUserLabels(m.Labels),
	}
}

func meterEventsFrom(m kkComps.Meter) *string {
	if m.EventsFrom == nil {
		return nil
	}
	value := m.EventsFrom.Format(time.RFC3339Nano)
	return &value
}
