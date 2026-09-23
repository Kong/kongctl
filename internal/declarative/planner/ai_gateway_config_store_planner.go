package planner

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) planAIGatewayConfigStoreChanges(
	ctx context.Context,
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	gatewayChangeID string,
	desired []resources.AIGatewayConfigStoreResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning AI Gateway Config Store changes",
		slog.String("gateway_ref", gatewayRef),
		slog.String("gateway_id", gatewayID),
		slog.Int("desired_count", len(desired)),
	)
	if gatewayID == "" {
		var dependsOn []string
		if gatewayChangeID != "" {
			dependsOn = []string{gatewayChangeID}
		}
		for _, store := range desired {
			storeChangeID := p.planAIGatewayConfigStoreCreate(
				namespace, gatewayRef, gatewayName, "", store, dependsOn, plan,
			)
			if err := p.planAIGatewayConfigStoreSecretChangesIfNeeded(
				ctx, namespace, gatewayRef, "", store.Ref, "", storeChangeID, plan,
			); err != nil {
				return err
			}
		}
		return nil
	}

	currentStores, err := p.client.ListAIGatewayConfigStores(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway Config Stores for gateway %s: %w", gatewayID, err)
	}
	return reconcileNameMatchedCollection(p, ResourceTypeAIGatewayConfigStore, desired, currentStores,
		nameMatchedCollectionOperations[resources.AIGatewayConfigStoreResource, state.AIGatewayConfigStore]{
			desiredName: func(store resources.AIGatewayConfigStoreResource) string { return store.Name },
			currentName: func(store state.AIGatewayConfigStore) (string, bool) { return store.Name, true },
			reconcile: func(desiredStore resources.AIGatewayConfigStoreResource, current *state.AIGatewayConfigStore) error {
				if current == nil {
					storeChangeID := p.planAIGatewayConfigStoreCreate(
						namespace, gatewayRef, gatewayName, gatewayID, desiredStore, nil, plan,
					)
					return p.planAIGatewayConfigStoreSecretChangesIfNeeded(
						ctx, namespace, gatewayRef, gatewayID, desiredStore.Ref, "", storeChangeID, plan,
					)
				}
				if resource := p.resources.GetAIGatewayConfigStoreByRef(desiredStore.Ref); resource != nil {
					resource.SetKonnectID(current.ID)
				}
				if desiredStore.DisplayName != nil &&
					!stringPointersEqual(desiredStore.DisplayName, current.DisplayName) {
					fields := map[string]any{FieldDisplayName: *desiredStore.DisplayName}
					var oldDisplayName any
					if current.DisplayName != nil {
						oldDisplayName = *current.DisplayName
					}
					changed := map[string]FieldChange{
						FieldDisplayName: {Old: oldDisplayName, New: *desiredStore.DisplayName},
					}
					plan.AddChange(PlannedChange{
						ID:            p.nextChangeID(ActionUpdate, ResourceTypeAIGatewayConfigStore, desiredStore.Ref),
						ResourceType:  ResourceTypeAIGatewayConfigStore,
						ResourceRef:   desiredStore.Ref,
						ResourceID:    current.ID,
						Action:        ActionUpdate,
						Fields:        fields,
						ChangedFields: changed,
						Namespace:     namespace,
						Parent:        &ParentInfo{Ref: gatewayRef, ID: gatewayID},
					})
				}
				return p.planAIGatewayConfigStoreSecretChangesIfNeeded(
					ctx, namespace, gatewayRef, gatewayID, desiredStore.Ref, current.ID, "", plan,
				)
			},
			remove: func(current state.AIGatewayConfigStore) {
				plan.AddChange(PlannedChange{
					ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGatewayConfigStore, current.Name),
					ResourceType: ResourceTypeAIGatewayConfigStore,
					ResourceRef:  current.Name,
					ResourceID:   current.ID,
					Action:       ActionDelete,
					Namespace:    namespace,
					Fields:       map[string]any{FieldName: current.Name},
					Parent:       &ParentInfo{Ref: gatewayRef, ID: gatewayID},
				})
			},
		}, plan)
}

func (p *Planner) planAIGatewayConfigStoreSecretChangesIfNeeded(
	ctx context.Context,
	namespace string,
	gatewayRef string,
	gatewayID string,
	storeRef string,
	storeID string,
	storeChangeID string,
	plan *Plan,
) error {
	secrets := p.resources.GetAIGatewayConfigStoreSecretsForStore(storeRef)
	if !p.shouldPlanChild(
		plan,
		resources.ResourceTypeAIGatewayConfigStore,
		storeRef,
		resources.ResourceTypeAIGatewayConfigStoreSecret,
	) || (len(secrets) == 0 && plan.Metadata.Mode != PlanModeSync) {
		return nil
	}
	return p.planAIGatewayConfigStoreSecretChanges(
		ctx, namespace, gatewayRef, gatewayID, storeRef, storeID, storeChangeID, secrets, plan,
	)
}

func (p *Planner) planAIGatewayConfigStoreCreate(
	namespace string,
	gatewayRef string,
	_ string,
	gatewayID string,
	store resources.AIGatewayConfigStoreResource,
	dependsOn []string,
	plan *Plan,
) string {
	fields, err := store.MutablePayloadMap()
	if err != nil {
		plan.AddWarning(
			store.GetRef(),
			fmt.Sprintf("failed to build AI Gateway Config Store create payload: %s", err),
		)
		return ""
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayConfigStore, store.Ref),
		ResourceType: ResourceTypeAIGatewayConfigStore,
		ResourceRef:  store.Ref,
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
	return change.ID
}

func stringPointersEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
