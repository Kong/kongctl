package planner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
)

type aiCostManagementTagsPlanner struct{ *BasePlanner }

func (p *aiCostManagementTagsPlanner) PlanChanges(
	ctx context.Context,
	cfg *Config,
	plan *Plan,
) error {
	desired := resources.GetResourcesByNamespace[resources.AICostManagementTagsResource](
		p.planner.resources,
		cfg.Namespace,
	)
	if len(desired) == 0 {
		return nil
	}
	if plan.Metadata.Mode == PlanModeDelete {
		return fmt.Errorf(
			"AI cost management tags cannot be deleted; apply explicit empty headers and prefixes to reset capture",
		)
	}
	current, err := p.GetClient().GetAICostManagementTags(ctx)
	if err != nil {
		return err
	}
	fields, err := desired[0].PayloadMap()
	if err != nil {
		return err
	}
	data, err := json.Marshal(current)
	if err != nil {
		return err
	}
	var old map[string]any
	if err := json.Unmarshal(data, &old); err != nil {
		return err
	}
	delete(old, resources.SchemaFieldCreatedAt)
	delete(old, resources.SchemaFieldUpdatedAt)
	oldCompare, desiredCompare := normalizeAIGatewayPayloadsForComparison(old, fields)
	changes := diffAIGatewayPayloads(old, fields, oldCompare, desiredCompare)
	if len(changes) == 0 {
		return nil
	}
	plan.AddChange(PlannedChange{
		ID: p.NextChangeID(
			ActionUpdate,
			ResourceTypeAICostManagementTags,
			desired[0].Ref,
		),
		ResourceType: ResourceTypeAICostManagementTags, ResourceRef: desired[0].Ref,
		Action:        ActionUpdate,
		ResourceID:    ResourceTypeAICostManagementTags,
		Namespace:     cfg.Namespace,
		Fields:        fields,
		ChangedFields: changes,
	})
	return nil
}
