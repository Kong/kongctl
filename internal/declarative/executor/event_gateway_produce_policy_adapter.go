package executor

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// EventGatewayProducePolicyAdapter implements ResourceOperations for Event Gateway Produce Policy resources.
// Produce policies are grandchildren: Event Gateway → Virtual Cluster → Produce Policy.
// Both the gateway ID and virtual cluster ID are required for all operations.
type EventGatewayProducePolicyAdapter struct {
	client *state.Client
}

// NewEventGatewayProducePolicyAdapter creates a new EventGatewayProducePolicyAdapter.
func NewEventGatewayProducePolicyAdapter(client *state.Client) *EventGatewayProducePolicyAdapter {
	return &EventGatewayProducePolicyAdapter{client: client}
}

// MapCreateFields maps the planner fields into EventGatewayProducePolicyCreate.
// The fields map contains the full union-typed policy body; the SDK union type handles
// discriminating on the "type" field.
func (a *EventGatewayProducePolicyAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.EventGatewayProducePolicyCreate,
) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("failed to marshal produce policy fields: %w", err)
	}
	if err := json.Unmarshal(data, create); err != nil {
		return fmt.Errorf("failed to unmarshal produce policy create request: %w", err)
	}
	return nil
}

// MapUpdateFields maps the planner fields into EventGatewayProducePolicyUpdate.
func (a *EventGatewayProducePolicyAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fieldsToUpdate map[string]any,
	update *kkComps.EventGatewayProducePolicyUpdate,
	_ map[string]string,
) error {
	data, err := json.Marshal(fieldsToUpdate)
	if err != nil {
		return fmt.Errorf("failed to marshal produce policy update fields: %w", err)
	}
	if err := json.Unmarshal(data, update); err != nil {
		return fmt.Errorf("failed to unmarshal produce policy update request: %w", err)
	}
	return nil
}

// Create creates a new produce policy.
func (a *EventGatewayProducePolicyAdapter) Create(
	ctx context.Context,
	req kkComps.EventGatewayProducePolicyCreate,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, virtualClusterID, err := a.getGatewayAndVirtualClusterIDs(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateEventGatewayVirtualClusterProducePolicy(ctx, gatewayID, virtualClusterID, req, namespace)
}

// Update updates an existing produce policy.
func (a *EventGatewayProducePolicyAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.EventGatewayProducePolicyUpdate,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, virtualClusterID, err := a.getGatewayAndVirtualClusterIDs(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateEventGatewayVirtualClusterProducePolicy(ctx, gatewayID, virtualClusterID, id, req, namespace)
}

// Delete deletes a produce policy.
func (a *EventGatewayProducePolicyAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	gatewayID, virtualClusterID, err := a.getGatewayAndVirtualClusterIDs(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteEventGatewayVirtualClusterProducePolicy(ctx, gatewayID, virtualClusterID, id)
}

// GetByID gets a produce policy by ID.
func (a *EventGatewayProducePolicyAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, virtualClusterID, err := a.getGatewayAndVirtualClusterIDs(execCtx)
	if err != nil {
		return nil, err
	}
	policy, err := a.client.GetEventGatewayVirtualClusterProducePolicy(ctx, gatewayID, virtualClusterID, id)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, nil
	}
	return &EventGatewayProducePolicyResourceInfo{policy: policy}, nil
}

// GetByName is not supported for produce policies (API does not support name filtering).
func (a *EventGatewayProducePolicyAdapter) GetByName(
	_ context.Context,
	_ string,
) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for event gateway produce policies")
}

// ResourceType returns the resource type string.
func (a *EventGatewayProducePolicyAdapter) ResourceType() string {
	return planner.ResourceTypeEventGatewayProducePolicy
}

// RequiredFields returns the required fields (validation is handled by the SDK union type).
func (a *EventGatewayProducePolicyAdapter) RequiredFields() []string {
	return []string{planner.FieldType}
}

// SupportsUpdate indicates whether this resource supports update operations.
func (a *EventGatewayProducePolicyAdapter) SupportsUpdate() bool {
	return true
}

// getGatewayAndVirtualClusterIDs extracts both the event gateway ID and virtual cluster ID
// from the execution context. Produce policies are grandchildren and require both parent IDs.
func (a *EventGatewayProducePolicyAdapter) getGatewayAndVirtualClusterIDs(
	execCtx *ExecutionContext,
) (string, string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", "", fmt.Errorf("execution context required")
	}

	change := *execCtx.PlannedChange

	var gatewayID, virtualClusterID string

	// Resolve virtual cluster ID: Priority 1 = References, Priority 2 = Parent
	if vcRef, ok := change.References[planner.FieldEventGatewayVirtualClusterID]; ok && vcRef.ID != "" {
		virtualClusterID = vcRef.ID
	}
	if virtualClusterID == "" && change.Parent != nil && change.Parent.ID != "" {
		virtualClusterID = change.Parent.ID
	}
	if virtualClusterID == "" {
		return "", "", fmt.Errorf("event gateway virtual cluster ID required for produce policy operations")
	}

	// Resolve gateway ID from References
	if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID != "" {
		gatewayID = gatewayRef.ID
	}
	if gatewayID == "" {
		return "", "", fmt.Errorf("event gateway ID required for produce policy operations")
	}

	return gatewayID, virtualClusterID, nil
}

// EventGatewayProducePolicyResourceInfo wraps a ProducePolicyInfo to implement ResourceInfo.
type EventGatewayProducePolicyResourceInfo struct {
	policy *state.EventGatewayVirtualClusterProducePolicyInfo
}

func (e *EventGatewayProducePolicyResourceInfo) GetID() string {
	return e.policy.ID
}

func (e *EventGatewayProducePolicyResourceInfo) GetName() string {
	if e.policy.Name != nil {
		return *e.policy.Name
	}
	return ""
}

func (e *EventGatewayProducePolicyResourceInfo) GetLabels() map[string]string {
	return e.policy.Labels
}

func (e *EventGatewayProducePolicyResourceInfo) GetNormalizedLabels() map[string]string {
	return e.policy.Labels
}
