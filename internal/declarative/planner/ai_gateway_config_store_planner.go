package planner

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/util"
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
			p.planAIGatewayConfigStoreCreate(namespace, gatewayRef, gatewayName, "", store, dependsOn, plan)
		}
		return nil
	}

	currentStores, err := p.client.ListAIGatewayConfigStores(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway Config Stores for gateway %s: %w", gatewayID, err)
	}
	currentByID, currentByName := indexAIGatewayConfigStores(currentStores)
	desiredKeys := make(map[string]bool)

	for _, desiredStore := range desired {
		current, exists := matchCurrentAIGatewayConfigStore(desiredStore, currentByID, currentByName)
		desiredKeys[desiredStore.Name] = true
		if id := aiGatewayConfigStoreDesiredID(desiredStore); id != "" {
			desiredKeys[id] = true
		}
		if !exists {
			p.planAIGatewayConfigStoreCreate(
				namespace,
				gatewayRef,
				gatewayName,
				gatewayID,
				desiredStore,
				nil,
				plan,
			)
			continue
		}

		if id := aiGatewayConfigStoreDesiredID(desiredStore); id != "" && current.Name != desiredStore.Name {
			return fmt.Errorf(
				"AI Gateway Config Store %q is matched by ID %s but its immutable name is %q; "+
					"delete and recreate it to use name %q",
				desiredStore.Ref,
				id,
				current.Name,
				desiredStore.Name,
			)
		}
		if resource := p.resources.GetAIGatewayConfigStoreByRef(desiredStore.Ref); resource != nil {
			resource.SetKonnectID(current.ID)
		}
		if desiredStore.DisplayName == nil || stringPointersEqual(desiredStore.DisplayName, current.DisplayName) {
			continue
		}

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

	if plan.Metadata.Mode == PlanModeSync && !p.isAIGatewayExternal(gatewayRef) {
		for _, current := range currentStores {
			if desiredKeys[current.ID] || desiredKeys[current.Name] {
				continue
			}
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
		}
	}
	return nil
}

func (p *Planner) planAIGatewayConfigStoreCreate(
	namespace string,
	gatewayRef string,
	_ string,
	gatewayID string,
	store resources.AIGatewayConfigStoreResource,
	dependsOn []string,
	plan *Plan,
) {
	fields, _ := store.MutablePayloadMap()
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
}

func indexAIGatewayConfigStores(
	stores []state.AIGatewayConfigStore,
) (map[string]state.AIGatewayConfigStore, map[string]state.AIGatewayConfigStore) {
	byID := make(map[string]state.AIGatewayConfigStore, len(stores))
	byName := make(map[string]state.AIGatewayConfigStore, len(stores))
	for _, store := range stores {
		byID[store.ID] = store
		byName[store.Name] = store
	}
	return byID, byName
}

func matchCurrentAIGatewayConfigStore(
	desired resources.AIGatewayConfigStoreResource,
	currentByID map[string]state.AIGatewayConfigStore,
	currentByName map[string]state.AIGatewayConfigStore,
) (state.AIGatewayConfigStore, bool) {
	if id := aiGatewayConfigStoreDesiredID(desired); id != "" {
		store, ok := currentByID[id]
		return store, ok
	}
	store, ok := currentByName[desired.Name]
	return store, ok
}

func aiGatewayConfigStoreDesiredID(desired resources.AIGatewayConfigStoreResource) string {
	if id := desired.GetKonnectID(); id != "" {
		return id
	}
	if util.IsValidUUID(desired.Ref) {
		return desired.Ref
	}
	return ""
}

func stringPointersEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
