package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// planEventGatewayListenerChanges plans changes for Event Gateway Listeners for a specific gateway
func (p *Planner) planEventGatewayListenerChanges(
	ctx context.Context,
	_ *Config,
	namespace string,
	gatewayName string,
	gatewayID string,
	gatewayRef string,
	gatewayChangeID string,
	desired []resources.EventGatewayListenerResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning Event Gateway Listener changes",
		"gateway_name", gatewayName,
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"gateway_change_id", gatewayChangeID,
		"desired_count", len(desired),
		"namespace", namespace,
	)

	if gatewayID != "" {
		// Gateway exists: full diff
		return p.planListenerChangesForExistingGateway(
			ctx, namespace, gatewayID, gatewayRef, gatewayName, desired, plan,
		)
	}

	// Gateway doesn't exist: plan creates only with dependency on gateway creation
	p.planListenerCreatesForNewGateway(namespace, gatewayRef, gatewayName, gatewayChangeID, desired, plan)
	return nil
}

// planListenerChangesForExistingGateway handles full diff for listeners of an existing gateway
func (p *Planner) planListenerChangesForExistingGateway(
	ctx context.Context,
	namespace string,
	gatewayID string,
	gatewayRef string,
	gatewayName string,
	desired []resources.EventGatewayListenerResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning changes for existing gateway listeners",
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"desired_count", len(desired),
	)

	// 1. List current listeners for this gateway
	currentListeners, err := p.client.ListEventGatewayListeners(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list listeners for gateway %s: %w", gatewayID, err)
	}

	p.logger.Debug("Fetched current listeners",
		"gateway_id", gatewayID,
		"current_count", len(currentListeners),
	)

	// 2. Index by name
	currentByName := make(map[string]state.EventGatewayListener)
	for _, listener := range currentListeners {
		currentByName[listener.Name] = listener
	}

	// 3. Compare desired vs current
	desiredNames := make(map[string]bool)
	for _, desiredListener := range desired {
		desiredNames[desiredListener.Name] = true
		current, exists := currentByName[desiredListener.Name]
		if !exists {
			// CREATE
			p.logger.Debug("Planning listener CREATE",
				"listener_name", desiredListener.Name,
				"gateway_ref", gatewayRef,
			)
			p.planListenerCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredListener, []string{}, plan)
		} else {
			// CHECK UPDATE
			p.logger.Debug("Checking if listener needs update",
				"listener_name", desiredListener.Name,
				"listener_id", current.ID,
			)

			// Fetch full details if needed
			fullListener, err := p.client.GetEventGatewayListener(ctx, gatewayID, current.ID)
			if err != nil {
				return fmt.Errorf("failed to get listener %s: %w", current.ID, err)
			}

			needsUpdate, updateFields := p.shouldUpdateListener(*fullListener, desiredListener)
			if needsUpdate {
				p.logger.Debug("Planning listener UPDATE",
					"listener_name", desiredListener.Name,
					"listener_id", current.ID,
					"update_fields", updateFields,
				)
				p.planListenerUpdate(
					namespace, gatewayRef, gatewayName, gatewayID,
					current.ID, desiredListener, updateFields, plan)
			}
		}
	}

	// 4. SYNC MODE: Delete unmanaged listeners
	if plan.Metadata.Mode == PlanModeSync {
		for name, current := range currentByName {
			if !desiredNames[name] {
				p.logger.Debug("Planning listener DELETE (sync mode)",
					"listener_name", name,
					"listener_id", current.ID,
				)
				p.planListenerDelete(gatewayRef, gatewayName, gatewayID, current.ID, name, plan)
			}
		}
	}

	return nil
}

// planListenerCreatesForNewGateway plans creates for listeners when the gateway doesn't exist yet
func (p *Planner) planListenerCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	listeners []resources.EventGatewayListenerResource,
	plan *Plan,
) {
	p.logger.Debug("Planning listener creates for new gateway",
		"gateway_ref", gatewayRef,
		"gateway_change_id", gatewayChangeID,
		"listener_count", len(listeners),
	)

	// Build dependencies - listeners depend on gateway being created first
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}

	for _, listener := range listeners {
		p.planListenerCreate(namespace, gatewayRef, gatewayName, "", listener, dependsOn, plan)
	}
}

