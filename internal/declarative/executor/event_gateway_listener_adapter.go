package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// EventGatewayListenerAdapter implements ResourceOperations for Event Gateway Listener resources
type EventGatewayListenerAdapter struct {
	client *state.Client
}

// NewEventGatewayListenerAdapter creates a new EventGatewayListenerAdapter
func NewEventGatewayListenerAdapter(client *state.Client) *EventGatewayListenerAdapter {
	return &EventGatewayListenerAdapter{
		client: client,
	}
}

// MapCreateFields maps fields to CreateEventGatewayListenerRequest
func (a *EventGatewayListenerAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateEventGatewayListenerRequest,
) error {
	// Required fields
	name, ok := fields["name"].(string)
	if !ok {
		return fmt.Errorf("name is required")
	}
	create.Name = name

	// Addresses (required)
	addressesField, ok := fields["addresses"]
	if !ok {
		return fmt.Errorf("addresses is required")
	}
	addresses, err := buildAddresses(addressesField)
	if err != nil {
		return fmt.Errorf("failed to build addresses: %w", err)
	}
	if len(addresses) == 0 {
		return fmt.Errorf("at least one address is required")
	}
	create.Addresses = addresses

	// Ports (required)
	portsField, ok := fields["ports"]
	if !ok {
		return fmt.Errorf("ports is required")
	}
	ports, err := buildPorts(portsField)
	if err != nil {
		return fmt.Errorf("failed to build ports: %w", err)
	}
	if len(ports) == 0 {
		return fmt.Errorf("at least one port or range of ports is required")
	}
	create.Ports = ports

	// Optional fields
	if desc, ok := fields["description"].(string); ok {
		create.Description = &desc
	}

	if labelsMap := extractLabelsField(fields); labelsMap != nil {
		create.Labels = labelsMap
	}

	return nil
}

// MapUpdateFields maps the fields to update into an UpdateEventGatewayListenerRequest
func (a *EventGatewayListenerAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fieldsToUpdate map[string]any,
	update *kkComps.UpdateEventGatewayListenerRequest,
	_ map[string]string,
) error {
	// Required fields - always sent even if not changed
	if name, ok := fieldsToUpdate["name"].(string); ok {
		update.Name = name
	}
	if addressesField, ok := fieldsToUpdate["addresses"]; ok {
		addresses, err := buildAddresses(addressesField)
		if err != nil {
			return fmt.Errorf("failed to build addresses: %w", err)
		} else if len(addresses) == 0 {
			return fmt.Errorf("at least one address is required")
		}
		update.Addresses = addresses
	}
	if portsField, ok := fieldsToUpdate["ports"]; ok {
		ports, err := buildPorts(portsField)
		if err != nil {
			return fmt.Errorf("failed to build ports: %w", err)
		} else if len(ports) == 0 {
			return fmt.Errorf("at least one port or range of ports is required")
		}
		update.Ports = ports
	}

	// Optional fields
	if description, ok := fieldsToUpdate["description"]; ok {
		if desc, ok := description.(string); ok {
			update.Description = &desc
		} else if description == nil {
			// Handle nil description (clear it)
			emptyStr := ""
			update.Description = &emptyStr
		}
	}

	if labelsMap := extractLabelsField(fieldsToUpdate); labelsMap != nil {
		update.Labels = labelsMap
	}

	return nil
}

// Create creates a new listener
func (a *EventGatewayListenerAdapter) Create(
	ctx context.Context,
	req kkComps.CreateEventGatewayListenerRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.CreateEventGatewayListener(ctx, gatewayID, req, namespace)
}

// Update updates an existing listener
func (a *EventGatewayListenerAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateEventGatewayListenerRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.UpdateEventGatewayListener(ctx, gatewayID, id, req, namespace)
}

// Delete deletes a listener
func (a *EventGatewayListenerAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}

	return a.client.DeleteEventGatewayListener(ctx, gatewayID, id)
}

// GetByID gets a listener by ID
func (a *EventGatewayListenerAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}

	listener, err := a.client.GetEventGatewayListener(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if listener == nil {
		return nil, nil
	}

	return &EventGatewayListenerResourceInfo{listener: listener}, nil
}

