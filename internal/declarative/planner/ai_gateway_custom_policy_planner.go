package planner

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) planAIGatewayCustomPolicyChanges(
	ctx context.Context,
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	gatewayChangeID string,
	desired []resources.AIGatewayCustomPolicyResource,
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
		p.planAIGatewayCustomPolicyCreatesForNewGateway(
			namespace,
			gatewayRef,
			gatewayName,
			gatewayChangeID,
			desired,
			plan,
		)
		return nil
	}

	currentPolicies, err := p.listAIGatewayCustomPolicies(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway Custom Policies for gateway %s: %w", gatewayID, err)
	}

	return p.reconcileOwnedAIGatewayCustomPolicies(ctx, gatewayRef, gatewayName, gatewayID, desired, currentPolicies, plan)
}

func (p *Planner) planAIGatewayCustomPolicyCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	policies []resources.AIGatewayCustomPolicyResource,
	plan *Plan,
) {
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}
	for _, policy := range policies {
		p.planAIGatewayCustomPolicyCreate(namespace, gatewayRef, gatewayName, "", policy, dependsOn, plan)
	}
}

func (p *Planner) planAIGatewayCustomPolicyCreate(
	_ string,
	gatewayRef string,
	_ string,
	gatewayID string,
	policy resources.AIGatewayCustomPolicyResource,
	dependsOn []string,
	plan *Plan,
) {
	policy.ManagedBy = resources.BuildChildManagedBy(nil, policy.ManagedBy,
		resources.GetNamespace(policy.Kongctl), policy.Protected(), "")
	fields, err := policy.MutablePayloadMap()
	if err != nil {
		plan.AddWarning(
			policy.GetRef(),
			fmt.Sprintf("failed to build AI Gateway Custom Policy create payload: %s", err),
		)
		return
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayCustomPolicy, policy.Ref),
		ResourceType: ResourceTypeAIGatewayCustomPolicy,
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

func (p *Planner) planAIGatewayCustomPolicyUpdate(
	_ string,
	gatewayRef string,
	gatewayID string,
	policyID string,
	policy resources.AIGatewayCustomPolicyResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeAIGatewayCustomPolicy, policy.Ref),
		ResourceType:  ResourceTypeAIGatewayCustomPolicy,
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

func (p *Planner) planAIGatewayCustomPolicyDelete(
	namespace string,
	gatewayRef string,
	gatewayID string,
	policyID string,
	policyName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGatewayCustomPolicy, policyName),
		ResourceType: ResourceTypeAIGatewayCustomPolicy,
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

func shouldUpdateAIGatewayCustomPolicy(
	current state.AIGatewayCustomPolicy,
	desired resources.AIGatewayCustomPolicyResource,
) (bool, map[string]any, map[string]FieldChange, error) {
	currentPayload, err := resources.AIGatewayCustomPolicyMutablePayloadMap(current.AIGatewayCustomPolicy)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to normalize current AI Gateway Custom Policy: %w", err)
	}
	desiredPayload, err := desired.MutablePayloadMap()
	if err != nil {
		return false, nil, nil, fmt.Errorf(
			"failed to normalize desired AI Gateway Custom Policy %q: %w",
			desired.Ref,
			err,
		)
	}

	if currentPayload[FieldType] != desired.Type {
		return false, nil, nil, fmt.Errorf("AI Gateway custom policy type is immutable")
	}
	if desired.Labels == nil {
		desiredPayload[FieldLabels] = currentPayload[FieldLabels]
	}
	if desired.ManagedBy == nil {
		desiredPayload[FieldManagedBy] = currentPayload[FieldManagedBy]
	}
	currentCompare, desiredCompare := normalizeAIGatewayPayloadsForComparison(currentPayload, desiredPayload)
	changedFields := diffAIGatewayPayloads(currentPayload, desiredPayload, currentCompare, desiredCompare)
	if len(changedFields) == 0 {
		return false, nil, nil, nil
	}

	return true, clonePayloadMap(desiredPayload), changedFields, nil
}
