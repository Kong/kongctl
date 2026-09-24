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

	current, err := p.listAIGatewayModels(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to inspect AI Gateway models before deleting providers: %w", err)
	}

	type modelProviderReferences struct {
		id, name  string
		providers []string
	}
	var plannedModels []modelProviderReferences
	for _, change := range plan.Changes {
		if change.ResourceType != ResourceTypeAIGatewayModel ||
			(change.Action != ActionCreate && change.Action != ActionUpdate) ||
			change.Namespace != namespace || !aiGatewayChildChangeMatchesParent(change, gatewayRef) {
			continue
		}
		plannedModels = append(plannedModels, modelProviderReferences{
			name: change.ResourceRef, providers: aiGatewayModelProviderNames(change.Fields),
		})
	}
	observedModels := make([]modelProviderReferences, 0, len(current))
	for _, model := range current {
		payload, err := resources.AIGatewayModelMutablePayloadMap(model.AIGatewayModel)
		if err != nil {
			return fmt.Errorf("failed to inspect AI Gateway model %q in gateway %q before deleting providers: %w",
				resources.AIGatewayModelName(model.AIGatewayModel), gatewayRef, err)
		}
		observedModels = append(observedModels, modelProviderReferences{
			id:        resources.AIGatewayModelID(model.AIGatewayModel),
			name:      resources.AIGatewayModelName(model.AIGatewayModel),
			providers: aiGatewayModelProviderNames(payload),
		})
	}
	for _, index := range deletes {
		deletion := &plan.Changes[index]
		providerName, _ := deletion.Fields[FieldName].(string)
		for _, model := range plannedModels {
			if slices.Contains(model.providers, providerName) {
				return fmt.Errorf(
					"cannot delete AI Gateway Model Provider %q in gateway %q while planned model %q still references it",
					providerName, gatewayRef, model.name,
				)
			}
		}
		for _, model := range observedModels {
			if !slices.Contains(model.providers, providerName) {
				continue
			}
			change := modelChanges[model.id]
			if change == nil || (change.Action != ActionDelete && change.Action != ActionUpdate) {
				return fmt.Errorf(
					"cannot delete AI Gateway Model Provider %q in gateway %q while model %q still references it",
					providerName, gatewayRef, model.name,
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
