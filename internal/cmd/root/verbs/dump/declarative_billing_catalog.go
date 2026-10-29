package dump

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/kong/kongctl/internal/declarative/labels"
	r "github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

func collectBillingCatalog[T any](ctx context.Context, d *declarativeDumpContext, kind r.ResourceType) ([]T, error) {
	api := d.sdk.GetBillingCatalogAPI()
	if api == nil {
		return nil, fmt.Errorf("billing catalog API is not configured")
	}
	values, err := api.List(ctx, r.BillingKindForResourceType(kind), d.pageSize)
	if err != nil {
		return nil, err
	}
	values = filterByNameOrID(
		values,
		d.filter,
		func(v helpers.BillingObject) (string, string) { return v.String("name"), v.ID() },
	)
	slices.SortFunc(values, func(a, b helpers.BillingObject) int { return cmp.Compare(a.ID(), b.ID()) })
	result := make([]T, 0, len(values))
	for _, v := range values {
		if kind == r.ResourceTypeCurrency && v.String("type") != "custom" {
			continue
		}
		fields, err := helpers.BillingConfiguration(r.BillingKindForResourceType(kind), v)
		if err != nil {
			return nil, err
		}
		fields["id"] = v.ID()
		fields["ref"] = string(kind) + "-" + v.ID()
		if kind != r.ResourceTypeCurrency {
			fields["kongctl"] = kongctlMetaFromLabels(v.Labels())
			fields["labels"] = labels.GetUserLabels(v.Labels())
		}
		if kind == r.ResourceTypeBillingPlan {
			fields["version"] = v["version"]
		}
		data, err := json.Marshal(fields)
		if err != nil {
			return nil, err
		}
		var target T
		if err = json.Unmarshal(data, &target); err != nil {
			return nil, err
		}
		result = append(result, target)
	}
	return result, nil
}
