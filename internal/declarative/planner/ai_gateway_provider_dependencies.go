package planner

import (
	"context"
	"fmt"
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
)

// Inspect all observed models when pruning providers, even if models are outside
// this sync's scope. A retained or protected model must not lose its provider.
func (p *Planner) resolveAIGatewayProviderDeletes(
	ctx context.Context, namespace, gatewayRef, gatewayID string, plan *Plan,
) error {
	var deletes []int
	modelChanges := make(map[string]*PlannedChange)
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.Namespace != namespace || !aiGatewayChildChangeMatchesParent(*change, gatewayRef) {
			continue
		}
		if change.ResourceType == ResourceTypeAIGatewayProvider && change.Action == ActionDelete {
			deletes = append(deletes, i)
		}
		if change.ResourceType == ResourceTypeAIGatewayModel && change.ResourceID != "" {
			modelChanges[change.ResourceID] = change
		}
	}
	if len(deletes) == 0 {
		return nil
	}

	current, err := p.client.ListAIGatewayModels(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to inspect AI Gateway models before deleting providers: %w", err)
	}
	for _, index := range deletes {
		deletion := &plan.Changes[index]
		providerName, _ := deletion.Fields[FieldName].(string)
		for _, change := range plan.Changes {
			if change.ResourceType != ResourceTypeAIGatewayModel ||
				(change.Action != ActionCreate && change.Action != ActionUpdate) ||
				change.Namespace != namespace || !aiGatewayChildChangeMatchesParent(change, gatewayRef) {
				continue
			}
			if slices.Contains(aiGatewayModelProviderNames(change.Fields), providerName) {
				return fmt.Errorf(
					"cannot delete AI Gateway Model Provider %q in gateway %q while planned model %q still references it",
					providerName,
					gatewayRef,
					change.ResourceRef,
				)
			}
		}
		for _, model := range current {
			payload, err := resources.AIGatewayModelMutablePayloadMap(model.AIGatewayModel)
			if err != nil {
				return fmt.Errorf("failed to inspect AI Gateway model %q before deleting provider %q: %w",
					resources.AIGatewayModelName(model.AIGatewayModel), providerName, err)
			}
			if !slices.Contains(aiGatewayModelProviderNames(payload), providerName) {
				continue
			}
			change := modelChanges[resources.AIGatewayModelID(model.AIGatewayModel)]
			if change == nil || (change.Action != ActionDelete && change.Action != ActionUpdate) {
				return fmt.Errorf(
					"cannot delete AI Gateway Model Provider %q in gateway %q while model %q still references it",
					providerName,
					gatewayRef,
					resources.AIGatewayModelName(model.AIGatewayModel),
				)
			}
			deletion.DependsOn = appendDependsOn(deletion.DependsOn, change.ID)
		}
	}
	return nil
}

// Providers are named in target models and in semantic-balancer embeddings.
func aiGatewayModelProviderNames(payload map[string]any) []string {
	var names []string
	add := func(value any) {
		if name, ok := value.(string); ok && name != "" && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	targets, _ := payload[FieldTargets].([]any)
	for _, value := range targets {
		if target, ok := value.(map[string]any); ok {
			add(target[FieldProvider])
		}
	}
	config, _ := payload[FieldConfig].(map[string]any)
	balancer, _ := config[FieldBalancer].(map[string]any)
	embeddings, _ := balancer[FieldEmbeddings].(map[string]any)
	add(embeddings[FieldProvider])
	return names
}
