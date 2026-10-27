package planner

import (
	"context"
	"fmt"
	"slices"

	"github.com/kong/kongctl/internal/declarative/tags"
)

func aiGatewayResourceUsesDatastores(kind string) bool {
	switch kind {
	case ResourceTypeAIGatewayProvider,
		ResourceTypeAIGatewayAuthStrategy,
		ResourceTypeAIGatewayMCPServer,
		ResourceTypeAIGatewayPolicy,
		ResourceTypeAIGatewayModel:
		return true
	}
	return false
}

func (p *Planner) resolveAIGatewayDatastoreDeletes(
	ctx context.Context,
	namespace, gatewayRef, gatewayID string,
	plan *Plan,
) error {
	storeNames := map[string]string{}
	if p.resources != nil {
		for _, store := range p.resources.GetAIGatewayDatastoresForGateway(gatewayRef) {
			storeNames[store.Ref] = store.Name()
		}
	}
	references := func(deletion PlannedChange, names []string) bool {
		name, _ := deletion.Fields[FieldName].(string)
		return slices.Contains(names, name) || slices.Contains(names, deletion.ResourceID)
	}
	plannedReferences := func(deletion, change PlannedChange) bool {
		var names []string
		fields := normalizeAIGatewayJSONMap(change.Fields)
		stores, _ := fields[FieldDatastores].([]any)
		for _, value := range stores {
			store, _ := value.(map[string]any)
			name, _ := store[FieldName].(string)
			if ref, _, ok := tags.ParseRefPlaceholder(name); ok && storeNames[ref] != "" {
				name = storeNames[ref]
			}
			names = append(names, name)
		}
		return references(deletion, names)
	}
	return resolveObservedReferenceDeletes(
		ctx,
		namespace,
		gatewayRef,
		plan,
		observedReferenceDeletePolicy{
			gatewayID: gatewayID,
			targetType: ResourceTypeAIGatewayDatastore, usesTarget: aiGatewayResourceUsesDatastores,
			observe: func(ctx context.Context) ([]observedReferenceUser, error) {
				var users []observedReferenceUser
				for _, change := range plan.Changes {
					if change.ResourceType != ResourceTypeAIGatewayDatastore ||
						change.Action != ActionDelete ||
						!aiGatewayChangeMatchesGateway(change, gatewayRef, gatewayID) {
						continue
					}
					usage, err := p.client.ListAIGatewayDatastoreUsage(
						ctx,
						gatewayID,
						change.ResourceID,
					)
					if err != nil {
						return nil, err
					}
					kinds := map[string]string{
						"policy": ResourceTypeAIGatewayPolicy, "model": ResourceTypeAIGatewayModel,
						"model_provider": ResourceTypeAIGatewayProvider,
						"auth_strategy":  ResourceTypeAIGatewayAuthStrategy,
						"mcp_server":     ResourceTypeAIGatewayMCPServer,
					}
					for _, entry := range usage {
						kind := kinds[string(entry.Type)]
						if kind == "" {
							return nil, fmt.Errorf(
								"cannot inspect unknown datastore usage type %q",
								entry.Type,
							)
						}
						users = append(
							users,
							observedReferenceUser{
								resourceType: kind,
								id:           entry.ID,
								name:         entry.Name,
								references:   []string{change.ResourceID},
							},
						)
					}
				}
				return users, nil
			},
			observedReferences: references, plannedReferences: plannedReferences,
			updateDetaches: func(deletion, change PlannedChange) bool {
				// These resources replace their writable payload on PUT. Require a
				// recorded datastore change as evidence of detachment.
				if _, changed := change.ChangedFields[FieldDatastores]; !changed {
					return false
				}
				return !plannedReferences(deletion, change)
			},
			conflict: func(deletion PlannedChange, user observedReferenceUser, _ bool) error {
				return fmt.Errorf(
					"cannot delete AI Gateway Datastore %q while %s %q still references it",
					deletion.Fields[FieldName],
					user.resourceType,
					user.name,
				)
			},
		},
	)
}