// planListenerCreate plans a CREATE change for a listener
func (p *Planner) planListenerCreate(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	listener resources.EventGatewayListenerResource,
	dependsOn []string,
	plan *Plan,
) {
	fields := make(map[string]any)
	fields["name"] = listener.Name
	if listener.Description != nil {
		fields["description"] = *listener.Description
	}
	fields["addresses"] = listener.Addresses
	fields["ports"] = listener.Ports
	if len(listener.Labels) > 0 {
		fields["labels"] = listener.Labels
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeEventGatewayListener, listener.Ref),
		ResourceType: ResourceTypeEventGatewayListener,
		ResourceRef:  listener.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		Namespace:    namespace,
		DependsOn:    dependsOn,
	}

	// Set parent reference
	if gatewayID != "" {
		change.Parent = &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		}
	} else {
		// Gateway doesn't exist yet, add reference for runtime resolution
		change.References = map[string]ReferenceInfo{
			"event_gateway_id": {
				Ref: gatewayRef,
				ID:  "", // to be resolved at runtime
				LookupFields: map[string]string{
					"name": gatewayName,
				},
			},
		}
	}

	p.logger.Debug("Enqueuing listener CREATE",
		"listener_ref", listener.Ref,
		"listener_name", listener.Name,
		"gateway_ref", gatewayRef,
	)
	plan.AddChange(change)
}

// planListenerUpdate plans an UPDATE change for a listener
func (p *Planner) planListenerUpdate(
	namespace string,
	gatewayRef string,
	_ string, // gatewayName - unused but kept for API consistency
	gatewayID string,
	listenerID string,
	listener resources.EventGatewayListenerResource,
	updateFields map[string]any,
	plan *Plan,
) {
	if len(updateFields) == 0 {
		return
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypeEventGatewayListener, listener.Ref),
		ResourceType: ResourceTypeEventGatewayListener,
		ResourceRef:  listener.Ref,
		ResourceID:   listenerID,
		Action:       ActionUpdate,
		Fields:       updateFields,
		Namespace:    namespace,
		Parent: &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		},
	}

	p.logger.Debug("Enqueuing listener UPDATE",
		"listener_ref", listener.Ref,
		"listener_name", listener.Name,
		"listener_id", listenerID,
		"fields", updateFields,
	)
	plan.AddChange(change)
}

// planListenerDelete plans a DELETE change for a listener
func (p *Planner) planListenerDelete(
	gatewayRef string,
	_ string, // gatewayName - unused but kept for API consistency
	gatewayID string,
	listenerID string,
	listenerName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeEventGatewayListener, listenerName),
		ResourceType: ResourceTypeEventGatewayListener,
		ResourceRef:  listenerName,
		ResourceID:   listenerID,
		Action:       ActionDelete,
		Parent: &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		},
	}

	p.logger.Debug("Enqueuing listener DELETE",
		"listener_name", listenerName,
		"listener_id", listenerID,
	)
	plan.AddChange(change)
}

// shouldUpdateListener compares current and desired listener state
func (p *Planner) shouldUpdateListener(
	current state.EventGatewayListener,
	desired resources.EventGatewayListenerResource,
) (bool, map[string]any) {
	updates := make(map[string]any)
	var needsUpdate bool

	// Compare name
	if current.Name != desired.Name {
		needsUpdate = true
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
		needsUpdate = true
	}

	// Compare addresses
	if !compareStringSlices(current.Addresses, desired.Addresses) {
		needsUpdate = true
	}

	// Compare ports (simplified - full comparison would need deep equality check)
	if len(current.Ports) != len(desired.Ports) {
		needsUpdate = true
	}

	// Compare labels
	if desired.Labels != nil {
		if !compareMaps(current.Labels, desired.Labels) {
			needsUpdate = true
		}
	}

	// If any changes detected, set ALL properties from desired state for PUT request
	if needsUpdate {
		updates["name"] = desired.Name

		if desired.Description != nil {
			updates["description"] = *desired.Description
		}

		updates["addresses"] = desired.Addresses
		updates["ports"] = desired.Ports

		if len(desired.Labels) > 0 {
			updates["labels"] = desired.Labels
		}
	}

	return needsUpdate, updates
}
