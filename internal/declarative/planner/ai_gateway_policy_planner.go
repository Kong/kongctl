package planner

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func (p *Planner) planAIGatewayPolicyChanges(
	ctx context.Context,
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	gatewayChangeID string,
	desired []resources.AIGatewayPolicyResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning AI Gateway policy changes",
		slog.String("gateway_ref", gatewayRef),
		slog.String("gateway_id", gatewayID),
		slog.String("gateway_change_id", gatewayChangeID),
		slog.Int("desired_count", len(desired)),
	)

	if gatewayID == "" {
		p.planAIGatewayPolicyCreatesForNewGateway(namespace, gatewayRef, gatewayName, gatewayChangeID, desired, plan)
		return nil
	}

	currentPolicies, err := p.listAIGatewayPolicies(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway Policies for gateway %s: %w", gatewayID, err)
	}

	return p.reconcileOwnedAIGatewayPolicies(ctx, gatewayRef, gatewayName, gatewayID, desired, currentPolicies, plan)
}

func (p *Planner) planAIGatewayPolicyCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	policies []resources.AIGatewayPolicyResource,
	plan *Plan,
) {
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}
	for _, policy := range policies {
		p.planAIGatewayPolicyCreate(namespace, gatewayRef, gatewayName, "", policy, dependsOn, plan)
	}
}

func (p *Planner) planAIGatewayPolicyCreate(
	_ string,
	gatewayRef string,
	_ string,
	gatewayID string,
	policy resources.AIGatewayPolicyResource,
	dependsOn []string,
	plan *Plan,
) {
	policy.ManagedBy = resources.BuildChildManagedBy(
		resources.GetNamespace(policy.Kongctl), policy.Protected(), "",
	)
	fields, err := policy.MutablePayloadMap()
	if err != nil {
		plan.AddWarning(policy.GetRef(), fmt.Sprintf("failed to build AI Gateway Policy create payload: %s", err))
		return
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayPolicy, policy.Ref),
		ResourceType: ResourceTypeAIGatewayPolicy,
		ResourceRef:  policy.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		Namespace:    resources.GetNamespace(policy.Kongctl),
		DependsOn:    dependsOn,
		Protection:   policy.Protected(),
	}
	if gatewayID != "" {
		change.Parent = &ParentInfo{Ref: gatewayRef, ID: gatewayID}
	} else {
		change.References = map[string]ReferenceInfo{
			FieldAIGatewayID: {
				Ref: gatewayRef,
				LookupFields: map[string]string{
					FieldName: gatewayRef,
				},
			},
		}
	}

	plan.AddChange(change)
}

func (p *Planner) planAIGatewayPolicyUpdate(
	_ string,
	gatewayRef string,
	gatewayID string,
	policyID string,
	policy resources.AIGatewayPolicyResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeAIGatewayPolicy, policy.Ref),
		ResourceType:  ResourceTypeAIGatewayPolicy,
		ResourceRef:   policy.Ref,
		ResourceID:    policyID,
		Action:        ActionUpdate,
		Fields:        updateFields,
		ChangedFields: changedFields,
		Namespace:     resources.GetNamespace(policy.Kongctl),
		Parent:        &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func (p *Planner) planAIGatewayPolicyDelete(
	namespace string,
	gatewayRef string,
	gatewayID string,
	policyID string,
	policyName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGatewayPolicy, policyName),
		ResourceType: ResourceTypeAIGatewayPolicy,
		ResourceRef:  policyName,
		ResourceID:   policyID,
		Action:       ActionDelete,
		Namespace:    namespace,
		Fields: map[string]any{
			FieldName: policyName,
		},
		Parent: &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func shouldUpdateAIGatewayPolicy(
	current state.AIGatewayPolicy,
	desired resources.AIGatewayPolicyResource,
) (bool, map[string]any, map[string]FieldChange, error) {
	currentPayload, err := resources.AIGatewayPolicyMutablePayloadMap(current.AIGatewayPolicy)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to normalize current AI Gateway Policy: %w", err)
	}
	desiredPayload, err := desired.MutablePayloadMap()
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to normalize desired AI Gateway Policy %q: %w", desired.Ref, err)
	}

	if desired.Labels == nil {
		desiredPayload[FieldLabels] = currentPayload[FieldLabels]
	} else {
		desiredPayload[FieldLabels] = desired.Labels
	}
	if desired.ManagedBy == nil {
		desiredPayload[FieldManagedBy] = currentPayload[FieldManagedBy]
	}
	currentCompare, desiredCompare := normalizeAIGatewayPolicyPayloadsForComparison(currentPayload, desiredPayload)
	changedFields := diffAIGatewayPayloads(currentPayload, desiredPayload, currentCompare, desiredCompare)
	if len(changedFields) == 0 {
		return false, nil, nil, nil
	}

	return true, clonePayloadMap(desiredPayload), changedFields, nil
}

func aiGatewayPolicyCreateDependencies(plan *Plan, _ string, gatewayRef string) map[string]string {
	if plan == nil {
		return nil
	}

	depsByRefOrName := make(map[string]string)
	for _, change := range plan.Changes {
		if change.Action != ActionCreate ||
			change.ResourceType != ResourceTypeAIGatewayPolicy ||
			!aiGatewayChildChangeMatchesParent(change, gatewayRef) {
			continue
		}

		if change.ResourceRef != "" {
			depsByRefOrName[change.ResourceRef] = change.ID
		}
		name, ok := change.Fields[FieldName].(string)
		if ok && name != "" {
			depsByRefOrName[name] = change.ID
		}
	}
	if len(depsByRefOrName) == 0 {
		return nil
	}
	return depsByRefOrName
}

func aiGatewayPolicyReferenceDependencies(
	payload map[string]any,
	policyCreateDepsByRefOrName map[string]string,
) []string {
	if len(policyCreateDepsByRefOrName) == 0 {
		return nil
	}
	rawPolicies, ok := payload[FieldPolicies].([]any)
	if !ok {
		return nil
	}

	var deps []string
	for _, rawPolicy := range rawPolicies {
		policyRefOrName, ok := rawPolicy.(string)
		if !ok || policyRefOrName == "" {
			continue
		}
		if parsedRef, _, ok := tags.ParseRefPlaceholder(policyRefOrName); ok {
			policyRefOrName = parsedRef
		}
		if dep := policyCreateDepsByRefOrName[policyRefOrName]; dep != "" {
			deps = appendDependsOn(deps, dep)
		}
	}
	return deps
}
