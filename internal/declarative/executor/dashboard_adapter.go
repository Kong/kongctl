package executor

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// DashboardAdapter implements ResourceOperations for dashboards.
type DashboardAdapter struct {
	client *state.Client
}

// NewDashboardAdapter creates a new dashboard adapter.
func NewDashboardAdapter(client *state.Client) *DashboardAdapter {
	return &DashboardAdapter{client: client}
}

func (a *DashboardAdapter) MapCreateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	create *kkComps.DashboardUpdateRequest,
) error {
	create.Name, _ = fields[planner.FieldName].(string)
	definition, err := dashboardDefinitionFromField(fields[planner.FieldDefinition])
	if err != nil {
		return err
	}
	create.Definition = definition

	userLabels := labels.ExtractLabelsFromField(fields[planner.FieldLabels])
	create.Labels = labels.BuildCreateLabels(userLabels, execCtx.Namespace, execCtx.Protection)

	return nil
}

func (a *DashboardAdapter) MapUpdateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	update *kkComps.DashboardUpdateRequest,
	currentLabels map[string]string,
) error {
	update.Name, _ = fields[planner.FieldName].(string)
	definition, err := dashboardDefinitionFromField(fields[planner.FieldDefinition])
	if err != nil {
		return err
	}
	update.Definition = definition

	desiredLabels := labels.ExtractLabelsFromField(fields[planner.FieldLabels])
	if desiredLabels != nil {
		plannerCurrentLabels := labels.ExtractLabelsFromField(fields[planner.FieldCurrentLabels])
		if plannerCurrentLabels != nil {
			currentLabels = plannerCurrentLabels
		}
		update.Labels = labels.BuildUpdateStringLabels(
			desiredLabels,
			currentLabels,
			execCtx.Namespace,
			execCtx.Protection,
		)
	} else if currentLabels != nil {
		update.Labels = labels.BuildUpdateStringLabels(currentLabels, currentLabels, execCtx.Namespace, execCtx.Protection)
	}
	if update.Labels == nil {
		update.Labels = make(map[string]string)
	}

	return nil
}

func (a *DashboardAdapter) Create(
	ctx context.Context,
	req kkComps.DashboardUpdateRequest,
	namespace string,
	_ *ExecutionContext,
) (string, error) {
	resp, err := a.client.CreateDashboard(ctx, req, namespace)
	if err != nil {
		return "", err
	}
	return getStringPtr(resp.ID), nil
}

func (a *DashboardAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.DashboardUpdateRequest,
	namespace string,
	_ *ExecutionContext,
) (string, error) {
	resp, err := a.client.UpdateDashboard(ctx, id, req, namespace)
	if err != nil {
		return "", err
	}
	return getStringPtr(resp.ID), nil
}

func (a *DashboardAdapter) Delete(ctx context.Context, id string, _ *ExecutionContext) error {
	return a.client.DeleteDashboard(ctx, id)
}

func (a *DashboardAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, nil
}

func (a *DashboardAdapter) GetByID(ctx context.Context, id string, _ *ExecutionContext) (ResourceInfo, error) {
	dashboard, err := a.client.GetDashboardByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if dashboard == nil {
		return nil, nil
	}
	return &dashboardResourceInfo{dashboard: dashboard}, nil
}

func dashboardDefinitionFromField(value any) (kkComps.Dashboard, error) {
	if definition, ok := value.(kkComps.Dashboard); ok {
		return definition, nil
	}
	if value == nil {
		return kkComps.Dashboard{}, fmt.Errorf("%s is required", planner.FieldDefinition)
	}

	data, err := json.Marshal(value)
	if err != nil {
		return kkComps.Dashboard{}, fmt.Errorf("failed to encode dashboard definition: %w", err)
	}

	var definition kkComps.Dashboard
	if err := json.Unmarshal(data, &definition); err != nil {
		return kkComps.Dashboard{}, fmt.Errorf("failed to decode dashboard definition: %w", err)
	}
	return definition, nil
}

func (a *DashboardAdapter) ResourceType() string {
	return planner.ResourceTypeDashboard
}

func (a *DashboardAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldDefinition}
}

func (a *DashboardAdapter) SupportsUpdate() bool {
	return true
}

type dashboardResourceInfo struct {
	dashboard *state.Dashboard
}

func (d *dashboardResourceInfo) GetID() string {
	return getStringPtr(d.dashboard.ID)
}

func (d *dashboardResourceInfo) GetName() string {
	return d.dashboard.Name
}

func (d *dashboardResourceInfo) GetLabels() map[string]string {
	return d.dashboard.Labels
}

func (d *dashboardResourceInfo) GetNormalizedLabels() map[string]string {
	return d.dashboard.NormalizedLabels
}

func getStringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
