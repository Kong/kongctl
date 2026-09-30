package planner

import (
	"context"
	"fmt"
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
)

// Inspect all models, including models outside the selected sync scope.
func (p *Planner) resolveAIGatewayProviderDeletes(
	ctx context.Context, namespace, gatewayRef, gatewayID string, plan *Plan,
) error {
	references := func(deletion PlannedChange, values []string) bool {
		name, _ := deletion.Fields[FieldName].(string)
		return slices.Contains(values, name)
	}
	return resolveObservedReferenceDeletes(ctx, namespace, gatewayRef, plan, observedReferenceDeletePolicy{
		targetType: ResourceTypeAIGatewayProvider,
		usesTarget: func(kind string) bool { return kind == ResourceTypeAIGatewayModel },
		observe: func(ctx context.Context) ([]observedReferenceUser, error) {
			current, err := p.listAIGatewayModels(ctx, gatewayID)
			if err != nil {
				return nil, fmt.Errorf("failed to inspect AI Gateway models before deleting providers: %w", err)
			}
			users := make([]observedReferenceUser, 0, len(current))
			for _, model := range current {
				payload, err := resources.AIGatewayModelMutablePayloadMap(model.AIGatewayModel)
				if err != nil {
					return nil, fmt.Errorf(
						"failed to inspect AI Gateway model %q in gateway %q before deleting providers: %w",
						resources.AIGatewayModelName(model.AIGatewayModel),
						gatewayRef,
						err,
					)
				}
				users = append(
					users,
					observedReferenceUser{
						ResourceTypeAIGatewayModel,
						resources.AIGatewayModelID(model.AIGatewayModel),
						resources.AIGatewayModelName(model.AIGatewayModel),
						aiGatewayModelProviderNames(payload),
					},
				)
			}
			return users, nil
		},
		observedReferences: references,
		plannedReferences: func(deletion, change PlannedChange) bool {
			return references(deletion, aiGatewayModelProviderNames(change.Fields))
		},
		// Model updates are full mutable payloads with required targets. Their
		// provider references include both targets and semantic embeddings.
		updateDetaches: func(deletion, change PlannedChange) bool {
			_, present := change.Fields[FieldTargets]
			return present && !references(deletion, aiGatewayModelProviderNames(change.Fields))
		},
		conflict: func(deletion PlannedChange, user observedReferenceUser, planned bool) error {
			qualifier := ""
			if planned {
				qualifier = "planned "
			}
			return fmt.Errorf(
				"cannot delete AI Gateway Model Provider %q in gateway %q while %smodel %q still references it",
				deletion.Fields[FieldName],
				gatewayRef,
				qualifier,
				user.name,
			)
		},
	})
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
