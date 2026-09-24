package planner

import (
	"context"
	"fmt"
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
)

type aiGatewayPolicyUser struct {
	resourceType string
	id           string
	name         string
	policies     []string
}

// Inspect every policy consumer, including children outside the sync scope.
// A policy can only be removed after all existing attachments are removed.
func (p *Planner) resolveAIGatewayPolicyDeletes(
	ctx context.Context, namespace, gatewayRef, gatewayID string, plan *Plan,
) error {
	var deletes []int
	type resourceKey struct{ resourceType, id string }
	changes := make(map[resourceKey]*PlannedChange)
	var plannedUsers []*PlannedChange
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.Namespace != namespace || !aiGatewayChildChangeMatchesParent(*change, gatewayRef) {
			continue
		}
		if change.ResourceType == ResourceTypeAIGatewayPolicy && change.Action == ActionDelete {
			deletes = append(deletes, i)
		}
		if !aiGatewayResourceUsesPolicies(change.ResourceType) {
			continue
		}
		changes[resourceKey{change.ResourceType, change.ResourceID}] = change
		if change.Action == ActionCreate || change.Action == ActionUpdate {
			plannedUsers = append(plannedUsers, change)
		}
	}
	if len(deletes) == 0 {
		return nil
	}

	users, err := p.listAIGatewayPolicyUsers(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to inspect policy attachments in AI Gateway %q: %w", gatewayRef, err)
	}
	for _, index := range deletes {
		deletion := &plan.Changes[index]
		policyName, _ := deletion.Fields[FieldName].(string)
		referencesPolicy := func(policies []string) bool {
			return slices.ContainsFunc(policies, func(value string) bool {
				value = canonicalAIGatewayReference(value, nil)
				return value != "" && (value == policyName || value == deletion.ResourceID)
			})
		}
		for _, change := range plannedUsers {
			if referencesPolicy(aiGatewayPolicyNames(change.Fields)) {
				return fmt.Errorf(
					"cannot delete AI Gateway Policy %q in gateway %q while planned %s %q still references it",
					policyName, gatewayRef, change.ResourceType, change.ResourceRef,
				)
			}
		}
		for _, user := range users {
			if !referencesPolicy(user.policies) {
				continue
			}
			change := changes[resourceKey{user.resourceType, user.id}]
			if change != nil && change.Action == ActionDelete {
				deletion.DependsOn = appendDependsOn(deletion.DependsOn, change.ID)
				continue
			}
			if change != nil && change.Action == ActionUpdate {
				// An unrelated update that omits policies does not detach them.
				if policies, present := change.Fields[FieldPolicies]; present && policies != nil &&
					!referencesPolicy(aiGatewayPolicyNames(change.Fields)) {
					deletion.DependsOn = appendDependsOn(deletion.DependsOn, change.ID)
					continue
				}
			}
			return fmt.Errorf(
				"cannot delete AI Gateway Policy %q in gateway %q while %s %q still references it",
				policyName, gatewayRef, user.resourceType, user.name,
			)
		}
	}
	return nil
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

func aiGatewayPolicyNames(payload map[string]any) []string {
	if values, ok := payload[FieldPolicies].([]string); ok {
		return values
	}
	values, _ := payload[FieldPolicies].([]any)
	var policies []string
	for _, value := range values {
		if policy, ok := value.(string); ok {
			policies = append(policies, policy)
		}
	}
	return policies
}

func (p *Planner) listAIGatewayPolicyUsers(ctx context.Context, gatewayID string) ([]aiGatewayPolicyUser, error) {
	var users []aiGatewayPolicyUser
	agents, err := p.client.ListAIGatewayAgents(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	for _, agent := range agents {
		users = append(users, aiGatewayPolicyUser{ResourceTypeAIGatewayAgent, agent.ID, agent.Name, agent.Policies})
	}
	consumers, err := p.client.ListAIGatewayConsumers(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list consumers: %w", err)
	}
	for _, consumer := range consumers {
		users = append(users, aiGatewayPolicyUser{
			ResourceTypeAIGatewayConsumer, consumer.ID, consumer.Name, consumer.Policies,
		})
	}
	groups, err := p.client.ListAIGatewayConsumerGroups(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list consumer groups: %w", err)
	}
	for _, group := range groups {
		users = append(users, aiGatewayPolicyUser{ResourceTypeAIGatewayConsumerGroup, group.ID, group.Name, group.Policies})
	}
	models, err := p.client.ListAIGatewayModels(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	for _, model := range models {
		payload, err := resources.AIGatewayModelMutablePayloadMap(model.AIGatewayModel)
		if err != nil {
			return nil, fmt.Errorf("inspect model policy attachments: %w", err)
		}
		users = append(users, aiGatewayPolicyUser{
			ResourceTypeAIGatewayModel, resources.AIGatewayModelID(model.AIGatewayModel),
			resources.AIGatewayModelName(model.AIGatewayModel), aiGatewayPolicyNames(payload),
		})
	}
	servers, err := p.client.ListAIGatewayMCPServers(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list MCP servers: %w", err)
	}
	for _, server := range servers {
		payload, err := resources.AIGatewayMCPServerMutablePayloadMap(server.AIGatewayMCPServer)
		if err != nil {
			return nil, fmt.Errorf("inspect MCP server policy attachments: %w", err)
		}
		users = append(users, aiGatewayPolicyUser{
			ResourceTypeAIGatewayMCPServer, resources.AIGatewayMCPServerID(server.AIGatewayMCPServer),
			resources.AIGatewayMCPServerName(server.AIGatewayMCPServer), aiGatewayPolicyNames(payload),
		})
	}
	return users, nil
}
