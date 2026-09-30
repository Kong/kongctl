package planner

import (
	"context"
	"fmt"
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util"
)

type aiGatewayAuthStrategyUser struct {
	resourceType   string
	id             string
	name           string
	authStrategies []string
}

// Inspect every auth strategy user, including children outside the sync scope.
// A strategy can only be removed after all existing attachments are removed.
func (p *Planner) resolveAIGatewayAuthStrategyDeletes(
	ctx context.Context, namespace, gatewayRef, gatewayID string, plan *Plan,
) error {
	var deletes []int
	type resourceKey struct{ resourceType, id string }
	changes := make(map[resourceKey]*PlannedChange)
	var plannedUsers []*PlannedChange
	namesByRef := make(map[string]string)
	if p.resources != nil {
		for _, strategy := range p.resources.GetAIGatewayAuthStrategiesForGateway(gatewayRef) {
			namesByRef[strategy.Ref] = strategy.Name
		}
	}
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.Namespace != namespace || !aiGatewayChildChangeMatchesParent(*change, gatewayRef) {
			continue
		}
		if change.ResourceType == ResourceTypeAIGatewayAuthStrategy && change.Action == ActionDelete {
			deletes = append(deletes, i)
		}
		if change.ResourceType == ResourceTypeAIGatewayAuthStrategy && change.Action == ActionCreate {
			if name, ok := change.Fields[FieldName].(string); ok {
				namesByRef[change.ResourceRef] = name
			}
		}
		if !aiGatewayResourceUsesAuthStrategies(change.ResourceType) {
			continue
		}
		if change.ResourceID != "" {
			changes[resourceKey{change.ResourceType, change.ResourceID}] = change
		}
		if change.Action == ActionCreate || change.Action == ActionUpdate {
			plannedUsers = append(plannedUsers, change)
		}
	}
	if len(deletes) == 0 {
		return nil
	}

	users, err := p.listAIGatewayAuthStrategyUsers(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to inspect auth strategy attachments in AI Gateway %q: %w", gatewayRef, err)
	}
	for _, index := range deletes {
		deletion := &plan.Changes[index]
		authStrategyName, _ := deletion.Fields[FieldName].(string)
		referencesAuthStrategy := func(authStrategies []string) bool {
			return slices.ContainsFunc(authStrategies, func(value string) bool {
				value = canonicalAIGatewayReference(value, nil)
				return value != "" && (value == authStrategyName || value == deletion.ResourceID)
			})
		}
		plannedReferencesAuthStrategy := func(values []string) bool {
			resolved := make([]string, len(values))
			for i, value := range values {
				if ref, _, ok := tags.ParseRefPlaceholder(value); ok && namesByRef[ref] != "" {
					value = namesByRef[ref]
				}
				resolved[i] = value
			}
			return referencesAuthStrategy(resolved)
		}
		for _, change := range plannedUsers {
			if plannedReferencesAuthStrategy(aiGatewayAuthStrategies(change.Fields)) {
				return fmt.Errorf(
					"cannot delete AI Gateway Auth Strategy %q in gateway %q while planned %s %q still references it",
					authStrategyName, gatewayRef, change.ResourceType, change.ResourceRef,
				)
			}
		}
		for _, user := range users {
			if !referencesAuthStrategy(user.authStrategies) {
				continue
			}
			change := changes[resourceKey{user.resourceType, user.id}]
			if change != nil && change.Action == ActionDelete {
				deletion.DependsOn = appendDependsOn(deletion.DependsOn, change.ID)
				continue
			}
			if change != nil && change.Action == ActionUpdate &&
				aiGatewayAuthStrategyUpdateDetaches(*change, plannedReferencesAuthStrategy) {
				deletion.DependsOn = appendDependsOn(deletion.DependsOn, change.ID)
				continue
			}
			return fmt.Errorf(
				"cannot delete AI Gateway Auth Strategy %q in gateway %q while %s %q still references it",
				authStrategyName, gatewayRef, user.resourceType, user.name,
			)
		}
	}
	return nil
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
) ([]aiGatewayAuthStrategyUser, error) {
	var users []aiGatewayAuthStrategyUser
	agents, err := p.listAIGatewayAgents(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	for _, agent := range agents {
		payload, err := resources.AIGatewayAgentMutablePayloadMap(agent.AIGatewayAgent)
		if err != nil {
			return nil, fmt.Errorf("inspect agent auth strategy attachments: %w", err)
		}
		users = append(users, aiGatewayAuthStrategyUser{
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
		users = append(users, aiGatewayAuthStrategyUser{
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
		users = append(users, aiGatewayAuthStrategyUser{
			ResourceTypeAIGatewayMCPServer, resources.AIGatewayMCPServerID(server.AIGatewayMCPServer),
			resources.AIGatewayMCPServerName(server.AIGatewayMCPServer), aiGatewayAuthStrategies(payload),
		})
	}
	return users, nil
}
