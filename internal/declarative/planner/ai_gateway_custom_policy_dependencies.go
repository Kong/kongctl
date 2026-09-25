package planner

import (
	"context"
	"fmt"
)

// Custom definitions must exist before policies instantiate their plugin type.
// A definition can be removed only after its policy instances are removed or changed.
func (p *Planner) resolveAIGatewayCustomPolicyDependencies(
	ctx context.Context, namespace, gatewayRef, gatewayID string, plan *Plan,
) error {
	creates := make(map[string]string)
	policyChanges := make(map[string]*PlannedChange)
	var deletes []int
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
			if change.Action == ActionDelete {
				deletes = append(deletes, i)
			}
		}
		if change.ResourceType == ResourceTypeAIGatewayPolicy && change.ResourceID != "" {
			policyChanges[change.ResourceID] = change
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
		for _, index := range deletes {
			if plan.Changes[index].Fields[FieldName] == typeName {
				return fmt.Errorf(
					"cannot delete custom policy %q while planned policy %q uses it",
					typeName,
					change.ResourceRef,
				)
			}
		}
	}
	if len(deletes) == 0 {
		return nil
	}
	current, err := p.client.ListAIGatewayPolicies(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("inspect policies before deleting custom definitions: %w", err)
	}
	for _, index := range deletes {
		deletion := &plan.Changes[index]
		name, _ := deletion.Fields[FieldName].(string)
		for _, policy := range current {
			if policy.Type != name {
				continue
			}
			change := policyChanges[policy.ID]
			if change == nil || (change.Action != ActionDelete &&
				(change.Action != ActionUpdate || change.Fields[FieldType] == name || change.Fields[FieldType] == nil)) {
				return fmt.Errorf("cannot delete custom policy %q while policy %q still uses it", name, policy.Name)
			}
			deletion.DependsOn = appendDependsOn(deletion.DependsOn, change.ID)
		}
	}
	return nil
}
