package planner

import (
	"context"
	"fmt"
)

// Meter deletion must inspect features even when features are outside sync
// scope. The feature PATCH contract cannot detach an existing meter reference.
func (p *Planner) resolveMeterFeatureDeletes(ctx context.Context, plan *Plan) error {
	var deletions []*PlannedChange
	featureDeletes := map[string]string{}
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.Action != ActionDelete {
			continue
		}
		switch change.ResourceType {
		case ResourceTypeMeter:
			deletions = append(deletions, change)
		case ResourceTypeFeature:
			featureDeletes[change.ResourceID] = change.ID
		}
	}
	if len(deletions) == 0 {
		return nil
	}
	features, err := p.listFeatures(ctx)
	if err != nil {
		return fmt.Errorf("inspect features before deleting meters: %w", err)
	}
	for _, deletion := range deletions {
		for _, feature := range features {
			if feature.Meter == nil || feature.Meter.ID != deletion.ResourceID {
				continue
			}
			if dependency := featureDeletes[feature.ID]; dependency != "" {
				deletion.DependsOn = appendDependsOn(deletion.DependsOn, dependency)
			} else {
				return fmt.Errorf("cannot delete meter %q while feature %q still references it; "+
					"delete the feature first or include it in the deletion scope", deletion.ResourceRef, feature.Key)
			}
		}
		for _, change := range plan.Changes {
			if change.ResourceType != ResourceTypeFeature || change.Action != ActionCreate {
				continue
			}
			meter, _ := change.Fields[FieldMeter].(map[string]any)
			if meter[FieldID] == deletion.ResourceID {
				return fmt.Errorf(
					"cannot delete meter %q while planned feature %q references it",
					deletion.ResourceRef,
					change.ResourceRef,
				)
			}
		}
	}
	return nil
}
