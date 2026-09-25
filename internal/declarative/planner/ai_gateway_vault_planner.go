package planner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) planAIGatewayVaultChanges(
	ctx context.Context,
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	gatewayChangeID string,
	desired []resources.AIGatewayVaultResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning AI Gateway Vault changes",
		slog.String("gateway_ref", gatewayRef),
		slog.String("gateway_id", gatewayID),
		slog.String("gateway_change_id", gatewayChangeID),
		slog.Int("desired_count", len(desired)),
	)

	if gatewayID == "" {
		p.planAIGatewayVaultCreatesForNewGateway(namespace, gatewayRef, gatewayName, gatewayChangeID, desired, plan)
		return nil
	}

	currentVaults, err := p.client.ListAIGatewayVaults(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway Vaults for gateway %s: %w", gatewayID, err)
	}

	return reconcileNameMatchedChildren(p, ResourceTypeAIGatewayVault, desired, currentVaults,
		nameMatchedChildOperations[resources.AIGatewayVaultResource, state.AIGatewayVault]{
			desiredName: func(desired resources.AIGatewayVaultResource) string {
				return desired.Name()
			},
			currentName: func(current state.AIGatewayVault) string {
				return resources.AIGatewayVaultName(current.AIGatewayVault)
			},
			fetch: func(_ resources.AIGatewayVaultResource, current state.AIGatewayVault) (*state.AIGatewayVault, error) {
				id := resources.AIGatewayVaultID(current.AIGatewayVault)
				full, err := p.client.GetAIGatewayVault(ctx, gatewayID, id)
				if err != nil {
					return nil, fmt.Errorf("failed to get AI Gateway Vault %s: %w", id, err)
				}
				return full, nil
			},
			diff: func(current state.AIGatewayVault, desired resources.AIGatewayVaultResource) (
				bool, map[string]any, map[string]FieldChange, error,
			) {
				return shouldUpdateAIGatewayVault(current, desired, p.resources)
			},
			create: func(desired resources.AIGatewayVaultResource) {
				p.planAIGatewayVaultCreate(namespace, gatewayRef, gatewayName, gatewayID, desired,
					nil, plan)
			},
			update: func(current state.AIGatewayVault, desired resources.AIGatewayVaultResource,
				fields map[string]any, changed map[string]FieldChange,
			) {
				p.planAIGatewayVaultUpdate(namespace, gatewayRef, gatewayID,
					resources.AIGatewayVaultID(current.AIGatewayVault), desired, fields, changed,
					nil, plan)
			},
			protected: func(current state.AIGatewayVault) bool {
				return labels.IsProtectedResource(current.NormalizedLabels)
			},
			remove: func(current state.AIGatewayVault) {
				p.planAIGatewayVaultDelete(namespace, gatewayRef, gatewayID,
					resources.AIGatewayVaultID(current.AIGatewayVault),
					resources.AIGatewayVaultName(current.AIGatewayVault), plan)
			},
		}, plan)
}

func (p *Planner) planAIGatewayVaultCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	vaults []resources.AIGatewayVaultResource,
	plan *Plan,
) {
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}
	for _, vault := range vaults {
		p.planAIGatewayVaultCreate(namespace, gatewayRef, gatewayName, "", vault, dependsOn, plan)
	}
}

func (p *Planner) planAIGatewayVaultCreate(
	namespace string,
	gatewayRef string,
	_ string,
	gatewayID string,
	vault resources.AIGatewayVaultResource,
	dependsOn []string,
	plan *Plan,
) {
	fields, err := vault.MutablePayloadMap()
	if err != nil {
		plan.AddWarning(vault.GetRef(), fmt.Sprintf("failed to build AI Gateway Vault create payload: %s", err))
		return
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayVault, vault.Ref),
		ResourceType: ResourceTypeAIGatewayVault,
		ResourceRef:  vault.Ref,
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

func (p *Planner) planAIGatewayVaultUpdate(
	namespace string,
	gatewayRef string,
	gatewayID string,
	vaultID string,
	vault resources.AIGatewayVaultResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	dependsOn []string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeAIGatewayVault, vault.Ref),
		ResourceType:  ResourceTypeAIGatewayVault,
		ResourceRef:   vault.Ref,
		ResourceID:    vaultID,
		Action:        ActionUpdate,
		Fields:        updateFields,
		ChangedFields: changedFields,
		Namespace:     namespace,
		DependsOn:     dependsOn,
		Parent:        &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func (p *Planner) planAIGatewayVaultDelete(
	namespace string,
	gatewayRef string,
	gatewayID string,
	vaultID string,
	vaultName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGatewayVault, vaultName),
		ResourceType: ResourceTypeAIGatewayVault,
		ResourceRef:  vaultName,
		ResourceID:   vaultID,
		Action:       ActionDelete,
		Namespace:    namespace,
		Fields: map[string]any{
			FieldName: vaultName,
		},
		Parent: &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func shouldUpdateAIGatewayVault(
	current state.AIGatewayVault,
	desired resources.AIGatewayVaultResource,
	resourceSet *resources.ResourceSet,
) (bool, map[string]any, map[string]FieldChange, error) {
	currentPayload, err := resources.AIGatewayVaultMutablePayloadMap(current.AIGatewayVault)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to normalize current AI Gateway Vault: %w", err)
	}
	desiredPayload, err := desired.MutablePayloadMap()
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to normalize desired AI Gateway Vault %q: %w", desired.Ref, err)
	}

	currentCompare, desiredCompare := comparableAIGatewayVaultPayloads(currentPayload, desiredPayload)
	normalizeAIGatewayVaultConfigStoreReferenceForComparison(desiredCompare, resourceSet)

	currentPlanPayload := scrubAIGatewayVaultWriteOnlyFields(currentPayload).(map[string]any)
	desiredPlanPayload := scrubAIGatewayVaultWriteOnlyFields(desiredPayload).(map[string]any)

	changedFields := diffAIGatewayPayloads(currentPlanPayload, desiredPlanPayload, currentCompare, desiredCompare)
	if len(changedFields) == 0 {
		return false, nil, nil, nil
	}

	return true, desiredPayload, changedFields, nil
}

func comparableAIGatewayVaultPayloads(current, desired map[string]any) (map[string]any, map[string]any) {
	return comparableAIGatewayWriteOnlyPayloads(
		current,
		desired,
		normalizeAIGatewayVaultPayloadsForComparison,
		isAIGatewayVaultWriteOnlyField,
	)
}

func normalizeAIGatewayVaultConfigStoreReferenceForComparison(
	payload map[string]any,
	resourceSet *resources.ResourceSet,
) {
	if resourceSet == nil {
		return
	}
	config, ok := payload[FieldConfig].(map[string]any)
	if !ok {
		return
	}
	configuredID, ok := config[FieldConfigStoreID].(string)
	if !ok || configuredID == "" {
		return
	}
	store := resourceSet.GetAIGatewayConfigStoreByRef(resources.NormalizeResourceRef(configuredID))
	if store == nil || store.GetKonnectID() == "" {
		return
	}
	config[FieldConfigStoreID] = store.GetKonnectID()
}

func scrubAIGatewayVaultWriteOnlyFields(value any) any {
	return scrubAIGatewayWriteOnlyFields(value, isAIGatewayVaultWriteOnlyField)
}

func isAIGatewayVaultWriteOnlyField(key string) bool {
	switch strings.ToLower(key) {
	case FieldAPIKey, FieldClientSecret, "key", "secret_access_key", "secret_id", FieldToken:
		return true
	default:
		return false
	}
}