// GetByName is not supported for listeners (they are looked up by name within a gateway)
func (a *EventGatewayListenerAdapter) GetByName(
	_ context.Context,
	_ string,
) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for event gateway listeners")
}

// ResourceType returns the resource type string
func (a *EventGatewayListenerAdapter) ResourceType() string {
	return planner.ResourceTypeEventGatewayListener
}

// RequiredFields returns the list of required fields for this resource
func (a *EventGatewayListenerAdapter) RequiredFields() []string {
	return []string{"name", "addresses", "ports"}
}

// SupportsUpdate indicates whether this resource supports update operations
func (a *EventGatewayListenerAdapter) SupportsUpdate() bool {
	return true
}

// getEventGatewayIDFromExecutionContext extracts the event gateway ID from the execution context
func (a *EventGatewayListenerAdapter) getEventGatewayIDFromExecutionContext(
	execCtx *ExecutionContext,
) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context required")
	}

	change := *execCtx.PlannedChange

	// Priority 1: Check References (for new parent)
	if gatewayRef, ok := change.References["event_gateway_id"]; ok && gatewayRef.ID != "" {
		return gatewayRef.ID, nil
	}

	// Priority 2: Check Parent field (for existing parent)
	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("event gateway ID required for listener operations")
}

// EventGatewayListenerResourceInfo wraps an Event Gateway Listener to implement ResourceInfo
type EventGatewayListenerResourceInfo struct {
	listener *state.EventGatewayListener
}

func (e *EventGatewayListenerResourceInfo) GetID() string {
	return e.listener.ID
}

func (e *EventGatewayListenerResourceInfo) GetName() string {
	return e.listener.Name
}

func (e *EventGatewayListenerResourceInfo) GetLabels() map[string]string {
	return e.listener.Labels
}

func (e *EventGatewayListenerResourceInfo) GetNormalizedLabels() map[string]string {
	return e.listener.NormalizedLabels
}

// buildAddresses constructs addresses slice from a slice or []string
func buildAddresses(field any) ([]string, error) {
	// If it's already a string slice, return it directly
	if addresses, ok := field.([]string); ok {
		return addresses, nil
	}

	// Otherwise, build from []any
	addressSlice, ok := field.([]any)
	if !ok {
		return nil, fmt.Errorf("addresses must be an array, got %T", field)
	}

	result := make([]string, 0, len(addressSlice))
	for i, addr := range addressSlice {
		addrStr, ok := addr.(string)
		if !ok {
			return nil, fmt.Errorf("addresses[%d] must be a string, got %T", i, addr)
		}
		result = append(result, addrStr)
	}

	return result, nil
}

// buildPorts constructs EventGatewayListenerPort slice from a slice of strings
// Note: ports are normalized to strings before reaching this function
func buildPorts(field any) ([]kkComps.EventGatewayListenerPort, error) {
	// If it's already the SDK type, return it directly
	if ports, ok := field.([]kkComps.EventGatewayListenerPort); ok {
		return ports, nil
	}

	// Handle []string (all ports normalized to strings)
	if strPorts, ok := field.([]string); ok {
		result := make([]kkComps.EventGatewayListenerPort, 0, len(strPorts))
		for _, portStr := range strPorts {
			result = append(result, kkComps.CreateEventGatewayListenerPortStr(portStr))
		}
		return result, nil
	}

	// Handle []any where each element is a string
	if portSlice, ok := field.([]any); ok {
		result := make([]kkComps.EventGatewayListenerPort, 0, len(portSlice))
		for i, port := range portSlice {
			portStr, ok := port.(string)
			if !ok {
				return nil, fmt.Errorf("ports[%d] must be a string, got %T", i, port)
			}
			result = append(result, kkComps.CreateEventGatewayListenerPortStr(portStr))
		}
		return result, nil
	}

	// Handle []interface{} where each element is a string
	if portSlice, ok := field.([]interface{}); ok {
		result := make([]kkComps.EventGatewayListenerPort, 0, len(portSlice))
		for i, port := range portSlice {
			portStr, ok := port.(string)
			if !ok {
				return nil, fmt.Errorf("ports[%d] must be a string, got %T", i, port)
			}
			result = append(result, kkComps.CreateEventGatewayListenerPortStr(portStr))
		}
		return result, nil
	}

	return nil, fmt.Errorf("ports must be an array, got %T", field)
}
