package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// EventGatewayDataPlaneCertificateAdapter implements ResourceOperations for
// Event Gateway Data Plane Certificate resources
type EventGatewayDataPlaneCertificateAdapter struct {
	client *state.Client
}

// NewEventGatewayDataPlaneCertificateAdapter creates a new EventGatewayDataPlaneCertificateAdapter
func NewEventGatewayDataPlaneCertificateAdapter(client *state.Client) *EventGatewayDataPlaneCertificateAdapter {
	return &EventGatewayDataPlaneCertificateAdapter{
		client: client,
	}
}

// MapCreateFields maps fields to CreateEventGatewayDataPlaneCertificateRequest
func (a *EventGatewayDataPlaneCertificateAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateEventGatewayDataPlaneCertificateRequest,
) error {
	// Required fields
	certificate, ok := fields["certificate"].(string)
	if !ok {
		return fmt.Errorf("certificate is required")
	}
	create.Certificate = certificate

	// Optional fields
	if name, ok := fields["name"].(string); ok {
		create.Name = &name
	}

	if desc, ok := fields["description"].(string); ok {
		create.Description = &desc
	}

	return nil
}

// MapUpdateFields maps the fields to update into an UpdateEventGatewayDataPlaneCertificateRequest
func (a *EventGatewayDataPlaneCertificateAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fieldsToUpdate map[string]any,
	update *kkComps.UpdateEventGatewayDataPlaneCertificateRequest,
	_ map[string]string,
) error {
	// Certificate is required for updates
	if certificate, ok := fieldsToUpdate["certificate"].(string); ok {
		update.Certificate = certificate
	}

	// Optional fields
	if name, ok := fieldsToUpdate["name"].(string); ok {
		update.Name = &name
	}

	if description, ok := fieldsToUpdate["description"]; ok {
		if desc, ok := description.(string); ok {
			update.Description = &desc
		} else if description == nil {
			// Handle nil description (clear it)
			emptyStr := ""
			update.Description = &emptyStr
		}
	}

	return nil
}

// Create creates a new data plane certificate
func (a *EventGatewayDataPlaneCertificateAdapter) Create(
	ctx context.Context,
	req kkComps.CreateEventGatewayDataPlaneCertificateRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.CreateEventGatewayDataPlaneCertificate(ctx, gatewayID, req, namespace)
}

// Update updates an existing data plane certificate
func (a *EventGatewayDataPlaneCertificateAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateEventGatewayDataPlaneCertificateRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.UpdateEventGatewayDataPlaneCertificate(ctx, gatewayID, id, req, namespace)
}

// Delete deletes a data plane certificate
func (a *EventGatewayDataPlaneCertificateAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}

	return a.client.DeleteEventGatewayDataPlaneCertificate(ctx, gatewayID, id)
}

// GetByID gets a data plane certificate by ID
func (a *EventGatewayDataPlaneCertificateAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}

	cert, err := a.client.GetEventGatewayDataPlaneCertificate(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if cert == nil {
		return nil, nil
	}

	return &EventGatewayDataPlaneCertificateResourceInfo{cert: cert}, nil
}

// GetByName is not supported for data plane certificates
// (they are looked up by name within a gateway)
func (a *EventGatewayDataPlaneCertificateAdapter) GetByName(
	_ context.Context,
	_ string,
) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for event gateway data plane certificates")
}

// ResourceType returns the resource type string
func (a *EventGatewayDataPlaneCertificateAdapter) ResourceType() string {
	return planner.ResourceTypeEventGatewayDataPlaneCertificate
}

// RequiredFields returns the list of required fields for this resource
func (a *EventGatewayDataPlaneCertificateAdapter) RequiredFields() []string {
	return []string{"certificate"}
}

// SupportsUpdate indicates whether this resource supports update operations
func (a *EventGatewayDataPlaneCertificateAdapter) SupportsUpdate() bool {
	return true
}

// getEventGatewayIDFromExecutionContext extracts the event gateway ID from the execution context
func (a *EventGatewayDataPlaneCertificateAdapter) getEventGatewayIDFromExecutionContext(
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

	return "", fmt.Errorf("event gateway ID required for data plane certificate operations")
}

// EventGatewayDataPlaneCertificateResourceInfo wraps an Event Gateway Data Plane Certificate
// to implement ResourceInfo
type EventGatewayDataPlaneCertificateResourceInfo struct {
	cert *state.EventGatewayDataPlaneCertificate
}

func (e *EventGatewayDataPlaneCertificateResourceInfo) GetID() string {
	return e.cert.ID
}

func (e *EventGatewayDataPlaneCertificateResourceInfo) GetName() string {
	if e.cert.Name != nil {
		return *e.cert.Name
	}
	return ""
}

func (e *EventGatewayDataPlaneCertificateResourceInfo) GetLabels() map[string]string {
	// Data plane certificates don't have labels
	return nil
}

func (e *EventGatewayDataPlaneCertificateResourceInfo) GetNormalizedLabels() map[string]string {
	// Data plane certificates don't have labels
	return nil
}
