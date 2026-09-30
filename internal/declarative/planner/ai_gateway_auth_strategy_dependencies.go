package planner

import (
	"context"
	"fmt"
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util"
)

func (p *Planner) resolveAIGatewayAuthStrategyDeletes(
	ctx context.Context, namespace, gatewayRef, gatewayID string, plan *Plan,
) error {
	namesByRef := make(map[string]string)
	if p.resources != nil {
		for _, strategy := range p.resources.GetAIGatewayAuthStrategiesForGateway(gatewayRef) {
			namesByRef[strategy.Ref] = strategy.Name
		}
	}
	for _, change := range plan.Changes {
		if change.Namespace == namespace && aiGatewayChildChangeMatchesParent(change, gatewayRef) &&
			change.ResourceType == ResourceTypeAIGatewayAuthStrategy && change.Action == ActionCreate {
			if name, ok := change.Fields[FieldName].(string); ok {
				namesByRef[change.ResourceRef] = name
			}
		}
	}
	references := func(deletion PlannedChange, values []string) bool {
		name, _ := deletion.Fields[FieldName].(string)
		return slices.ContainsFunc(values, func(value string) bool {
			value = canonicalAIGatewayReference(value, nil)
			return value != "" && (value == name || value == deletion.ResourceID)
		})
	}
	plannedReferences := func(deletion PlannedChange, values []string) bool {
		resolved := make([]string, len(values))
		for i, value := range values {
			if ref, _, ok := tags.ParseRefPlaceholder(value); ok && namesByRef[ref] != "" {
				value = namesByRef[ref]
			}
			resolved[i] = value
		}
		return references(deletion, resolved)
	}
	return resolveObservedReferenceDeletes(ctx, namespace, gatewayRef, plan, observedReferenceDeletePolicy{
		targetType: ResourceTypeAIGatewayAuthStrategy, usesTarget: aiGatewayResourceUsesAuthStrategies,
		observe: func(ctx context.Context) ([]observedReferenceUser, error) {
			users, err := p.listAIGatewayAuthStrategyUsers(ctx, gatewayID)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to inspect auth strategy attachments in AI Gateway %q: %w",
					gatewayRef,
					err,
				)
			}
			return users, nil
		},
		observedReferences: references,
		plannedReferences: func(deletion, change PlannedChange) bool {
			return plannedReferences(deletion, aiGatewayAuthStrategies(change.Fields))
		},
		updateDetaches: func(deletion, change PlannedChange) bool {
			return aiGatewayAuthStrategyUpdateDetaches(
				change,
				func(values []string) bool { return plannedReferences(deletion, values) },
			)
		},
		conflict: func(deletion PlannedChange, user observedReferenceUser, planned bool) error {
			qualifier := ""
			if planned {
				qualifier = "planned "
			}
			return fmt.Errorf(
				"cannot delete AI Gateway Auth Strategy %q in gateway %q while %s%s %q still references it",
				deletion.Fields[FieldName],
				gatewayRef,
				qualifier,
				user.resourceType,
				user.name,
			)
		},
	})
}

func aiGatewayResourceUsesAuthStrategies(resourceType string) bool {
	switch resourceType {
	case ResourceTypeAIGatewayAgent, ResourceTypeAIGatewayModel, ResourceTypeAIGatewayMCPServer:
		return true
	default:
		return false
	}
}

func aiGatewayAuthStrategies(payload map[string]any) []string {
	access, _ := payload[FieldAccess].(map[string]any)
	return stringSliceFromField(access, FieldAuthStrategies)
}

func aiGatewayAuthStrategyUpdateDetaches(change PlannedChange, referencesAuthStrategy func([]string) bool) bool {
	if access, ok := change.Fields[FieldAccess].(map[string]any); ok {
		if raw, present := access[FieldAuthStrategies]; present {
			strategies, valid := util.StringSliceFromAny(raw)
			return valid && !referencesAuthStrategy(strategies)
		}
	}
	// SDK omitempty can remove the empty list or the entire access object.
	// Require the diff to establish removal, rather than payload absence alone.
	field, changed := change.ChangedFields[FieldAccess]
	if !changed {
		return false
	}
	if field.New == nil {
		return true
	}
	access, valid := field.New.(map[string]any)
	if !valid {
		return false
	}
	_, present := access[FieldAuthStrategies]
	return !present
}

func (p *Planner) listAIGatewayAuthStrategyUsers(
	ctx context.Context,
	gatewayID string,
) ([]observedReferenceUser, error) {
	var users []observedReferenceUser
	agents, err := p.listAIGatewayAgents(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	for _, agent := range agents {
		payload, err := resources.AIGatewayAgentMutablePayloadMap(agent.AIGatewayAgent)
		if err != nil {
			return nil, fmt.Errorf("inspect agent auth strategy attachments: %w", err)
		}
		users = append(users, observedReferenceUser{
			ResourceTypeAIGatewayAgent, agent.ID, agent.Name, aiGatewayAuthStrategies(payload),
		})
	}
	models, err := p.listAIGatewayModels(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	for _, model := range models {
		payload, err := resources.AIGatewayModelMutablePayloadMap(model.AIGatewayModel)
		if err != nil {
			return nil, fmt.Errorf("inspect model auth strategy attachments: %w", err)
		}
		users = append(users, observedReferenceUser{
			ResourceTypeAIGatewayModel, resources.AIGatewayModelID(model.AIGatewayModel),
			resources.AIGatewayModelName(model.AIGatewayModel), aiGatewayAuthStrategies(payload),
		})
	}
	servers, err := p.listAIGatewayMCPServers(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list MCP servers: %w", err)
	}
	for _, server := range servers {
		payload, err := resources.AIGatewayMCPServerMutablePayloadMap(server.AIGatewayMCPServer)
		if err != nil {
			return nil, fmt.Errorf("inspect MCP server auth strategy attachments: %w", err)
		}
		users = append(users, observedReferenceUser{
			ResourceTypeAIGatewayMCPServer, resources.AIGatewayMCPServerID(server.AIGatewayMCPServer),
			resources.AIGatewayMCPServerName(server.AIGatewayMCPServer), aiGatewayAuthStrategies(payload),
		})
	}
	return users, nil
}
