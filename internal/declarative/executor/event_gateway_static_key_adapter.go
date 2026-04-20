package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// EventGatewayStaticKeyAdapter implements ResourceOperations for Event Gateway Static Key resources.
// Static keys do not support update operations – use delete + create instead.
type EventGatewayStaticKeyAdapter struct {
	client *state.Client
}

// NewEventGatewayStaticKeyAdapter creates a new EventGatewayStaticKeyAdapter.
func NewEventGatewayStaticKeyAdapter(client *state.Client) *EventGatewayStaticKeyAdapter {
	return &EventGatewayStaticKeyAdapter{client: client}
}

// MapCreateFields maps fields to EventGatewayStaticKeyCreate.
func (a *EventGatewayStaticKeyAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.EventGatewayStaticKeyCreate,
) error {
	name, ok := fields["name"].(string)
	if !ok || name == "" {
		return fmt.Errorf("name is required")
	}
	create.Name = name

	value, ok := fields["value"].(string)
	if !ok || value == "" {
		return fmt.Errorf("value is required")
	}
	create.Value = value

	if desc, ok := fields["description"].(string); ok {
		create.Description = &desc
	}

	if labelsRaw, ok := fields["labels"].(map[string]any); ok {
		labels := make(map[string]string, len(labelsRaw))
		for k, v := range labelsRaw {
			if sv, ok := v.(string); ok {
				labels[k] = sv
			}
		}
		create.Labels = labels
	} else if labels, ok := fields["labels"].(map[string]string); ok {
		create.Labels = labels
	}

	return nil
}

// MapUpdateFields is not supported for static keys.
func (a *EventGatewayStaticKeyAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	_ map[string]any,
	_ *kkComps.EventGatewayStaticKeyCreate, // reuse create type as sentinel
	_ map[string]string,
) error {
	return fmt.Errorf("event gateway static keys do not support update operations")
}

// Create creates a new static key.
func (a *EventGatewayStaticKeyAdapter) Create(
	ctx context.Context,
	req kkComps.EventGatewayStaticKeyCreate,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.CreateEventGatewayStaticKey(ctx, gatewayID, req, namespace)
}

// Update is not supported for static keys.
func (a *EventGatewayStaticKeyAdapter) Update(
	_ context.Context,
	_ string,
	_ kkComps.EventGatewayStaticKeyCreate,
	_ string,
	_ *ExecutionContext,
) (string, error) {
	return "", fmt.Errorf("event gateway static keys do not support update operations")
}

// Delete deletes a static key.
func (a *EventGatewayStaticKeyAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}

	return a.client.DeleteEventGatewayStaticKey(ctx, gatewayID, id)
}

// GetByID retrieves a static key by ID.
func (a *EventGatewayStaticKeyAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}

	sk, err := a.client.GetEventGatewayStaticKey(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if sk == nil {
		return nil, nil
	}

	return &EventGatewayStaticKeyResourceInfo{key: sk}, nil
}

// GetByName is not supported for static keys (no direct name-based lookup).
func (a *EventGatewayStaticKeyAdapter) GetByName(
	_ context.Context,
	_ string,
) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for event gateway static keys")
}

// ResourceType returns the resource type string.
func (a *EventGatewayStaticKeyAdapter) ResourceType() string {
	return planner.ResourceTypeEventGatewayStaticKey
}

// RequiredFields returns the list of required fields for this resource.
func (a *EventGatewayStaticKeyAdapter) RequiredFields() []string {
	return []string{"name", "value"}
}

// SupportsUpdate returns false – static keys must be deleted and re-created on change.
func (a *EventGatewayStaticKeyAdapter) SupportsUpdate() bool {
	return false
}

// getEventGatewayIDFromExecutionContext extracts the event gateway ID from the execution context.
func (a *EventGatewayStaticKeyAdapter) getEventGatewayIDFromExecutionContext(
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

	return "", fmt.Errorf("event gateway ID required for static key operations")
}

// EventGatewayStaticKeyResourceInfo wraps an Event Gateway Static Key to implement ResourceInfo.
type EventGatewayStaticKeyResourceInfo struct {
	key *state.EventGatewayStaticKey
}

func (e *EventGatewayStaticKeyResourceInfo) GetID() string {
	return e.key.ID
}

func (e *EventGatewayStaticKeyResourceInfo) GetName() string {
	return e.key.Name
}

func (e *EventGatewayStaticKeyResourceInfo) GetLabels() map[string]string {
	return e.key.Labels
}

func (e *EventGatewayStaticKeyResourceInfo) GetNormalizedLabels() map[string]string {
	return e.key.Labels
}
