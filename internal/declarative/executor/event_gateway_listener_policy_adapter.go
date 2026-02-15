package executor

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// EventGatewayListenerPolicyAdapter implements ResourceOperations for Event Gateway Listener Policy resources.
// Listener policies are grandchildren: Event Gateway → Listener → Listener Policy.
// Both the gateway ID and listener ID are required for all operations.
type EventGatewayListenerPolicyAdapter struct {
	client *state.Client
}

// NewEventGatewayListenerPolicyAdapter creates a new EventGatewayListenerPolicyAdapter
func NewEventGatewayListenerPolicyAdapter(client *state.Client) *EventGatewayListenerPolicyAdapter {
	return &EventGatewayListenerPolicyAdapter{
		client: client,
	}
}

// MapCreateFields maps fields to EventGatewayListenerPolicyCreate (union type).
// The fields map should contain the full union-typed policy body (serialized from the resource).
func (a *EventGatewayListenerPolicyAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.EventGatewayListenerPolicyCreate,
) error {
	// The listener policy is a union type. We serialize the fields to JSON
	// and delegate unmarshaling to the SDK union type which handles discriminating
	// between TLSServer and ForwardToVirtualCluster.
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("failed to marshal listener policy fields: %w", err)
	}

	if err := json.Unmarshal(data, create); err != nil {
		return fmt.Errorf("failed to unmarshal listener policy create request: %w", err)
	}

	return nil
}

// MapUpdateFields maps the fields into an EventGatewayListenerPolicyUpdate (union type).
// The update type uses SensitiveDataAware variants where applicable.
func (a *EventGatewayListenerPolicyAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fieldsToUpdate map[string]any,
	update *kkComps.EventGatewayListenerPolicyUpdate,
	_ map[string]string,
) error {
	// Serialize fields to JSON and delegate to the SDK union type unmarshaler.
	// The Update union type uses EventGatewayTLSListenerSensitiveDataAwarePolicy
	// instead of EventGatewayTLSListenerPolicy for the TLS variant.
	data, err := json.Marshal(fieldsToUpdate)
	if err != nil {
		return fmt.Errorf("failed to marshal listener policy update fields: %w", err)
	}

	if err := json.Unmarshal(data, update); err != nil {
		return fmt.Errorf("failed to unmarshal listener policy update request: %w", err)
	}

	return nil
}

// Create creates a new listener policy
func (a *EventGatewayListenerPolicyAdapter) Create(
	ctx context.Context,
	req kkComps.EventGatewayListenerPolicyCreate,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, listenerID, err := a.getGatewayAndListenerIDs(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.CreateEventGatewayListenerPolicy(ctx, gatewayID, listenerID, req, namespace)
}

// Update updates an existing listener policy
func (a *EventGatewayListenerPolicyAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.EventGatewayListenerPolicyUpdate,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, listenerID, err := a.getGatewayAndListenerIDs(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.UpdateEventGatewayListenerPolicy(ctx, gatewayID, listenerID, id, req, namespace)
}

// Delete deletes a listener policy
func (a *EventGatewayListenerPolicyAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	gatewayID, listenerID, err := a.getGatewayAndListenerIDs(execCtx)
	if err != nil {
		return err
	}

	return a.client.DeleteEventGatewayListenerPolicy(ctx, gatewayID, listenerID, id)
}

// GetByID gets a listener policy by ID
func (a *EventGatewayListenerPolicyAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, listenerID, err := a.getGatewayAndListenerIDs(execCtx)
	if err != nil {
		return nil, err
	}

	policy, err := a.client.GetEventGatewayListenerPolicy(ctx, gatewayID, listenerID, id)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, nil
	}

	return &EventGatewayListenerPolicyResourceInfo{policy: policy}, nil
}

// GetByName is not supported for listener policies
func (a *EventGatewayListenerPolicyAdapter) GetByName(
	_ context.Context,
	_ string,
) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for event gateway listener policies")
}

// ResourceType returns the resource type string
func (a *EventGatewayListenerPolicyAdapter) ResourceType() string {
	return planner.ResourceTypeEventGatewayListenerPolicy
}

// RequiredFields returns the list of required fields for this resource.
// For union types, the required fields depend on which variant is set,
// so validation is delegated to the SDK type.
func (a *EventGatewayListenerPolicyAdapter) RequiredFields() []string {
	return []string{"type"}
}

// SupportsUpdate indicates whether this resource supports update operations
func (a *EventGatewayListenerPolicyAdapter) SupportsUpdate() bool {
	return true
}

// getGatewayAndListenerIDs extracts both the event gateway ID and listener ID from the execution context.
// Listener policies are grandchildren and require both parent IDs.
// The listener ID comes from Parent or References["event_gateway_listener_id"].
// The gateway ID comes from References["event_gateway_id"].
func (a *EventGatewayListenerPolicyAdapter) getGatewayAndListenerIDs(
	execCtx *ExecutionContext,
) (string, string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", "", fmt.Errorf("execution context required")
	}

	change := *execCtx.PlannedChange

	var gatewayID, listenerID string

	// Resolve listener ID: Priority 1 = References, Priority 2 = Parent
	if listenerRef, ok := change.References["event_gateway_listener_id"]; ok && listenerRef.ID != "" {
		listenerID = listenerRef.ID
	}
	if listenerID == "" && change.Parent != nil && change.Parent.ID != "" {
		listenerID = change.Parent.ID
	}
	if listenerID == "" {
		return "", "", fmt.Errorf("event gateway listener ID required for listener policy operations")
	}

	// Resolve gateway ID from References
	if gatewayRef, ok := change.References["event_gateway_id"]; ok && gatewayRef.ID != "" {
		gatewayID = gatewayRef.ID
	}
	if gatewayID == "" {
		return "", "", fmt.Errorf("event gateway ID required for listener policy operations")
	}

	return gatewayID, listenerID, nil
}

// EventGatewayListenerPolicyResourceInfo wraps a Listener Policy to implement ResourceInfo
type EventGatewayListenerPolicyResourceInfo struct {
	policy *state.EventGatewayListenerPolicyInfo
}

func (e *EventGatewayListenerPolicyResourceInfo) GetID() string {
	return e.policy.ID
}

func (e *EventGatewayListenerPolicyResourceInfo) GetName() string {
	if e.policy.Name != nil {
		return *e.policy.Name
	}
	return ""
}

func (e *EventGatewayListenerPolicyResourceInfo) GetLabels() map[string]string {
	return e.policy.Labels
}

func (e *EventGatewayListenerPolicyResourceInfo) GetNormalizedLabels() map[string]string {
	return e.policy.NormalizedLabels
}
