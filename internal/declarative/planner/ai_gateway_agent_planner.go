package planner

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/util"
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

	currentByID, currentByName := indexAIGatewayAgents(currentAgents)
	desiredKeys := make(map[string]bool)

	for _, desiredAgent := range desired {
		current, exists := matchCurrentAIGatewayAgent(desiredAgent, currentByID, currentByName)
		desiredKeys[desiredAgent.Name] = true
		if id := aiGatewayAgentDesiredID(desiredAgent); id != "" {
			desiredKeys[id] = true
		}

		if !exists {
			dependsOn := aiGatewayAgentPolicyCreateDependencies(
				desiredAgent,
				policyCreateDepsByRefOrName,
			)
			p.planAIGatewayAgentCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredAgent, dependsOn, plan)
			continue
		}

		agentID := resources.AIGatewayAgentID(current.AIGatewayAgent)
		if agent := p.resources.GetAIGatewayAgentByRef(desiredAgent.Ref); agent != nil {
			agent.SetKonnectID(agentID)
		}
		fullAgent, err := p.client.GetAIGatewayAgent(ctx, gatewayID, agentID)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway Agent %s: %w", agentID, err)
		}
		if fullAgent == nil {
			dependsOn := aiGatewayAgentPolicyCreateDependencies(
				desiredAgent,
				policyCreateDepsByRefOrName,
			)
			p.planAIGatewayAgentCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredAgent, dependsOn, plan)
			continue
		}

		needsUpdate, updateFields, changedFields, err := p.shouldUpdateAIGatewayAgent(*fullAgent, desiredAgent)
		if err != nil {
			return err
		}
		if needsUpdate {
			p.planAIGatewayAgentUpdate(
				namespace,
				gatewayRef,
				gatewayID,
				agentID,
				desiredAgent,
				updateFields,
				changedFields,
				aiGatewayAgentPolicyCreateDependencies(
					desiredAgent,
					policyCreateDepsByRefOrName,
				),
				plan,
			)
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		for _, current := range currentAgents {
			agentID := resources.AIGatewayAgentID(current.AIGatewayAgent)
			agentName := resources.AIGatewayAgentName(current.AIGatewayAgent)
			if desiredKeys[agentID] || desiredKeys[agentName] {
				continue
			}
			p.planAIGatewayAgentDelete(gatewayRef, gatewayID, agentID, agentName, plan)
		}
	}

	return nil
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
	currentCompare, desiredCompare = normalizeAIGatewayPolicyReferencesForComparison(
		currentCompare,
		desiredCompare,
		p.resources,
	)

	changedFields := diffAIGatewayPayloads(currentPayload, desiredPayload, currentCompare, desiredCompare)
	if len(changedFields) == 0 {
		return false, nil, nil, nil
	}

	return true, clonePayloadMap(desiredPayload), changedFields, nil
}

func indexAIGatewayAgents(
	agents []state.AIGatewayAgent,
) (map[string]state.AIGatewayAgent, map[string]state.AIGatewayAgent) {
	byID := make(map[string]state.AIGatewayAgent)
	byName := make(map[string]state.AIGatewayAgent)
	for _, agent := range agents {
		if id := resources.AIGatewayAgentID(agent.AIGatewayAgent); id != "" {
			byID[id] = agent
		}
		if name := resources.AIGatewayAgentName(agent.AIGatewayAgent); name != "" {
			byName[name] = agent
		}
	}
	return byID, byName
}

func matchCurrentAIGatewayAgent(
	desired resources.AIGatewayAgentResource,
	currentByID map[string]state.AIGatewayAgent,
	currentByName map[string]state.AIGatewayAgent,
) (state.AIGatewayAgent, bool) {
	if id := aiGatewayAgentDesiredID(desired); id != "" {
		current, exists := currentByID[id]
		return current, exists
	}
	current, exists := currentByName[desired.Name]
	return current, exists
}

func aiGatewayAgentDesiredID(desired resources.AIGatewayAgentResource) string {
	if id := desired.GetKonnectID(); id != "" {
		return id
	}
	if util.IsValidUUID(desired.Ref) {
		return desired.Ref
	}
	return ""
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
