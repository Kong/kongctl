package executor

import (
	"context"

	"github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

type EventGatewayControlPlaneControlPlaneAdapter struct {
	client *state.Client
}

func NewEventGatewayControlPlaneControlPlaneAdapter(client *state.Client) *EventGatewayControlPlaneControlPlaneAdapter {
	return &EventGatewayControlPlaneControlPlaneAdapter{
		client: client,
	}
}

func (a *EventGatewayControlPlaneControlPlaneAdapter) MapCreateFields(_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any, create *components.CreateGatewayRequest,
) error {
	namespace := execCtx.Namespace
	protection := execCtx.Protection

	// Map required fields
	create.Name = common.ExtractResourceName(fields)

	// Map optional fields
	common.MapOptionalStringFieldToPtr(&create.Description, fields, "description")

	// Handle labels
	userLabels := labels.ExtractLabelsFromField(fields["labels"])
	labelsMap := labels.BuildCreateLabels(userLabels, namespace, protection)

	// Convert to SDK format
	if len(labelsMap) > 0 {
		create.Labels = labelsMap
	}

	return nil
}

func (a *EventGatewayControlPlaneControlPlaneAdapter) MapUpdateFields(
	_ context.Context, execCtx *ExecutionContext,
	fields map[string]any, update *components.UpdateGatewayRequest,
	currentLabels map[string]string,
) error {
	namespace := execCtx.Namespace
	protection := execCtx.Protection

	// Only include changed fields
	for field, value := range fields {
		switch field {
		case "name":
			if name, ok := value.(string); ok {
				update.Name = &name
			}
		case "description":
			if desc, ok := value.(string); ok {
				update.Description = &desc
			}
		}
	}

	// Handle labels
	desiredLabels := labels.ExtractLabelsFromField(fields["labels"])
	if desiredLabels != nil {
		plannerCurrentLabels := labels.ExtractLabelsFromField(fields[planner.FieldCurrentLabels])
		if plannerCurrentLabels != nil {
			currentLabels = plannerCurrentLabels
		}

		labelsMap := labels.BuildUpdateLabels(desiredLabels, currentLabels, namespace, protection)

		// Convert to SDK format
		update.Labels = labels.ConvertPointerMapsToStringMap(labelsMap)
		// OR: update.Labels = labelsMap
	} else if currentLabels != nil {
		// No label changes, preserve with updated protection
		labelsMap := labels.BuildUpdateLabels(currentLabels, currentLabels, namespace, protection)
		update.Labels = labels.ConvertPointerMapsToStringMap(labelsMap)
	}

	return nil
}

func (a *EventGatewayControlPlaneControlPlaneAdapter) Create(
	ctx context.Context, req components.CreateGatewayRequest,
	namespace string, _ *ExecutionContext,
) (string, error) {
	resp, err := a.client.CreateEventGatewayControlPlane(ctx, req, namespace)
	if err != nil {
		return "", err
	}
	return resp, nil
}

func (a *EventGatewayControlPlaneControlPlaneAdapter) Update(
	ctx context.Context, id string, req components.UpdateGatewayRequest,
	namespace string, _ *ExecutionContext,
) (string, error) {
	resp, err := a.client.UpdateEventGatewayControlPlane(ctx, id, req, namespace)
	if err != nil {
		return "", err
	}
	return resp, nil
}

// GetByID gets a event_gateway_control_plane by ID
func (a *EventGatewayControlPlaneControlPlaneAdapter) GetByID(
	ctx context.Context, id string, _ *ExecutionContext,
) (ResourceInfo, error) {
	eventGateway, err := a.client.GetEventGatewayControlPlaneByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if eventGateway == nil {
		return nil, nil
	}
	return &EventGatewayControlPlaneResourceInfo{eventGatewayControlPlane: eventGateway}, nil
}

// GetByName gets a event_gateway_control_plane by name
func (a *EventGatewayControlPlaneControlPlaneAdapter) GetByName(
	_ context.Context, _ string,
) (ResourceInfo, error) {
	// TODO - why and fix
	return nil, nil
}

func (a *EventGatewayControlPlaneControlPlaneAdapter) Delete(
	ctx context.Context, id string, _ *ExecutionContext,
) error {
	return a.client.DeleteEventGatewayControlPlane(ctx, id)
}

func (a *EventGatewayControlPlaneControlPlaneAdapter) ResourceType() string {
	return "event_gateway_control_plane"
}

func (a *EventGatewayControlPlaneControlPlaneAdapter) RequiredFields() []string {
	return []string{"name"}
}

func (a *EventGatewayControlPlaneControlPlaneAdapter) SupportsUpdate() bool {
	return true
}

// APIResourceInfo wraps an API to implement ResourceInfo
type EventGatewayControlPlaneResourceInfo struct {
	eventGatewayControlPlane *state.EventGatewayControlPlane
}

func (e *EventGatewayControlPlaneResourceInfo) GetID() string {
	return e.eventGatewayControlPlane.ID
}

func (e *EventGatewayControlPlaneResourceInfo) GetName() string {
	return e.eventGatewayControlPlane.Name
}

func (e *EventGatewayControlPlaneResourceInfo) GetLabels() map[string]string {
	// API.Labels is already map[string]string
	return e.eventGatewayControlPlane.Labels
}

func (e *EventGatewayControlPlaneResourceInfo) GetNormalizedLabels() map[string]string {
	return e.eventGatewayControlPlane.NormalizedLabels
}
