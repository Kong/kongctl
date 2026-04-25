package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// planEventGatewayStaticKeyChanges plans changes for Event Gateway Static Keys for a specific gateway.
// Static keys do not support update – detected changes are planned as DELETE + CREATE.
func (p *Planner) planEventGatewayStaticKeyChanges(
	ctx context.Context,
	_ *Config,
	namespace string,
	gatewayName string,
	gatewayID string,
	gatewayRef string,
	gatewayChangeID string,
	desired []resources.EventGatewayStaticKeyResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning Event Gateway Static Key changes",
		"gateway_name", gatewayName,
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"gateway_change_id", gatewayChangeID,
		"desired_count", len(desired),
		"namespace", namespace,
	)

	if gatewayID != "" {
		// Gateway exists: full diff
		return p.planStaticKeyChangesForExistingGateway(
			ctx, namespace, gatewayID, gatewayRef, gatewayName, desired, plan,
		)
	}

	// Gateway doesn't exist yet: plan creates only with dependency on gateway creation
	p.planStaticKeyCreatesForNewGateway(namespace, gatewayRef, gatewayName, gatewayChangeID, desired, plan)
	return nil
}

// planStaticKeyChangesForExistingGateway handles full diff for static keys of an existing gateway.
func (p *Planner) planStaticKeyChangesForExistingGateway(
	ctx context.Context,
	namespace string,
	gatewayID string,
	gatewayRef string,
	gatewayName string,
	desired []resources.EventGatewayStaticKeyResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning changes for existing gateway static keys",
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"desired_count", len(desired),
	)

	// 1. List current static keys for this gateway
	currentKeys, err := p.client.ListEventGatewayStaticKeys(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list static keys for gateway %s: %w", gatewayID, err)
	}

	p.logger.Debug("Fetched current static keys",
		"gateway_id", gatewayID,
		"current_count", len(currentKeys),
	)

	// 2. Index current by name
	currentByName := make(map[string]state.EventGatewayStaticKey)
	for _, sk := range currentKeys {
		currentByName[sk.Name] = sk
	}

	desiredNames := make(map[string]bool)

	// 3. Compare desired vs current
	for _, desiredKey := range desired {
		desiredNames[desiredKey.Name] = true

		current, exists := currentByName[desiredKey.Name]

		if !exists {
			// CREATE
			p.logger.Debug("Planning static key CREATE",
				"key_name", desiredKey.Name,
				"gateway_ref", gatewayRef,
			)
			p.planStaticKeyCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredKey, []string{}, plan)
		} else {
			// Static keys do not support update – detect changes and plan DELETE + CREATE instead.
			needsChange := p.doesStaticKeyNeedChange(current, desiredKey)
			if needsChange {
				p.logger.Debug("Planning static key DELETE+CREATE (no update supported)",
					"key_name", desiredKey.Name,
					"key_id", current.ID,
					"gateway_ref", gatewayRef,
				)
				deleteChangeID := p.planStaticKeyDelete(gatewayRef, gatewayName, gatewayID, current.ID, desiredKey.Name, plan)
				// CREATE depends on the DELETE being applied first
				p.planStaticKeyCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredKey,
					[]string{deleteChangeID}, plan)
			}
		}
	}

	// 4. SYNC MODE: Delete unmanaged static keys
	if plan.Metadata.Mode == PlanModeSync {
		for name, current := range currentByName {
			if !desiredNames[name] {
				p.logger.Debug("Planning static key DELETE (sync mode)",
					"key_name", name,
					"key_id", current.ID,
				)
				p.planStaticKeyDelete(gatewayRef, gatewayName, gatewayID, current.ID, name, plan)
			}
		}
	}

	return nil
}

// planStaticKeyCreatesForNewGateway plans creates for static keys when the gateway doesn't exist yet.
func (p *Planner) planStaticKeyCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	keys []resources.EventGatewayStaticKeyResource,
	plan *Plan,
) {
	p.logger.Debug("Planning static key creates for new gateway",
		"gateway_ref", gatewayRef,
		"gateway_change_id", gatewayChangeID,
		"key_count", len(keys),
	)

	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}

	for _, key := range keys {
		p.planStaticKeyCreate(namespace, gatewayRef, gatewayName, "", key, dependsOn, plan)
	}
}

// planStaticKeyCreate plans a CREATE change for a static key.
func (p *Planner) planStaticKeyCreate(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	key resources.EventGatewayStaticKeyResource,
	dependsOn []string,
	plan *Plan,
) {
	fields := make(map[string]any)
	fields[FieldName] = key.Name
	fields[FieldValue] = key.Value

	if key.Description != nil {
		fields[FieldDescription] = *key.Description
	}

	if len(key.Labels) > 0 {
		fields[FieldLabels] = key.Labels
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeEventGatewayStaticKey, key.Ref),
		ResourceType: ResourceTypeEventGatewayStaticKey,
		ResourceRef:  key.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		Namespace:    namespace,
		DependsOn:    dependsOn,
	}

	if gatewayID != "" {
		change.Parent = &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		}
	} else {
		// Gateway doesn't exist yet, add reference for runtime resolution
		change.References = map[string]ReferenceInfo{
			FieldEventGatewayID: {
				Ref: gatewayRef,
				ID:  "", // to be resolved at runtime
				LookupFields: map[string]string{
					FieldName: gatewayName,
				},
			},
		}
	}

	p.logger.Debug("Enqueuing static key CREATE",
		"key_ref", key.Ref,
		"key_name", key.Name,
		"gateway_ref", gatewayRef,
	)

	plan.AddChange(change)
}

// planStaticKeyDelete plans a DELETE change for a static key and returns the change ID.
func (p *Planner) planStaticKeyDelete(
	gatewayRef string,
	_ string, // gatewayName - unused but kept for API consistency
	gatewayID string,
	keyID string,
	keyName string,
	plan *Plan,
) string {
	changeID := p.nextChangeID(ActionDelete, ResourceTypeEventGatewayStaticKey, keyName)

	change := PlannedChange{
		ID:           changeID,
		ResourceType: ResourceTypeEventGatewayStaticKey,
		ResourceRef:  keyName,
		ResourceID:   keyID,
		Action:       ActionDelete,
		Fields:       map[string]any{},
		Parent: &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		},
	}

	p.logger.Debug("Enqueuing static key DELETE",
		"key_name", keyName,
		"key_id", keyID,
		"gateway_ref", gatewayRef,
	)

	plan.AddChange(change)

	return changeID
}

// doesStaticKeyNeedChange checks whether a static key needs to be replaced.
// The API returns the stored value when it is a vault reference, but does not return plaintext. We compare
// it directly against the desired value anyway.
//
// Any change triggers a replace (static keys do not support update).
func (p *Planner) doesStaticKeyNeedChange(
	current state.EventGatewayStaticKey,
	desired resources.EventGatewayStaticKeyResource,
) bool {
	// Compare value
	currentVal := ""
	if current.Value != nil {
		currentVal = *current.Value
	}
	if currentVal != desired.Value {
		return true
	}

	// Compare description
	currentDesc := ""
	if current.Description != nil {
		currentDesc = *current.Description
	}
	desiredDesc := ""
	if desired.Description != nil {
		desiredDesc = *desired.Description
	}
	if currentDesc != desiredDesc {
		return true
	}

	// Compare labels
	if !compareStringMaps(current.Labels, desired.Labels) {
		return true
	}

	return false
}
