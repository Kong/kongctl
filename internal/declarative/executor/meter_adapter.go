package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type MeterAdapter struct{ client *state.Client }

func NewMeterAdapter(client *state.Client) *MeterAdapter { return &MeterAdapter{client: client} }
func decodeMeterFields(fields map[string]any, target any) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("encode meter fields: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode meter fields: %w", err)
	}
	return nil
}

func (a *MeterAdapter) MapCreateFields(
	_ context.Context,
	ctx *ExecutionContext,
	fields map[string]any,
	req *kkComps.CreateMeterRequest,
) error {
	type request kkComps.CreateMeterRequest
	var decoded request
	if err := decodeMeterFields(fields, &decoded); err != nil {
		return err
	}
	*req = kkComps.CreateMeterRequest(decoded)
	req.Labels = labels.BuildCreateLabels(req.Labels, ctx.Namespace, ctx.Protection)
	return helpers.ValidateMeterCreate(*req)
}

func (a *MeterAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	req *kkComps.UpdateMeterRequest,
	_ map[string]string,
) error {
	fields = maps.Clone(fields)
	delete(fields, planner.FieldCurrentLabels)
	if err := decodeMeterFields(fields, req); err != nil {
		return err
	}
	return helpers.ValidateMeterUpdate(*req)
}

func (a *MeterAdapter) MapUpdateLabels(
	ctx *ExecutionContext,
	req *kkComps.UpdateMeterRequest,
	desired, current map[string]string,
) {
	req.Labels = labels.BuildUpdateStringLabels(desired, current, ctx.Namespace, ctx.Protection)
}

func (a *MeterAdapter) Create(
	ctx context.Context,
	req kkComps.CreateMeterRequest,
	namespace string,
	_ *ExecutionContext,
) (string, error) {
	resp, err := a.client.CreateMeter(ctx, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (a *MeterAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateMeterRequest,
	namespace string,
	_ *ExecutionContext,
) (string, error) {
	resp, err := a.client.UpdateMeter(ctx, id, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (a *MeterAdapter) Delete(ctx context.Context, id string, _ *ExecutionContext) error {
	return a.client.DeleteMeter(ctx, id)
}

func (a *MeterAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) { return nil, nil }

func (a *MeterAdapter) GetByID(ctx context.Context, id string, execCtx *ExecutionContext) (ResourceInfo, error) {
	if !helpers.IsMeterULID(id) {
		return nil, fmt.Errorf("meter resource_id must be a valid ULID")
	}
	m, err := a.client.GetMeterByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if execCtx != nil && (!labels.IsManagedResource(m.Labels) || m.Labels[labels.NamespaceKey] != execCtx.Namespace) {
		return nil, fmt.Errorf(
			"meter %q is no longer owned by namespace %q; regenerate the plan",
			m.Key,
			execCtx.Namespace,
		)
	}
	if execCtx != nil && labels.IsProtectedResource(m.Labels) && !meterUnprotectOnly(execCtx.PlannedChange) {
		return nil, fmt.Errorf(
			"meter %q is protected; unprotect it in a separate apply before changing or deleting it",
			m.Key,
		)
	}
	return &meterResourceInfo{m}, nil
}
func (a *MeterAdapter) ResourceType() string { return planner.ResourceTypeMeter }
func (a *MeterAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldKey, planner.FieldAggregation, planner.FieldEventType}
}
func (a *MeterAdapter) SupportsUpdate() bool { return true }

type meterResourceInfo struct{ meter *state.Meter }

func (m *meterResourceInfo) GetID() string                          { return m.meter.ID }
func (m *meterResourceInfo) GetName() string                        { return m.meter.Key }
func (m *meterResourceInfo) GetLabels() map[string]string           { return m.meter.Labels }
func (m *meterResourceInfo) GetNormalizedLabels() map[string]string { return m.meter.NormalizedLabels }

func (a *MeterAdapter) GetForUpdate(ctx context.Context, change planner.PlannedChange) (ResourceInfo, error) {
	return a.GetByID(ctx, change.ResourceID, NewExecutionContext(&change))
}

func meterUnprotectOnly(change *planner.PlannedChange) bool {
	if change == nil || change.Action != planner.ActionUpdate {
		return false
	}
	var old, next bool
	switch protection := change.Protection.(type) {
	case planner.ProtectionChange:
		old, next = protection.Old, protection.New
	case map[string]any:
		old, _ = protection["old"].(bool)
		next, _ = protection["new"].(bool)
	default:
		return false
	}
	if !old || next {
		return false
	}
	for field := range change.Fields {
		if field != planner.FieldCurrentLabels {
			return false
		}
	}
	return true
}
