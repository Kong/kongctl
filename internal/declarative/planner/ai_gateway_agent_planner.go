package planner

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

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

	currentAgents, err := p.listAIGatewayAgents(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway Agents for gateway %s: %w", gatewayID, err)
	}

	return p.reconcileOwnedAIGatewayAgents(
		ctx,
		gatewayRef,
		gatewayName,
		gatewayID,
		desired,
		currentAgents,
		policyCreateDepsByRefOrName,
		plan,
	)
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
	_ string,
	gatewayRef string,
	_ string,
	gatewayID string,
	agent resources.AIGatewayAgentResource,
	dependsOn []string,
	plan *Plan,
) {
	agent.ManagedBy = resources.BuildChildManagedBy(
		resources.GetNamespace(agent.Kongctl),
		agent.Protected(),
		"",
	)
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
		Namespace:    resources.GetNamespace(agent.Kongctl),
		Protection:   agent.Protected(),
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
	_ string,
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
		Namespace:     resources.GetNamespace(agent.Kongctl),
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

	if desired.Labels == nil {
		desiredPayload[FieldLabels] = currentPayload[FieldLabels]
	} else {
		desiredPayload[FieldLabels] = desired.Labels
	}
	currentCompare, desiredCompare := normalizeAIGatewayPayloadsForComparison(currentPayload, desiredPayload)
	currentCompare = scrubAIGatewayAgentWriteOnlyFields(currentCompare)
	desiredCompare = scrubAIGatewayAgentWriteOnlyFields(desiredCompare)
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

func scrubAIGatewayAgentWriteOnlyFields(payload map[string]any) map[string]any {
	result := normalizeAIGatewayJSONMap(payload)
	config, _ := result[FieldConfig].(map[string]any)
	upstream, _ := config["upstream"].(map[string]any)
	if auth, ok := upstream["auth"].(map[string]any); ok {
		upstream["auth"] = scrubAIGatewayUpstreamWriteOnlyFields(auth)
	}
	proxy, _ := config["proxy"].(map[string]any)
	if auth, ok := proxy["auth"].(map[string]any); ok {
		proxy["auth"] = scrubAIGatewayWriteOnlyFields(auth, func(key string) bool { return key == "password" })
	}
	return result
}
