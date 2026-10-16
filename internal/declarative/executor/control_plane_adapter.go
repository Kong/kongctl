package executor

import (
	"context"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/fieldcontracts"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// ControlPlaneAdapter implements ResourceOperations for control planes
type ControlPlaneAdapter struct {
	client *state.Client
}

// NewControlPlaneAdapter creates a new control plane adapter
func NewControlPlaneAdapter(client *state.Client) *ControlPlaneAdapter {
	return &ControlPlaneAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateControlPlaneRequest
func (a *ControlPlaneAdapter) MapCreateFields(_ context.Context, execCtx *ExecutionContext,
	fields map[string]any, create *kkComps.CreateControlPlaneRequest,
) error {
	create.Name = common.ExtractResourceName(fields)
	if err := fieldcontracts.MapControlPlaneCreateFields(fields, create); err != nil {
		return err
	}

	userLabels := labels.ExtractLabelsFromField(fields[planner.FieldLabels])
	create.Labels = labels.BuildCreateLabels(userLabels, execCtx.Namespace, execCtx.Protection)

	return nil
}

// MapUpdateFields maps planner fields to UpdateControlPlaneRequest
func (a *ControlPlaneAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext,
	fields map[string]any, update *kkComps.UpdateControlPlaneRequest, _ map[string]string,
) error {
	for field, value := range fields {
		switch field {
		case planner.FieldName:
			if name, ok := value.(string); ok {
				update.Name = &name
			}
		default:
			if err := fieldcontracts.MapControlPlaneUpdateField(field, value, update); err != nil {
				return err
			}
		}
	}

	return nil
}

func (a *ControlPlaneAdapter) MapUpdateLabels(
	execCtx *ExecutionContext,
	update *kkComps.UpdateControlPlaneRequest,
	desiredLabels map[string]string,
	currentLabels map[string]string,
) {
	update.Labels = labels.BuildUpdateStringLabels(
		desiredLabels,
		currentLabels,
		execCtx.Namespace,
		execCtx.Protection,
	)
}

// Create issues a create call via the state client
func (a *ControlPlaneAdapter) Create(ctx context.Context, req kkComps.CreateControlPlaneRequest,
	namespace string, _ *ExecutionContext,
) (string, error) {
	resp, err := a.client.CreateControlPlane(ctx, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Update issues an update call via the state client
func (a *ControlPlaneAdapter) Update(ctx context.Context, id string, req kkComps.UpdateControlPlaneRequest,
	namespace string, _ *ExecutionContext,
) (string, error) {
	resp, err := a.client.UpdateControlPlane(ctx, id, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Delete removes a control plane
func (a *ControlPlaneAdapter) Delete(ctx context.Context, id string, _ *ExecutionContext) error {
	return a.client.DeleteControlPlane(ctx, id)
}

// GetByName resolves a control plane by name
func (a *ControlPlaneAdapter) GetByName(ctx context.Context, name string) (ResourceInfo, error) {
	cp, err := a.client.GetControlPlaneByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if cp == nil {
		return nil, nil
	}
	return &ControlPlaneResourceInfo{controlPlane: cp}, nil
}

// GetByID resolves a control plane by ID
func (a *ControlPlaneAdapter) GetByID(ctx context.Context, id string, _ *ExecutionContext) (ResourceInfo, error) {
	cp, err := a.client.GetControlPlaneByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cp == nil {
		return nil, nil
	}
	return &ControlPlaneResourceInfo{controlPlane: cp}, nil
}

// ResourceType returns the adapter resource type
func (a *ControlPlaneAdapter) ResourceType() string {
	return planner.ResourceTypeControlPlane
}

// RequiredFields lists required fields for create
func (a *ControlPlaneAdapter) RequiredFields() []string {
	return []string{planner.FieldName}
}

// SupportsUpdate indicates control planes support updates
func (a *ControlPlaneAdapter) SupportsUpdate() bool {
	return true
}

// ControlPlaneResourceInfo implements ResourceInfo for control planes
type ControlPlaneResourceInfo struct {
	controlPlane *state.ControlPlane
}

func (c *ControlPlaneResourceInfo) GetID() string {
	return c.controlPlane.ID
}

func (c *ControlPlaneResourceInfo) GetName() string {
	return c.controlPlane.Name
}

func (c *ControlPlaneResourceInfo) GetLabels() map[string]string {
	return c.controlPlane.Labels
}

func (c *ControlPlaneResourceInfo) GetNormalizedLabels() map[string]string {
	return c.controlPlane.NormalizedLabels
}
