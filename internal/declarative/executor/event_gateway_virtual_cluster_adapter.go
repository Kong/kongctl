package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// EventGatewayVirtualClusterAdapter implements ResourceOperations for Event Gateway Virtual Cluster resources
type EventGatewayVirtualClusterAdapter struct {
	client *state.Client
}

// NewEventGatewayVirtualClusterAdapter creates a new EventGatewayVirtualClusterAdapter
func NewEventGatewayVirtualClusterAdapter(client *state.Client) *EventGatewayVirtualClusterAdapter {
	return &EventGatewayVirtualClusterAdapter{
		client: client,
	}
}

// MapCreateFields maps fields to CreateVirtualClusterRequest
func (a *EventGatewayVirtualClusterAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateVirtualClusterRequest,
) error {
	// Required fields
	name, ok := fields["name"].(string)
	if !ok {
		return fmt.Errorf("name is required")
	}
	create.Name = name

	// Destination (required)
	destination, ok := fields["destination"].(kkComps.BackendClusterReferenceModify)
	if !ok {
		return fmt.Errorf("destination is required")
	}
	create.Destination = destination

	// Authentication (required)
	authentication, ok := fields["authentication"].([]kkComps.VirtualClusterAuthenticationScheme)
	if !ok {
		return fmt.Errorf("authentication is required")
	}
	create.Authentication = authentication

	// ACL Mode (required)
	aclMode, ok := fields["acl_mode"].(kkComps.VirtualClusterACLMode)
	if !ok {
		return fmt.Errorf("acl_mode is required")
	}
	create.ACLMode = aclMode

	// DNS Label (required)
	dnsLabel, ok := fields["dns_label"].(string)
	if !ok {
		return fmt.Errorf("dns_label is required")
	}
	create.DNSLabel = dnsLabel

	// Optional fields
	if desc, ok := fields["description"].(string); ok {
		create.Description = &desc
	}

	if namespace, ok := fields["namespace"].(*kkComps.VirtualClusterNamespace); ok {
		create.Namespace = namespace
	}

	if labelsMap := extractLabelsField(fields, "labels"); labelsMap != nil {
		create.Labels = labelsMap
	}

	return nil
}

// MapUpdateFields maps the fields to update into an UpdateVirtualClusterRequest
func (a *EventGatewayVirtualClusterAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fieldsToUpdate map[string]any,
	update *kkComps.UpdateVirtualClusterRequest,
	_ map[string]string,
) error {
	// Required fields - always sent even if not changed
	if name, ok := fieldsToUpdate["name"].(string); ok {
		update.Name = name
	}
	if destination, ok := fieldsToUpdate["destination"].(kkComps.BackendClusterReferenceModify); ok {
		update.Destination = destination
	}
	if aclMode, ok := fieldsToUpdate["acl_mode"].(kkComps.VirtualClusterACLMode); ok {
		update.ACLMode = aclMode
	}
	if dnsLabel, ok := fieldsToUpdate["dns_label"].(string); ok {
		update.DNSLabel = dnsLabel
	}

	// Authentication requires conversion from Scheme to SensitiveDataAwareScheme
	if authentication, ok := fieldsToUpdate["authentication"].([]kkComps.VirtualClusterAuthenticationScheme); ok {
		// Convert to SensitiveDataAwareScheme
		sensitiveAuth := make([]kkComps.VirtualClusterAuthenticationSensitiveDataAwareScheme, len(authentication))
		for i, auth := range authentication {
			// This is a simplified conversion - in production might need more complete logic
			sensitiveAuth[i] = kkComps.VirtualClusterAuthenticationSensitiveDataAwareScheme{
				Type: kkComps.VirtualClusterAuthenticationSensitiveDataAwareSchemeType(auth.Type),
			}
		}
		update.Authentication = sensitiveAuth
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

	if namespace, ok := fieldsToUpdate["namespace"].(*kkComps.VirtualClusterNamespace); ok {
		update.Namespace = namespace
	}

	if labels, ok := fieldsToUpdate["labels"].(map[string]string); ok {
		update.Labels = labels
	}

	return nil
}

// Create creates a new virtual cluster
func (a *EventGatewayVirtualClusterAdapter) Create(
	ctx context.Context,
	req kkComps.CreateVirtualClusterRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.CreateEventGatewayVirtualCluster(ctx, gatewayID, req, namespace)
}

// Update updates an existing virtual cluster
func (a *EventGatewayVirtualClusterAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateVirtualClusterRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.UpdateEventGatewayVirtualCluster(ctx, gatewayID, id, req, namespace)
}

// Delete deletes a virtual cluster
func (a *EventGatewayVirtualClusterAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}

	return a.client.DeleteEventGatewayVirtualCluster(ctx, gatewayID, id)
}

// GetByID gets a virtual cluster by ID
func (a *EventGatewayVirtualClusterAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}

	cluster, err := a.client.GetEventGatewayVirtualCluster(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, nil
	}

	return &EventGatewayVirtualClusterResourceInfo{virtualCluster: cluster}, nil
}

// GetByName is not supported for virtual clusters (they are looked up by name within a gateway)
func (a *EventGatewayVirtualClusterAdapter) GetByName(
	_ context.Context,
	_ string,
) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for event gateway virtual clusters")
}

// ResourceType returns the resource type string
func (a *EventGatewayVirtualClusterAdapter) ResourceType() string {
	return planner.ResourceTypeEventGatewayVirtualCluster
}

// RequiredFields returns the list of required fields for this resource
func (a *EventGatewayVirtualClusterAdapter) RequiredFields() []string {
	return []string{"name", "destination", "authentication", "acl_mode", "dns_label"}
}

// SupportsUpdate indicates whether this resource supports update operations
func (a *EventGatewayVirtualClusterAdapter) SupportsUpdate() bool {
	return true
}

// getEventGatewayIDFromExecutionContext extracts the event gateway ID from the execution context
func (a *EventGatewayVirtualClusterAdapter) getEventGatewayIDFromExecutionContext(
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

	return "", fmt.Errorf("event gateway ID required for virtual cluster operations")
}

// EventGatewayVirtualClusterResourceInfo wraps an Event Gateway Virtual Cluster to implement ResourceInfo
type EventGatewayVirtualClusterResourceInfo struct {
	virtualCluster *state.EventGatewayVirtualCluster
}

func (e *EventGatewayVirtualClusterResourceInfo) GetID() string {
	return e.virtualCluster.ID
}

func (e *EventGatewayVirtualClusterResourceInfo) GetName() string {
	return e.virtualCluster.Name
}

func (e *EventGatewayVirtualClusterResourceInfo) GetLabels() map[string]string {
	return e.virtualCluster.Labels
}

func (e *EventGatewayVirtualClusterResourceInfo) GetNormalizedLabels() map[string]string {
	return e.virtualCluster.NormalizedLabels
}
