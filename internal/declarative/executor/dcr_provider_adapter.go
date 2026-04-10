package executor

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

type DCRProviderAdapter struct {
	client *state.Client
}

func NewDCRProviderAdapter(client *state.Client) *DCRProviderAdapter {
	return &DCRProviderAdapter{client: client}
}

func (a *DCRProviderAdapter) MapCreateFields(
	_ context.Context, execCtx *ExecutionContext, fields map[string]any,
	create *kkComps.CreateDcrProviderRequest,
) error {
	payload := map[string]any{
		planner.FieldName:                    common.ExtractResourceName(fields),
		planner.FieldDCRProviderProviderType: fields[planner.FieldDCRProviderProviderType],
		planner.FieldDCRProviderIssuer:       fields[planner.FieldDCRProviderIssuer],
		planner.FieldDCRProviderConfig:       fields[planner.FieldDCRProviderConfig],
	}
	if displayName, ok := fields[planner.FieldDisplayName].(string); ok && displayName != "" {
		payload[planner.FieldDisplayName] = displayName
	}

	userLabels := labels.ExtractLabelsFromField(fields[planner.FieldLabels])
	managedLabels := labels.BuildCreateLabels(userLabels, execCtx.Namespace, execCtx.Protection)
	if managedLabels != nil {
		payload[planner.FieldLabels] = managedLabels
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal DCR provider create payload: %w", err)
	}
	if err := json.Unmarshal(payloadBytes, create); err != nil {
		return fmt.Errorf("failed to unmarshal DCR provider create payload: %w", err)
	}
	return nil
}

func (a *DCRProviderAdapter) MapUpdateFields(
	_ context.Context, execCtx *ExecutionContext, fields map[string]any,
	update *kkComps.UpdateDcrProviderRequest, currentLabels map[string]string,
) error {
	payload := map[string]any{}
	if displayName, ok := fields[planner.FieldDisplayName].(string); ok {
		payload[planner.FieldDisplayName] = displayName
	}
	if issuer, ok := fields[planner.FieldDCRProviderIssuer].(string); ok {
		payload[planner.FieldDCRProviderIssuer] = issuer
	}
	if dcrConfig, ok := fields[planner.FieldDCRProviderConfig]; ok {
		payload[planner.FieldDCRProviderConfig] = dcrConfig
	}

	desiredLabels := labels.ExtractLabelsFromField(fields[planner.FieldLabels])
	if desiredLabels != nil {
		plannerCurrentLabels := labels.ExtractLabelsFromField(fields[planner.FieldCurrentLabels])
		if plannerCurrentLabels != nil {
			currentLabels = plannerCurrentLabels
		}
		payload[planner.FieldLabels] = labels.BuildUpdateLabels(
			desiredLabels,
			currentLabels,
			execCtx.Namespace,
			execCtx.Protection,
		)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal DCR provider update payload: %w", err)
	}
	if err := json.Unmarshal(payloadBytes, update); err != nil {
		return fmt.Errorf("failed to unmarshal DCR provider update payload: %w", err)
	}
	return nil
}

func (a *DCRProviderAdapter) Create(ctx context.Context, req kkComps.CreateDcrProviderRequest,
	namespace string, _ *ExecutionContext,
) (string, error) {
	provider, err := a.client.CreateDCRProvider(ctx, req, namespace)
	if err != nil {
		return "", err
	}
	return provider.ID, nil
}

func (a *DCRProviderAdapter) Update(ctx context.Context, id string, req kkComps.UpdateDcrProviderRequest,
	namespace string, _ *ExecutionContext,
) (string, error) {
	if err := a.client.UpdateDCRProvider(ctx, id, req, namespace); err != nil {
		return "", err
	}
	return id, nil
}

func (a *DCRProviderAdapter) Delete(ctx context.Context, id string, _ *ExecutionContext) error {
	return a.client.DeleteDCRProvider(ctx, id)
}

func (a *DCRProviderAdapter) GetByName(ctx context.Context, name string) (ResourceInfo, error) {
	provider, err := a.client.GetDCRProviderByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, nil
	}
	return &DCRProviderResourceInfo{provider: provider}, nil
}

func (a *DCRProviderAdapter) GetByID(ctx context.Context, id string, _ *ExecutionContext) (ResourceInfo, error) {
	provider, err := a.client.GetDCRProviderByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, nil
	}
	return &DCRProviderResourceInfo{provider: provider}, nil
}

func (a *DCRProviderAdapter) ResourceType() string {
	return "dcr_provider"
}

func (a *DCRProviderAdapter) RequiredFields() []string {
	return []string{planner.FieldDCRProviderProviderType, planner.FieldDCRProviderIssuer, planner.FieldDCRProviderConfig}
}

func (a *DCRProviderAdapter) SupportsUpdate() bool {
	return true
}

type DCRProviderResourceInfo struct {
	provider *state.DCRProvider
}

func (d *DCRProviderResourceInfo) GetID() string {
	return d.provider.ID
}

func (d *DCRProviderResourceInfo) GetName() string {
	return d.provider.Name
}

func (d *DCRProviderResourceInfo) GetLabels() map[string]string {
	return d.provider.NormalizedLabels
}

func (d *DCRProviderResourceInfo) GetNormalizedLabels() map[string]string {
	return d.provider.NormalizedLabels
}
