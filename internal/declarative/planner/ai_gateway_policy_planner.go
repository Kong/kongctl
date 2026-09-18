package planner

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/labels"
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

	currentPolicies, err := p.client.ListAIGatewayPolicies(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway Policies for gateway %s: %w", gatewayID, err)
	}

	return reconcileNameMatchedChildren(p, ResourceTypeAIGatewayPolicy, desired, currentPolicies,
		nameMatchedChildOperations[resources.AIGatewayPolicyResource, state.AIGatewayPolicy]{
			desiredName: func(desired resources.AIGatewayPolicyResource) string {
				return desired.Name
			},
			currentName: func(current state.AIGatewayPolicy) string {
				return resources.AIGatewayPolicyName(current.AIGatewayPolicy)
			},
			fetch: func(
				desired resources.AIGatewayPolicyResource,
				current state.AIGatewayPolicy,
			) (*state.AIGatewayPolicy, error) {
				id := resources.AIGatewayPolicyID(current.AIGatewayPolicy)
				if policy := p.resources.GetAIGatewayPolicyByRef(desired.Ref); policy != nil {
					policy.SetKonnectID(id)
				}
				full, err := p.client.GetAIGatewayPolicy(ctx, gatewayID, id)
				if err != nil {
					return nil, fmt.Errorf("failed to get AI Gateway Policy %s: %w", id, err)
				}
				return full, nil
			},
			diff: shouldUpdateAIGatewayPolicy,
			create: func(desired resources.AIGatewayPolicyResource) {
				p.planAIGatewayPolicyCreate(namespace, gatewayRef, gatewayName, gatewayID, desired, nil, plan)
			},
			update: func(current state.AIGatewayPolicy, desired resources.AIGatewayPolicyResource,
				fields map[string]any, changed map[string]FieldChange,
			) {
				p.planAIGatewayPolicyUpdate(namespace, gatewayRef, gatewayID,
					resources.AIGatewayPolicyID(current.AIGatewayPolicy), desired, fields, changed, plan)
			},
			protected: func(current state.AIGatewayPolicy) bool {
				return labels.IsProtectedResource(current.NormalizedLabels)
			},
			remove: func(current state.AIGatewayPolicy) {
				p.planAIGatewayPolicyDelete(namespace, gatewayRef, gatewayID,
					resources.AIGatewayPolicyID(current.AIGatewayPolicy),
					resources.AIGatewayPolicyName(current.AIGatewayPolicy), plan)
			},
		}, plan)
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
	namespace string,
	gatewayRef string,
	_ string,
	gatewayID string,
	policy resources.AIGatewayPolicyResource,
	dependsOn []string,
	plan *Plan,
) {
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

func (p *Planner) planAIGatewayPolicyUpdate(
	namespace string,
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
		Namespace:     namespace,
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

	currentCompare, desiredCompare := normalizeAIGatewayPolicyPayloadsForComparison(currentPayload, desiredPayload)
	changedFields := diffAIGatewayPayloads(currentPayload, desiredPayload, currentCompare, desiredCompare)
	if len(changedFields) == 0 {
		return false, nil, nil, nil
	}

	return true, clonePayloadMap(desiredPayload), changedFields, nil
}

func aiGatewayPolicyCreateDependencies(plan *Plan, namespace string, gatewayRef string) map[string]string {
	if plan == nil {
		return nil
	}

	depsByRefOrName := make(map[string]string)
	for _, change := range plan.Changes {
		if change.Action != ActionCreate ||
			change.ResourceType != ResourceTypeAIGatewayPolicy ||
			change.Namespace != namespace ||
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
