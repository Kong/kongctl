package planner

import (
	"context"
	"fmt"
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util"
)

func (p *Planner) resolveAIGatewayPolicyDeletes(
	ctx context.Context, namespace, gatewayRef, gatewayID string, plan *Plan,
) error {
	references := func(deletion PlannedChange, values []string) bool {
		name, _ := deletion.Fields[FieldName].(string)
		return slices.ContainsFunc(values, func(value string) bool {
			value = canonicalAIGatewayReference(value, nil)
			return value != "" && (value == name || value == deletion.ResourceID)
		})
	}
	return resolveObservedReferenceDeletes(ctx, namespace, gatewayRef, plan, observedReferenceDeletePolicy{
		targetType: ResourceTypeAIGatewayPolicy, usesTarget: aiGatewayResourceUsesPolicies,
		observe: func(ctx context.Context) ([]observedReferenceUser, error) {
			users, err := p.listAIGatewayPolicyUsers(ctx, gatewayID)
			if err != nil {
				return nil, fmt.Errorf("failed to inspect policy attachments in AI Gateway %q: %w", gatewayRef, err)
			}
			return users, nil
		},
		observedReferences: references,
		plannedReferences: func(deletion, change PlannedChange) bool {
			return references(deletion, stringSliceFromField(change.Fields, FieldPolicies))
		},
		updateDetaches: func(deletion, change PlannedChange) bool {
			return aiGatewayPolicyUpdateDetaches(
				change,
				func(values []string) bool { return references(deletion, values) },
			)
		},
		conflict: func(deletion PlannedChange, user observedReferenceUser, planned bool) error {
			qualifier := ""
			if planned {
				qualifier = "planned "
			}
			return fmt.Errorf(
				"cannot delete AI Gateway Policy %q in gateway %q while %s%s %q still references it",
				deletion.Fields[FieldName],
				gatewayRef,
				qualifier,
				user.resourceType,
				user.name,
			)
		},
	})
}

func aiGatewayResourceUsesPolicies(resourceType string) bool {
	switch resourceType {
	case ResourceTypeAIGatewayAgent, ResourceTypeAIGatewayConsumer, ResourceTypeAIGatewayConsumerGroup,
		ResourceTypeAIGatewayModel, ResourceTypeAIGatewayMCPServer:
		return true
	default:
		return false
	}
}

func aiGatewayPolicyUpdateDetaches(change PlannedChange, referencesPolicy func([]string) bool) bool {
	if raw, present := change.Fields[FieldPolicies]; present {
		policies, valid := util.StringSliceFromAny(raw)
		return valid && !referencesPolicy(policies)
	}
	// SDK omitempty removes empty policies from the desired PUT payload. The
	// diff records that removal; payload absence alone does not establish it.
	field, changed := change.ChangedFields[FieldPolicies]
	return changed && field.New == nil
}

func (p *Planner) listAIGatewayPolicyUsers(ctx context.Context, gatewayID string) ([]observedReferenceUser, error) {
	var users []observedReferenceUser
	agents, err := p.listAIGatewayAgents(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	for _, agent := range agents {
		users = append(users, observedReferenceUser{ResourceTypeAIGatewayAgent, agent.ID, agent.Name, agent.Policies})
	}
	consumers, err := p.listAIGatewayConsumers(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list consumers: %w", err)
	}
	for _, consumer := range consumers {
		users = append(users, observedReferenceUser{
			ResourceTypeAIGatewayConsumer, consumer.ID, consumer.Name, consumer.Policies,
		})
	}
	groups, err := p.listAIGatewayConsumerGroups(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list consumer groups: %w", err)
	}
	for _, group := range groups {
		users = append(
			users,
			observedReferenceUser{ResourceTypeAIGatewayConsumerGroup, group.ID, group.Name, group.Policies},
		)
	}
	models, err := p.listAIGatewayModels(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	for _, model := range models {
		payload, err := resources.AIGatewayModelMutablePayloadMap(model.AIGatewayModel)
		if err != nil {
			return nil, fmt.Errorf("inspect model policy attachments: %w", err)
		}
		users = append(users, observedReferenceUser{
			ResourceTypeAIGatewayModel, resources.AIGatewayModelID(model.AIGatewayModel),
			resources.AIGatewayModelName(model.AIGatewayModel), stringSliceFromField(payload, FieldPolicies),
		})
	}
	servers, err := p.listAIGatewayMCPServers(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list MCP servers: %w", err)
	}
	for _, server := range servers {
		payload, err := resources.AIGatewayMCPServerMutablePayloadMap(server.AIGatewayMCPServer)
		if err != nil {
			return nil, fmt.Errorf("inspect MCP server policy attachments: %w", err)
		}
		users = append(users, observedReferenceUser{
			ResourceTypeAIGatewayMCPServer, resources.AIGatewayMCPServerID(server.AIGatewayMCPServer),
			resources.AIGatewayMCPServerName(server.AIGatewayMCPServer), stringSliceFromField(payload, FieldPolicies),
		})
	}
	return users, nil
}
