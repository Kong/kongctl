package executor

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// EventGatewayConsumePolicyAdapter implements ResourceOperations for Event Gateway Consume Policy resources.
// Consume policies are grandchildren: Event Gateway → Virtual Cluster → Consume Policy.
// Both the gateway ID and virtual cluster ID are required for all operations.
type EventGatewayConsumePolicyAdapter struct {
	client *state.Client
}

// NewEventGatewayConsumePolicyAdapter creates a new EventGatewayConsumePolicyAdapter
func NewEventGatewayConsumePolicyAdapter(client *state.Client) *EventGatewayConsumePolicyAdapter {
	return &EventGatewayConsumePolicyAdapter{
		client: client,
	}
}

// MapCreateFields maps fields to EventGatewayConsumePolicyCreate (union type).
// The fields map should contain the full union-typed policy body (serialized from the resource).
func (a *EventGatewayConsumePolicyAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.EventGatewayConsumePolicyCreate,
) error {
	// The consume policy is a union type. We serialize the fields to JSON
	// and delegate unmarshaling to the SDK union type which handles discriminating
	// based on the "type" field.
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("failed to marshal consume policy fields: %w", err)
	}

	if err := json.Unmarshal(data, create); err != nil {
		return fmt.Errorf("failed to unmarshal consume policy create request: %w", err)
	}

	return nil
}

// MapUpdateFields maps the fields into an EventGatewayConsumePolicyUpdate (union type).
// The update type has the same discriminator as the create type but may use different struct names.
func (a *EventGatewayConsumePolicyAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fieldsToUpdate map[string]any,
	update *kkComps.EventGatewayConsumePolicyUpdate,
	_ map[string]string,
) error {
	// Serialize fields to JSON and delegate to the SDK union type unmarshaler.
	data, err := json.Marshal(fieldsToUpdate)
	if err != nil {
		return fmt.Errorf("failed to marshal consume policy update fields: %w", err)
	}

	if err := json.Unmarshal(data, update); err != nil {
		return fmt.Errorf("failed to unmarshal consume policy update request: %w", err)
	}

	return nil
}

// Create creates a new consume policy
func (a *EventGatewayConsumePolicyAdapter) Create(
	ctx context.Context,
	req kkComps.EventGatewayConsumePolicyCreate,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, virtualClusterID, err := a.getGatewayAndVirtualClusterIDs(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.CreateEventGatewayConsumePolicy(ctx, gatewayID, virtualClusterID, req, namespace)
}

// Update updates an existing consume policy
func (a *EventGatewayConsumePolicyAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.EventGatewayConsumePolicyUpdate,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, virtualClusterID, err := a.getGatewayAndVirtualClusterIDs(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.UpdateEventGatewayConsumePolicy(ctx, gatewayID, virtualClusterID, id, req, namespace)
}

// Delete deletes a consume policy
func (a *EventGatewayConsumePolicyAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	gatewayID, virtualClusterID, err := a.getGatewayAndVirtualClusterIDs(execCtx)
	if err != nil {
		return err
	}

	return a.client.DeleteEventGatewayConsumePolicy(ctx, gatewayID, virtualClusterID, id)
}

// GetByID gets a consume policy by ID
func (a *EventGatewayConsumePolicyAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, virtualClusterID, err := a.getGatewayAndVirtualClusterIDs(execCtx)
	if err != nil {
		return nil, err
	}

	policy, err := a.client.GetEventGatewayConsumePolicy(ctx, gatewayID, virtualClusterID, id)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, nil
	}

	return &EventGatewayConsumePolicyResourceInfo{policy: policy}, nil
}

// GetByName is not supported for consume policies
func (a *EventGatewayConsumePolicyAdapter) GetByName(
	_ context.Context,
	_ string,
) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for event gateway consume policies")
}

// ResourceType returns the resource type string
func (a *EventGatewayConsumePolicyAdapter) ResourceType() string {
	return planner.ResourceTypeEventGatewayConsumePolicy
}

// RequiredFields returns the list of required fields for this resource.
// For union types, the required fields depend on which variant is set,
// so validation is delegated to the SDK type.
func (a *EventGatewayConsumePolicyAdapter) RequiredFields() []string {
	return []string{"type"}
}

// SupportsUpdate indicates whether this resource supports update operations
func (a *EventGatewayConsumePolicyAdapter) SupportsUpdate() bool {
	return true
}

// getGatewayAndVirtualClusterIDs extracts both the event gateway ID and virtual cluster ID
// from the execution context.
// Consume policies are grandchildren and require both parent IDs.
// The virtual cluster ID comes from Parent or References["event_gateway_virtual_cluster_id"].
// The gateway ID comes from References["event_gateway_id"].
func (a *EventGatewayConsumePolicyAdapter) getGatewayAndVirtualClusterIDs(
	execCtx *ExecutionContext,
) (string, string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", "", fmt.Errorf("execution context required")
	}

	change := *execCtx.PlannedChange

	var gatewayID, virtualClusterID string

	// Resolve virtual cluster ID: Priority 1 = References, Priority 2 = Parent
	if vcRef, ok := change.References["event_gateway_virtual_cluster_id"]; ok && vcRef.ID != "" {
		virtualClusterID = vcRef.ID
	}
	if virtualClusterID == "" && change.Parent != nil && change.Parent.ID != "" {
		virtualClusterID = change.Parent.ID
	}
	if virtualClusterID == "" {
		return "", "", fmt.Errorf("event gateway virtual cluster ID required for consume policy operations")
	}

	// Resolve gateway ID from References
	if gatewayRef, ok := change.References["event_gateway_id"]; ok && gatewayRef.ID != "" {
		gatewayID = gatewayRef.ID
	}
	if gatewayID == "" {
		return "", "", fmt.Errorf("event gateway ID required for consume policy operations")
	}

	return gatewayID, virtualClusterID, nil
}

// EventGatewayConsumePolicyResourceInfo wraps a Consume Policy to implement ResourceInfo
type EventGatewayConsumePolicyResourceInfo struct {
	policy *state.EventGatewayConsumePolicyInfo
}

func (e *EventGatewayConsumePolicyResourceInfo) GetID() string {
	return e.policy.ID
}

func (e *EventGatewayConsumePolicyResourceInfo) GetName() string {
	if e.policy.Name != nil {
		return *e.policy.Name
	}
	return ""
}

func (e *EventGatewayConsumePolicyResourceInfo) GetLabels() map[string]string {
	return e.policy.Labels
}

func (e *EventGatewayConsumePolicyResourceInfo) GetNormalizedLabels() map[string]string {
	return e.policy.NormalizedLabels
}
