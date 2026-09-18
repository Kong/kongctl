package planner

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) planAIGatewayAgentChanges(
	ctx context.Context,
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	gatewayChangeID string,
	policyCreateDepsByRefOrName map[string]string,
	desired []resources.AIGatewayAgentResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning AI Gateway Agent changes",
		slog.String("gateway_ref", gatewayRef),
		slog.String("gateway_id", gatewayID),
		slog.String("gateway_change_id", gatewayChangeID),
		slog.Int("desired_count", len(desired)),
	)

	if gatewayID == "" {
		p.planAIGatewayAgentCreatesForNewGateway(
			namespace,
			gatewayRef,
			gatewayName,
			gatewayChangeID,
			policyCreateDepsByRefOrName,
			desired,
			plan,
		)
		return nil
	}

	currentAgents, err := p.client.ListAIGatewayAgents(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway Agents for gateway %s: %w", gatewayID, err)
	}

	return reconcileNameMatchedChildren(p, ResourceTypeAIGatewayAgent, desired, currentAgents,
		nameMatchedChildOperations[resources.AIGatewayAgentResource, state.AIGatewayAgent]{
			desiredName: func(desired resources.AIGatewayAgentResource) string {
				return desired.Name
			},
			currentName: func(current state.AIGatewayAgent) string {
				return resources.AIGatewayAgentName(current.AIGatewayAgent)
			},
			fetch: func(desired resources.AIGatewayAgentResource, current state.AIGatewayAgent) (*state.AIGatewayAgent, error) {
				id := resources.AIGatewayAgentID(current.AIGatewayAgent)
				if agent := p.resources.GetAIGatewayAgentByRef(desired.Ref); agent != nil {
					agent.SetKonnectID(id)
				}
				full, err := p.client.GetAIGatewayAgent(ctx, gatewayID, id)
				if err != nil {
					return nil, fmt.Errorf("failed to get AI Gateway Agent %s: %w", id, err)
				}
				return full, nil
			},
			diff: p.shouldUpdateAIGatewayAgent,
			create: func(desired resources.AIGatewayAgentResource) {
				p.planAIGatewayAgentCreate(namespace, gatewayRef, gatewayName, gatewayID, desired,
					aiGatewayAgentPolicyCreateDependencies(desired, policyCreateDepsByRefOrName), plan)
			},
			update: func(current state.AIGatewayAgent, desired resources.AIGatewayAgentResource,
				fields map[string]any, changed map[string]FieldChange,
			) {
				p.planAIGatewayAgentUpdate(namespace, gatewayRef, gatewayID,
					resources.AIGatewayAgentID(current.AIGatewayAgent), desired, fields, changed,
					aiGatewayAgentPolicyCreateDependencies(desired, policyCreateDepsByRefOrName), plan)
			},
			protected: func(current state.AIGatewayAgent) bool {
				return labels.IsProtectedResource(current.NormalizedLabels)
			},
			remove: func(current state.AIGatewayAgent) {
				p.planAIGatewayAgentDelete(namespace, gatewayRef, gatewayID,
					resources.AIGatewayAgentID(current.AIGatewayAgent),
					resources.AIGatewayAgentName(current.AIGatewayAgent), plan)
			},
		}, plan)
}

func (p *Planner) planAIGatewayAgentCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	policyCreateDepsByRefOrName map[string]string,
	agents []resources.AIGatewayAgentResource,
	plan *Plan,
) {
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}
	for _, agent := range agents {
		agentDependsOn := slices.Clone(dependsOn)
		for _, dep := range aiGatewayAgentPolicyCreateDependencies(agent, policyCreateDepsByRefOrName) {
			agentDependsOn = appendDependsOn(agentDependsOn, dep)
		}
		p.planAIGatewayAgentCreate(namespace, gatewayRef, gatewayName, "", agent, agentDependsOn, plan)
	}
}

func (p *Planner) planAIGatewayAgentCreate(
	namespace string,
	gatewayRef string,
	_ string,
	gatewayID string,
	agent resources.AIGatewayAgentResource,
	dependsOn []string,
	plan *Plan,
) {
	fields, err := agent.MutablePayloadMap()
	if err != nil {
		plan.AddWarning(agent.GetRef(), fmt.Sprintf("failed to build AI Gateway Agent create payload: %s", err))
		return
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayAgent, agent.Ref),
		ResourceType: ResourceTypeAIGatewayAgent,
		ResourceRef:  agent.Ref,
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

func (p *Planner) planAIGatewayAgentUpdate(
	namespace string,
	gatewayRef string,
	gatewayID string,
	agentID string,
	agent resources.AIGatewayAgentResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	dependsOn []string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeAIGatewayAgent, agent.Ref),
		ResourceType:  ResourceTypeAIGatewayAgent,
		ResourceRef:   agent.Ref,
		ResourceID:    agentID,
		Action:        ActionUpdate,
		Fields:        updateFields,
		ChangedFields: changedFields,
		Namespace:     namespace,
		DependsOn:     dependsOn,
		Parent:        &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func (p *Planner) planAIGatewayAgentDelete(
	namespace string,
	gatewayRef string,
	gatewayID string,
	agentID string,
	agentName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGatewayAgent, agentName),
		ResourceType: ResourceTypeAIGatewayAgent,
		ResourceRef:  agentName,
		ResourceID:   agentID,
		Action:       ActionDelete,
		Namespace:    namespace,
		Fields: map[string]any{
			FieldName: agentName,
		},
		Parent: &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func (p *Planner) shouldUpdateAIGatewayAgent(
	current state.AIGatewayAgent,
	desired resources.AIGatewayAgentResource,
) (bool, map[string]any, map[string]FieldChange, error) {
	currentPayload, err := resources.AIGatewayAgentMutablePayloadMap(current.AIGatewayAgent)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to normalize current AI Gateway Agent: %w", err)
	}
	desiredPayload, err := desired.MutablePayloadMap()
	if err != nil {
		return false, nil, nil, fmt.Errorf(
			"failed to normalize desired AI Gateway Agent %q: %w",
			desired.Ref,
			err,
		)
	}

	currentCompare, desiredCompare := normalizeAIGatewayPayloadsForComparison(currentPayload, desiredPayload)
	currentCompare = scrubAIGatewayUpstreamWriteOnlyFields(currentCompare).(map[string]any)
	desiredCompare = scrubAIGatewayUpstreamWriteOnlyFields(desiredCompare).(map[string]any)
	updateFields := clonePayloadMap(desiredPayload)
	currentCompare, desiredCompare = normalizeAIGatewayPolicyReferencesForComparison(
		currentCompare,
		desiredCompare,
		p.resources,
	)
	currentCompare, desiredCompare = normalizeAIGatewayAuthStrategyReferencesForComparison(
		currentCompare,
		desiredCompare,
		p.resources,
	)

	changedFields := diffAIGatewayPayloads(currentPayload, desiredPayload, currentCompare, desiredCompare)
	if len(changedFields) == 0 {
		return false, nil, nil, nil
	}

	return true, updateFields, changedFields, nil
}

func aiGatewayAgentPolicyCreateDependencies(
	agent resources.AIGatewayAgentResource,
	policyCreateDepsByRefOrName map[string]string,
) []string {
	payload, err := agent.MutablePayloadMap()
	if err != nil {
		return nil
	}
	return aiGatewayPolicyReferenceDependencies(payload, policyCreateDepsByRefOrName)
}
