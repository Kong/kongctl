package planner

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/labels"
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

	currentPolicies, err := p.client.ListAIGatewayCustomPolicies(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway Custom Policies for gateway %s: %w", gatewayID, err)
	}

	return reconcileNameMatchedChildren(p, ResourceTypeAIGatewayCustomPolicy, desired, currentPolicies,
		nameMatchedChildOperations[resources.AIGatewayCustomPolicyResource, state.AIGatewayCustomPolicy]{
			desiredName: func(desired resources.AIGatewayCustomPolicyResource) string {
				return desired.Name
			},
			currentName: func(current state.AIGatewayCustomPolicy) string {
				return resources.AIGatewayCustomPolicyName(current.AIGatewayCustomPolicy)
			},
			fetch: func(
				desired resources.AIGatewayCustomPolicyResource,
				current state.AIGatewayCustomPolicy,
			) (*state.AIGatewayCustomPolicy, error) {
				id := resources.AIGatewayCustomPolicyID(current.AIGatewayCustomPolicy)
				if policy := p.resources.GetAIGatewayCustomPolicyByRef(desired.Ref); policy != nil {
					policy.SetKonnectID(id)
				}
				full, err := p.client.GetAIGatewayCustomPolicy(ctx, gatewayID, id)
				if err != nil {
					return nil, fmt.Errorf("failed to get AI Gateway Custom Policy %s: %w", id, err)
				}
				return full, nil
			},
			diff: shouldUpdateAIGatewayCustomPolicy,
			create: func(desired resources.AIGatewayCustomPolicyResource) {
				p.planAIGatewayCustomPolicyCreate(namespace, gatewayRef, gatewayName, gatewayID, desired, nil, plan)
			},
			update: func(current state.AIGatewayCustomPolicy, desired resources.AIGatewayCustomPolicyResource,
				fields map[string]any, changed map[string]FieldChange,
			) {
				p.planAIGatewayCustomPolicyUpdate(namespace, gatewayRef, gatewayID,
					resources.AIGatewayCustomPolicyID(current.AIGatewayCustomPolicy), desired, fields, changed, plan)
			},
			protected: func(current state.AIGatewayCustomPolicy) bool {
				return labels.IsProtectedResource(current.NormalizedLabels)
			},
			remove: func(current state.AIGatewayCustomPolicy) {
				p.planAIGatewayCustomPolicyDelete(namespace, gatewayRef, gatewayID,
					resources.AIGatewayCustomPolicyID(current.AIGatewayCustomPolicy),
					resources.AIGatewayCustomPolicyName(current.AIGatewayCustomPolicy), plan)
			},
		}, plan)
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
	namespace string,
	gatewayRef string,
	_ string,
	gatewayID string,
	policy resources.AIGatewayCustomPolicyResource,
	dependsOn []string,
	plan *Plan,
) {
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
		Namespace:    namespace,
		DependsOn:    dependsOn,
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
	namespace string,
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
		Namespace:     namespace,
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

	currentCompare, desiredCompare := currentPayload, desiredPayload
	changedFields := diffAIGatewayPayloads(currentPayload, desiredPayload, currentCompare, desiredCompare)
	if len(changedFields) == 0 {
		return false, nil, nil, nil
	}

	return true, clonePayloadMap(desiredPayload), changedFields, nil
}
