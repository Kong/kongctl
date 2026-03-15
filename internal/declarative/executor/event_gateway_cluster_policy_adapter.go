package executor

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// EventGatewayClusterPolicyAdapter implements ResourceOperations for Event Gateway Cluster Policy resources.
// Cluster policies are grandchildren: Event Gateway → Virtual Cluster → Cluster Policy.
// Both the gateway ID and virtual cluster ID are required for all operations.
type EventGatewayClusterPolicyAdapter struct {
	client *state.Client
}

// NewEventGatewayClusterPolicyAdapter creates a new EventGatewayClusterPolicyAdapter
func NewEventGatewayClusterPolicyAdapter(client *state.Client) *EventGatewayClusterPolicyAdapter {
	return &EventGatewayClusterPolicyAdapter{
		client: client,
	}
}

// MapCreateFields maps fields to EventGatewayClusterPolicyModify (union type).
// The fields map should contain the full union-typed policy body (serialized from the resource).
func (a *EventGatewayClusterPolicyAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.EventGatewayClusterPolicyModify,
) error {
	// The cluster policy is a union type. We serialize the fields to JSON
	// and delegate unmarshaling to the SDK union type which handles discriminating
	// based on the "type" field (currently only "acls" is supported).
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("failed to marshal cluster policy fields: %w", err)
	}

	if err := json.Unmarshal(data, create); err != nil {
		return fmt.Errorf("failed to unmarshal cluster policy create request: %w", err)
	}

	return nil
}

// MapUpdateFields maps the fields into an EventGatewayClusterPolicyModify (union type).
func (a *EventGatewayClusterPolicyAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fieldsToUpdate map[string]any,
	update *kkComps.EventGatewayClusterPolicyModify,
	_ map[string]string,
) error {
	// Serialize fields to JSON and delegate to the SDK union type unmarshaler.
	data, err := json.Marshal(fieldsToUpdate)
	if err != nil {
		return fmt.Errorf("failed to marshal cluster policy update fields: %w", err)
	}

	if err := json.Unmarshal(data, update); err != nil {
		return fmt.Errorf("failed to unmarshal cluster policy update request: %w", err)
	}

	return nil
}

// Create creates a new cluster policy
func (a *EventGatewayClusterPolicyAdapter) Create(
	ctx context.Context,
	req kkComps.EventGatewayClusterPolicyModify,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, virtualClusterID, err := a.getGatewayAndVirtualClusterIDs(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.CreateEventGatewayClusterPolicy(ctx, gatewayID, virtualClusterID, req, namespace)
}

// Update updates an existing cluster policy
func (a *EventGatewayClusterPolicyAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.EventGatewayClusterPolicyModify,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, virtualClusterID, err := a.getGatewayAndVirtualClusterIDs(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.UpdateEventGatewayClusterPolicy(ctx, gatewayID, virtualClusterID, id, req, namespace)
}

// Delete deletes a cluster policy
func (a *EventGatewayClusterPolicyAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	gatewayID, virtualClusterID, err := a.getGatewayAndVirtualClusterIDs(execCtx)
	if err != nil {
		return err
	}

	return a.client.DeleteEventGatewayClusterPolicy(ctx, gatewayID, virtualClusterID, id)
}

// GetByID gets a cluster policy by ID
func (a *EventGatewayClusterPolicyAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, virtualClusterID, err := a.getGatewayAndVirtualClusterIDs(execCtx)
	if err != nil {
		return nil, err
	}

	policy, err := a.client.GetEventGatewayClusterPolicy(ctx, gatewayID, virtualClusterID, id)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, nil
	}

	return &EventGatewayClusterPolicyResourceInfo{policy: policy}, nil
}

// GetByName is not supported for cluster policies
func (a *EventGatewayClusterPolicyAdapter) GetByName(
	_ context.Context,
	_ string,
) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for event gateway cluster policies")
}

// ResourceType returns the resource type string
func (a *EventGatewayClusterPolicyAdapter) ResourceType() string {
	return planner.ResourceTypeEventGatewayClusterPolicy
}

// RequiredFields returns the list of required fields for this resource.
// For union types, the required fields depend on which variant is set,
// so validation is delegated to the SDK type.
func (a *EventGatewayClusterPolicyAdapter) RequiredFields() []string {
	return []string{"type"}
}

// SupportsUpdate indicates whether this resource supports update operations
func (a *EventGatewayClusterPolicyAdapter) SupportsUpdate() bool {
	return true
}

// getGatewayAndVirtualClusterIDs extracts both the event gateway ID and virtual cluster ID
// from the execution context.
// Cluster policies are grandchildren and require both parent IDs.
// The virtual cluster ID comes from Parent or References["event_gateway_virtual_cluster_id"].
// The gateway ID comes from References["event_gateway_id"].
func (a *EventGatewayClusterPolicyAdapter) getGatewayAndVirtualClusterIDs(
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
		return "", "", fmt.Errorf("event gateway virtual cluster ID required for cluster policy operations")
	}

	// Resolve gateway ID from References
	if gatewayRef, ok := change.References["event_gateway_id"]; ok && gatewayRef.ID != "" {
		gatewayID = gatewayRef.ID
	}
	if gatewayID == "" {
		return "", "", fmt.Errorf("event gateway ID required for cluster policy operations")
	}

	return gatewayID, virtualClusterID, nil
}

// EventGatewayClusterPolicyResourceInfo wraps a Cluster Policy to implement ResourceInfo
type EventGatewayClusterPolicyResourceInfo struct {
	policy *state.EventGatewayClusterPolicyInfo
}

func (e *EventGatewayClusterPolicyResourceInfo) GetID() string {
	return e.policy.ID
}

func (e *EventGatewayClusterPolicyResourceInfo) GetName() string {
	if e.policy.Name != nil {
		return *e.policy.Name
	}
	return ""
}

func (e *EventGatewayClusterPolicyResourceInfo) GetLabels() map[string]string {
	return e.policy.Labels
}

func (e *EventGatewayClusterPolicyResourceInfo) GetNormalizedLabels() map[string]string {
	return e.policy.NormalizedLabels
}
