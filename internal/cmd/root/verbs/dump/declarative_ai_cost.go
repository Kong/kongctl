package dump

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"

	"github.com/kong/kongctl/internal/declarative/state"

	declexport "github.com/kong/kongctl/internal/declarative/export"
	"github.com/kong/kongctl/internal/declarative/resources"
)

func aiCostBudgetCollector() declarativeCollector {
	return rootCollector("ai.cost_management.budgets",
		func(ctx context.Context, d *declarativeDumpContext) ([]resources.AICostBudget, error) {
			return collectAICost(ctx, d, resources.ResourceTypeAICostBudget,
				func(base resources.BaseResource, config resources.AICostBudgetConfig) resources.AICostBudget {
					return resources.AICostBudget{BaseResource: base, AICostBudgetConfig: config}
				})
		}, func(rs *resources.ResourceSet) *[]resources.AICostBudget { return &rs.EnsureCostManagement().Budgets },
		func(ctx context.Context, d *declarativeDumpContext, budgets []resources.AICostBudget) error {
			for i := range budgets {
				budget := &budgets[i]
				if budget.Effect != "limit" {
					continue
				}
				rule, err := d.stateClient.Cost.Get(ctx, resources.ResourceTypeAICostRule, budget.GetKonnectID())
				if err != nil {
					return err
				}
				metadata := rule.GetManagedBy()
				if metadata[resources.ManagedByToolKey] != resources.ManagedByTool ||
					metadata[resources.ChildNamespaceKey] == "" {
					continue
				}
				var config resources.AICostRuleConfig
				if err := decodeAICostConfig(resources.ResourceTypeAICostRule, rule, &config); err != nil {
					return err
				}
				budget.NotificationRule = &resources.AICostRule{
					BaseResource: resources.BaseResource{
						Kongctl: declexport.KongctlMetaFromLabels(resources.NormalizedChildOwnership(metadata)),
					},
					AICostRuleConfig: config,
				}
			}
			return nil
		}, resources.ResourceTypeAICostRule)
}

func aiCostChannelCollector() declarativeCollector {
	return rootCollector("ai.cost_management.notification_channels",
		func(ctx context.Context, d *declarativeDumpContext) ([]resources.AICostChannel, error) {
			return collectAICost(ctx, d, resources.ResourceTypeAICostChannel,
				func(base resources.BaseResource, config resources.AICostChannelConfig) resources.AICostChannel {
					return resources.AICostChannel{BaseResource: base, AICostChannelConfig: config}
				})
		}, func(rs *resources.ResourceSet) *[]resources.AICostChannel {
			return &rs.EnsureCostManagement().NotificationChannels
		}, nil)
}

func aiCostScheduleCollector() declarativeCollector {
	return rootCollector("ai.cost_management.export_schedules",
		func(ctx context.Context, d *declarativeDumpContext) ([]resources.AICostSchedule, error) {
			return collectAICost(ctx, d, resources.ResourceTypeAICostSchedule,
				func(base resources.BaseResource, config resources.AICostScheduleConfig) resources.AICostSchedule {
					return resources.AICostSchedule{BaseResource: base, AICostScheduleConfig: config}
				})
		}, func(rs *resources.ResourceSet) *[]resources.AICostSchedule {
			return &rs.EnsureCostManagement().ExportSchedules
		}, nil)
}

func collectAICost[R, C any](ctx context.Context, d *declarativeDumpContext, kind resources.ResourceType,
	wrap func(resources.BaseResource, C) R,
) ([]R, error) {
	size := d.pageSize
	if size == 0 {
		size = 100
	}
	items, err := d.stateClient.Cost.List(ctx, kind, size)
	if err != nil {
		return nil, err
	}
	items = filterByNameOrID(
		items,
		d.filter,
		func(item state.AICostObject) (string, string) { return item.GetName(), item.GetID() },
	)
	slices.SortFunc(items, func(a, b state.AICostObject) int { return cmp.Compare(a.GetID(), b.GetID()) })
	result := make([]R, 0, len(items))
	for _, item := range items {
		metadata := item.GetManagedBy()
		ref := metadata[resources.ManagedByRefKey]
		if metadata[resources.ManagedByToolKey] != resources.ManagedByTool || ref == "" ||
			metadata[resources.ChildNamespaceKey] == "" {
			continue
		}
		var config C
		if err := decodeAICostConfig(kind, item, &config); err != nil {
			return nil, err
		}
		base := resources.BaseResource{
			Ref:     ref,
			Kongctl: declexport.KongctlMetaFromLabels(resources.NormalizedChildOwnership(metadata)),
		}
		base.SetKonnectID(item.GetID())
		result = append(result, wrap(base, config))
	}
	return result, nil
}

func decodeAICostConfig(kind resources.ResourceType, item state.AICostObject, dst any) error {
	data, err := json.Marshal(resources.AICostWriteFields(kind, item))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}
