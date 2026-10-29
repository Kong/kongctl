package planner

import (
	"context"
	"fmt"
	"maps"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

// Dependencies are checked across the complete organization, including parents
// outside the selected namespaces and sync scope.
func (p *Planner) resolveBillingDeletes(ctx context.Context, plan *Plan) error {
	byID := map[string]*PlannedChange{}
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.ResourceID != "" {
			byID[change.ResourceType+":"+change.ResourceID] = change
		}
	}
	for i := range plan.Changes {
		deletion := &plan.Changes[i]
		if deletion.Action != ActionDelete {
			continue
		}
		kind := resources.ResourceType(deletion.ResourceType)
		for _, parent := range state.BillingDependentKinds(kind) {
			values, err := p.listBilling(ctx, parent)
			if err != nil {
				return fmt.Errorf("inspect %s before deleting %s: %w", parent, kind, err)
			}
			for _, value := range values {
				effective := maps.Clone(value)
				change := byID[string(parent)+":"+value.ID()]
				if change != nil && change.Action == ActionUpdate {
					maps.Copy(effective, change.Fields)
				}
				referenced, err := billingReferencesID(parent, effective, kind, deletion.ResourceID)
				if err != nil {
					return err
				}
				if !referenced {
					before, err := billingReferencesID(parent, value, kind, deletion.ResourceID)
					if err != nil {
						return err
					}
					if before && change != nil {
						deletion.DependsOn = appendDependsOn(deletion.DependsOn, change.ID)
					}
					continue
				}
				if change != nil && change.Action == ActionDelete {
					deletion.DependsOn = appendDependsOn(deletion.DependsOn, change.ID)
				} else {
					return fmt.Errorf(
						"cannot delete %s %q: %s %s still references it; include the dependent resource in the deletion scope",
						kind, deletion.ResourceRef, parent, value.ID(),
					)
				}
			}
			for _, change := range plan.Changes {
				if change.ResourceType != string(parent) || change.Action != ActionCreate {
					continue
				}
				referenced, err := billingReferencesID(parent, change.Fields, kind, deletion.ResourceID)
				if err != nil {
					return err
				}
				if referenced {
					return fmt.Errorf(
						"cannot delete %s %q: planned %s %q references it",
						kind,
						deletion.ResourceRef,
						parent,
						change.ResourceRef,
					)
				}
			}
		}
	}
	return nil
}

func billingReferencesID(
	parent resources.ResourceType,
	value helpers.BillingObject,
	kind resources.ResourceType,
	id string,
) (bool, error) {
	for path, target := range resources.BillingReferenceTargets(parent, value) {
		if target != kind {
			continue
		}
		v, _, err := resources.BillingPointer(value, path)
		if err != nil {
			return false, err
		}
		if v == id {
			return true, nil
		}
	}
	return false, nil
}
