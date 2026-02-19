package planner

import (
	"context"
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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
			listenerChangeID := p.planListenerCreate(
				namespace, gatewayRef, gatewayName, gatewayID, desiredListener, []string{}, plan,
			)

			// Plan listener policies for this new listener (depends on listener creation)
			listenerPolicies := p.resources.GetPoliciesForListener(desiredListener.Ref)
			if len(listenerPolicies) > 0 {
				if err := p.planEventGatewayListenerPolicyChanges(
					ctx, nil, namespace, gatewayID, gatewayRef,
					desiredListener.Name, "", desiredListener.Ref,
					listenerChangeID, listenerPolicies, plan,
				); err != nil {
					return err
				}
			}
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

			// Plan listener policies for this existing listener
			listenerPolicies := p.resources.GetPoliciesForListener(desiredListener.Ref)
			if len(listenerPolicies) > 0 || plan.Metadata.Mode == PlanModeSync {
				if err := p.planEventGatewayListenerPolicyChanges(
					ctx, nil, namespace, gatewayID, gatewayRef,
					desiredListener.Name, current.ID, desiredListener.Ref,
					"", listenerPolicies, plan,
				); err != nil {
					return err
				}
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
		listenerChangeID := p.planListenerCreate(
			namespace, gatewayRef, gatewayName, "", listener, dependsOn, plan,
		)

		// Plan listener policies for this new listener (depends on listener creation)
		listenerPolicies := p.resources.GetPoliciesForListener(listener.Ref)
		if len(listenerPolicies) > 0 {
			p.planListenerPolicyCreatesForNewListener(
				namespace, gatewayRef, listener.Ref, listener.Name,
				listenerChangeID, listenerPolicies, plan,
			)
		}
	}
}

// planListenerCreate plans a CREATE change for a listener and returns the change ID
func (p *Planner) planListenerCreate(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	listener resources.EventGatewayListenerResource,
	dependsOn []string,
	plan *Plan,
) string {
	fields := make(map[string]any)
	fields["name"] = listener.Name
	if listener.Description != nil {
		fields["description"] = *listener.Description
	}
	fields["addresses"] = listener.Addresses
	// Normalize ports to strings for API compatibility
	fields["ports"] = normalizePortsToStrings(convertPortsToAny(listener.Ports))
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
	return change.ID
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

// convertPortsToAny converts ports field to []any for processing
// Handles various port types (EventGatewayListenerPort, []any, etc.)
func convertPortsToAny(ports any) []any {
	if ports == nil {
		return []any{}
	}

	// Check if already []any
	if portsSlice, ok := ports.([]any); ok {
		return portsSlice
	}

	// Use reflection to convert slice to []any
	val := reflect.ValueOf(ports)
	if val.Kind() != reflect.Slice {
		return []any{}
	}

	result := make([]any, val.Len())
	for i := 0; i < val.Len(); i++ {
		result[i] = val.Index(i).Interface()
	}
	return result
}

// extractPortStrings extracts string values from EventGatewayListenerPort array
// Assumes all ports are already normalized to string type
func extractPortStrings(ports any) []string {
	result := []string{}

	// Use reflection to handle the slice
	val := reflect.ValueOf(ports)
	if val.Kind() != reflect.Slice {
		return result
	}

	for i := 0; i < val.Len(); i++ {
		portVal := val.Index(i).Interface()

		// Extract string from EventGatewayListenerPort struct
		if egwPort, ok := portVal.(kkComps.EventGatewayListenerPort); ok {
			if egwPort.Str != nil {
				result = append(result, *egwPort.Str)
			}
		}
	}

	return result
}

// normalizePortsToStrings converts EventGatewayListenerPort array to string array
// Ports can be either integers or strings (for ranges), so we normalize to strings
func normalizePortsToStrings(ports []any) []string {
	result := make([]string, 0, len(ports))
	for _, port := range ports {
		// Check if it's an EventGatewayListenerPort struct
		if egwPort, ok := port.(kkComps.EventGatewayListenerPort); ok {
			// Extract the actual value based on the Type field
			if egwPort.Integer != nil {
				result = append(result, fmt.Sprintf("%d", *egwPort.Integer))
			} else if egwPort.Str != nil {
				result = append(result, *egwPort.Str)
			}
			continue
		}

		// Fallback to handle other types (for backwards compatibility)
		switch v := port.(type) {
		case string:
			result = append(result, v)
		case int:
			result = append(result, fmt.Sprintf("%d", v))
		case int64:
			result = append(result, fmt.Sprintf("%d", v))
		case float64:
			// JSON unmarshaling sometimes converts numbers to float64
			result = append(result, fmt.Sprintf("%.0f", v))
		default:
			// Fallback: use fmt.Sprint
			result = append(result, fmt.Sprint(v))
		}
	}
	return result
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

	// Compare ports - extract string values and compare
	// Ports are normalized to strings during unmarshaling
	currentPortStrings := extractPortStrings(current.Ports)
	desiredPortStrings := extractPortStrings(desired.Ports)
	if !compareStringSlices(currentPortStrings, desiredPortStrings) {
		needsUpdate = true
	}

	// Compare labels
	if desired.Labels != nil {
		if !compareMaps(current.Labels, desired.Labels) {
			needsUpdate = true
		}
	} else if len(current.Labels) > 0 {
		needsUpdate = true
	}

	// If any changes detected, set ALL properties from desired state for PUT request
	if needsUpdate {
		updates["name"] = desired.Name

		if desired.Description != nil {
			updates["description"] = *desired.Description
		}

		updates["addresses"] = desired.Addresses
		// Extract string values from ports for updates
		updates["ports"] = extractPortStrings(desired.Ports)

		if len(desired.Labels) > 0 {
			updates["labels"] = desired.Labels
		} else if len(current.Labels) > 0 {
			// Clear labels if desired state has no labels but current state has labels
			updates["labels"] = map[string]string{}
		}
	}

	return needsUpdate, updates
}
