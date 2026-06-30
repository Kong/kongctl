package planner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/util"
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

	currentByID, currentByName := indexAIGatewayVaults(currentVaults)
	desiredKeys := make(map[string]bool)

	for _, desiredVault := range desired {
		current, exists := matchCurrentAIGatewayVault(desiredVault, currentByID, currentByName)
		desiredKeys[desiredVault.Name()] = true
		if id := aiGatewayVaultDesiredID(desiredVault); id != "" {
			desiredKeys[id] = true
		}

		if !exists {
			p.planAIGatewayVaultCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredVault, nil, plan)
			continue
		}

		vaultID := resources.AIGatewayVaultID(current.AIGatewayVault)
		fullVault, err := p.client.GetAIGatewayVault(ctx, gatewayID, vaultID)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway Vault %s: %w", vaultID, err)
		}
		if fullVault == nil {
			p.planAIGatewayVaultCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredVault, nil, plan)
			continue
		}

		needsUpdate, updateFields, changedFields, err := shouldUpdateAIGatewayVault(*fullVault, desiredVault)
		if err != nil {
			return err
		}
		if needsUpdate {
			p.planAIGatewayVaultUpdate(
				namespace,
				gatewayRef,
				gatewayID,
				vaultID,
				desiredVault,
				updateFields,
				changedFields,
				nil,
				plan,
			)
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		for _, current := range currentVaults {
			vaultID := resources.AIGatewayVaultID(current.AIGatewayVault)
			vaultName := resources.AIGatewayVaultName(current.AIGatewayVault)
			if desiredKeys[vaultID] || desiredKeys[vaultName] {
				continue
			}
			p.planAIGatewayVaultDelete(gatewayRef, gatewayID, vaultID, vaultName, plan)
		}
	}

	return nil
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
) (bool, map[string]any, map[string]FieldChange, error) {
	currentPayload, err := resources.AIGatewayVaultMutablePayloadMap(current.AIGatewayVault)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to normalize current AI Gateway Vault: %w", err)
	}
	desiredPayload, err := desired.MutablePayloadMap()
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to normalize desired AI Gateway Vault %q: %w", desired.Ref, err)
	}

	currentCompare, desiredCompare := normalizeAIGatewayPayloadsForComparison(currentPayload, desiredPayload)
	currentCompare = scrubAIGatewayVaultWriteOnlyFields(currentCompare).(map[string]any)
	desiredCompare = scrubAIGatewayVaultWriteOnlyFields(desiredCompare).(map[string]any)

	currentPlanPayload := scrubAIGatewayVaultWriteOnlyFields(currentPayload).(map[string]any)
	desiredPlanPayload := scrubAIGatewayVaultWriteOnlyFields(desiredPayload).(map[string]any)

	changedFields := diffAIGatewayPayloads(currentPlanPayload, desiredPlanPayload, currentCompare, desiredCompare)
	if len(changedFields) == 0 {
		return false, nil, nil, nil
	}

	return true, desiredPlanPayload, changedFields, nil
}

func scrubAIGatewayVaultWriteOnlyFields(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, val := range typed {
			if isAIGatewayVaultWriteOnlyField(key) {
				continue
			}
			result[key] = scrubAIGatewayVaultWriteOnlyFields(val)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i := range typed {
			result[i] = scrubAIGatewayVaultWriteOnlyFields(typed[i])
		}
		return result
	default:
		return value
	}
}

func isAIGatewayVaultWriteOnlyField(key string) bool {
	switch strings.ToLower(key) {
	case "api_key", "client_secret", "key", "secret_access_key", "secret_id", "token":
		return true
	default:
		return false
	}
}

func indexAIGatewayVaults(
	vaults []state.AIGatewayVault,
) (map[string]state.AIGatewayVault, map[string]state.AIGatewayVault) {
	byID := make(map[string]state.AIGatewayVault)
	byName := make(map[string]state.AIGatewayVault)
	for _, vault := range vaults {
		if id := resources.AIGatewayVaultID(vault.AIGatewayVault); id != "" {
			byID[id] = vault
		}
		if name := resources.AIGatewayVaultName(vault.AIGatewayVault); name != "" {
			byName[name] = vault
		}
	}
	return byID, byName
}

func matchCurrentAIGatewayVault(
	desired resources.AIGatewayVaultResource,
	currentByID map[string]state.AIGatewayVault,
	currentByName map[string]state.AIGatewayVault,
) (state.AIGatewayVault, bool) {
	if id := aiGatewayVaultDesiredID(desired); id != "" {
		current, exists := currentByID[id]
		return current, exists
	}
	current, exists := currentByName[desired.Name()]
	return current, exists
}

func aiGatewayVaultDesiredID(desired resources.AIGatewayVaultResource) string {
	if id := desired.GetKonnectID(); id != "" {
		return id
	}
	if util.IsValidUUID(desired.Ref) {
		return desired.Ref
	}
	return ""
}
