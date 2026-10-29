package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type FeatureAdapter struct{ client *state.Client }

func NewFeatureAdapter(client *state.Client) *FeatureAdapter { return &FeatureAdapter{client: client} }

func decodeFeatureFields(fields map[string]any, target any) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("encode feature fields: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode feature fields: %w", err)
	}
	return nil
}

func (a *FeatureAdapter) MapCreateFields(
	_ context.Context,
	ctx *ExecutionContext,
	fields map[string]any,
	req *kkComps.CreateFeatureRequest,
) error {
	var decoded helpers.FeatureCreateRequest
	if err := decodeFeatureFields(fields, &decoded); err != nil {
		return err
	}
	var err error
	*req, err = decoded.SDKRequest()
	if err != nil {
		return err
	}
	req.Labels = labels.BuildCreateLabels(req.Labels, ctx.Namespace, ctx.Protection)
	if req.Meter != nil && !helpers.IsMeterULID(req.Meter.ID) && !tags.IsRefPlaceholder(req.Meter.ID) {
		return fmt.Errorf("feature meter.id must be an uppercase ULID or a resolved meter reference")
	}
	return helpers.ValidateFeatureFields(*req)
}

func (a *FeatureAdapter) MapUpdateFields(
	_ context.Context,
	ctx *ExecutionContext,
	fields map[string]any,
	req *helpers.FeatureUpdateRequest,
	_ map[string]string,
) error {
	if ctx != nil && featureProtectionChanged(ctx.PlannedChange) {
		return fmt.Errorf("feature protection cannot change: PATCH accepts only unit_cost")
	}
	if err := decodeFeatureFields(fields, req); err != nil {
		return err
	}
	return req.Validate()
}

func featureProtectionChanged(change *planner.PlannedChange) bool {
	if change == nil {
		return false
	}
	switch value := change.Protection.(type) {
	case planner.ProtectionChange:
		return value.Old != value.New
	case map[string]any:
		return value["old"] != value["new"]
	}
	return false
}

func (a *FeatureAdapter) Create(
	ctx context.Context,
	req kkComps.CreateFeatureRequest,
	namespace string,
	_ *ExecutionContext,
) (string, error) {
	resp, err := a.client.CreateFeature(ctx, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (a *FeatureAdapter) Update(
	ctx context.Context,
	id string,
	req helpers.FeatureUpdateRequest,
	namespace string,
	_ *ExecutionContext,
) (string, error) {
	resp, err := a.client.UpdateFeature(ctx, id, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (a *FeatureAdapter) Delete(ctx context.Context, id string, _ *ExecutionContext) error {
	return a.client.DeleteFeature(ctx, id)
}

func (a *FeatureAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, nil
}

func (a *FeatureAdapter) GetByID(ctx context.Context, id string, execCtx *ExecutionContext) (ResourceInfo, error) {
	if !helpers.IsMeterULID(id) {
		return nil, fmt.Errorf("feature resource_id must be a valid ULID")
	}
	m, err := a.client.GetFeatureByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if execCtx != nil && (!labels.IsManagedResource(m.Labels) || m.Labels[labels.NamespaceKey] != execCtx.Namespace) {
		return nil, fmt.Errorf(
			"feature %q is no longer owned by namespace %q; regenerate the plan",
			m.Key,
			execCtx.Namespace,
		)
	}
	if execCtx != nil && labels.IsProtectedResource(m.Labels) {
		return nil, fmt.Errorf(
			"feature %q is protected; feature protection cannot be changed through the current API",
			m.Key,
		)
	}
	return &featureResourceInfo{m}, nil
}
func (a *FeatureAdapter) ResourceType() string { return planner.ResourceTypeFeature }
func (a *FeatureAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldKey}
}
func (a *FeatureAdapter) SupportsUpdate() bool { return true }

type featureResourceInfo struct{ feature *state.Feature }

func (m *featureResourceInfo) GetID() string                { return m.feature.ID }
func (m *featureResourceInfo) GetName() string              { return m.feature.Key }
func (m *featureResourceInfo) GetLabels() map[string]string { return m.feature.Labels }
func (m *featureResourceInfo) GetNormalizedLabels() map[string]string {
	return m.feature.NormalizedLabels
}

func (a *FeatureAdapter) GetForUpdate(ctx context.Context, change planner.PlannedChange) (ResourceInfo, error) {
	return a.GetByID(ctx, change.ResourceID, NewExecutionContext(&change))
}
