package planner

import (
	"context"
	"fmt"
	"slices"
)

// Custom definitions must exist before policies instantiate their plugin type.
// A definition can be removed only after its policy instances are removed or changed.
func (p *Planner) resolveAIGatewayCustomPolicyDependencies(
	ctx context.Context, namespace, gatewayRef, gatewayID string, plan *Plan,
) error {
	creates := make(map[string]string)
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.Namespace != namespace || !aiGatewayChildChangeMatchesParent(*change, gatewayRef) {
			continue
		}
		if change.ResourceType == ResourceTypeAIGatewayCustomPolicy {
			name, _ := change.Fields[FieldName].(string)
			if change.Action == ActionCreate {
				creates[name] = change.ID
			}
		}
	}
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.Namespace != namespace || change.ResourceType != ResourceTypeAIGatewayPolicy ||
			!aiGatewayChildChangeMatchesParent(*change, gatewayRef) || change.Action == ActionDelete {
			continue
		}
		typeName, _ := change.Fields[FieldType].(string)
		if id := creates[typeName]; id != "" {
			change.DependsOn = appendDependsOn(change.DependsOn, id)
		}
	}
	return resolveObservedReferenceDeletes(ctx, namespace, gatewayRef, plan, observedReferenceDeletePolicy{
		targetType: ResourceTypeAIGatewayCustomPolicy,
		usesTarget: func(kind string) bool { return kind == ResourceTypeAIGatewayPolicy },
		observe: func(ctx context.Context) ([]observedReferenceUser, error) {
			current, err := p.listAIGatewayPolicies(ctx, gatewayID)
			if err != nil {
				return nil, fmt.Errorf("inspect policies before deleting custom definitions: %w", err)
			}
			users := make([]observedReferenceUser, 0, len(current))
			for _, policy := range current {
				users = append(
					users,
					observedReferenceUser{ResourceTypeAIGatewayPolicy, policy.ID, policy.Name, []string{policy.Type}},
				)
			}
			return users, nil
		},
		observedReferences: func(deletion PlannedChange, values []string) bool {
			name, _ := deletion.Fields[FieldName].(string)
			return name != "" && slices.Contains(values, name)
		},
		plannedReferences: func(deletion, change PlannedChange) bool {
			return change.Fields[FieldType] == deletion.Fields[FieldName]
		},
		updateDetaches: func(deletion, change PlannedChange) bool {
			value, valid := change.Fields[FieldType].(string)
			return valid && value != "" && value != deletion.Fields[FieldName]
		},
		conflict: func(deletion PlannedChange, user observedReferenceUser, planned bool) error {
			qualifier := ""
			if planned {
				qualifier = "planned "
			}
			return fmt.Errorf(
				"cannot delete custom policy %q in gateway %q while %spolicy %q still uses it",
				deletion.Fields[FieldName],
				gatewayRef,
				qualifier,
				user.name,
			)
		},
	})
}
