package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayProviderAdapter implements ResourceOperations for AI Gateway Model Providers.
type AIGatewayProviderAdapter struct {
	client *state.Client
}

// NewAIGatewayProviderAdapter creates a new AI Gateway Model Provider adapter.
func NewAIGatewayProviderAdapter(client *state.Client) *AIGatewayProviderAdapter {
	return &AIGatewayProviderAdapter{client: client}
}

func (a *AIGatewayProviderAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayModelProviderRequest,
) error {
	payload, err := aiGatewayProviderPayloadFromFields(fields)
	if err != nil {
		return err
	}

	return mapAIGatewaySDKRequest("AI Gateway Model Provider create", payload, create)
}

func (a *AIGatewayProviderAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayModelProviderRequest,
	_ map[string]string,
) error {
	payload, err := aiGatewayProviderPayloadFromFields(fields)
	if err != nil {
		return err
	}

	return mapAIGatewaySDKRequest("AI Gateway Model Provider update", payload, update)
}

func (a *AIGatewayProviderAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayModelProviderRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewayProvider(ctx, gatewayID, req, namespace)
}

func (a *AIGatewayProviderAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayModelProviderRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayProvider(ctx, gatewayID, id, req, namespace)
}

func (a *AIGatewayProviderAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayProvider(ctx, gatewayID, id)
}

func (a *AIGatewayProviderAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway Model Providers")
}

func (a *AIGatewayProviderAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}

	provider, err := a.client.GetAIGatewayProvider(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, nil
	}
	return &aiGatewayProviderResourceInfo{provider: provider}, nil
}

func (a *AIGatewayProviderAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayProvider
}

func (a *AIGatewayProviderAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldType, planner.FieldDisplayName, planner.FieldConfig}
}

func (a *AIGatewayProviderAdapter) SupportsUpdate() bool {
	return true
}

func (a *AIGatewayProviderAdapter) getAIGatewayIDFromExecutionContext(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context required")
	}

	change := *execCtx.PlannedChange
	if gatewayRef, ok := change.References[planner.FieldAIGatewayID]; ok && !unresolvedReferenceID(gatewayRef.ID) {
		return gatewayRef.ID, nil
	}
	if change.Parent != nil && !unresolvedReferenceID(change.Parent.ID) {
		return change.Parent.ID, nil
	}
	return "", fmt.Errorf("AI Gateway ID required for AI Gateway Model Provider operations")
}

func aiGatewayProviderPayloadFromFields(fields map[string]any) (map[string]any, error) {
	name, ok := fields[planner.FieldName].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("name is required")
	}
	providerType, ok := fields[planner.FieldType].(string)
	if !ok || providerType == "" {
		return nil, fmt.Errorf("type is required")
	}
	displayName, ok := fields[planner.FieldDisplayName].(string)
	if !ok || displayName == "" {
		return nil, fmt.Errorf("display_name is required")
	}
	config, ok := fields[planner.FieldConfig]
	if !ok || config == nil {
		return nil, fmt.Errorf("config is required")
	}

	payload := map[string]any{
		planner.FieldName:        name,
		planner.FieldType:        providerType,
		planner.FieldDisplayName: displayName,
		planner.FieldConfig:      config,
	}
	if labels, ok := fields[planner.FieldLabels]; ok {
		payload[planner.FieldLabels] = labels
	}
	if managedBy, ok := fields[planner.FieldManagedBy]; ok {
		payload[planner.FieldManagedBy] = managedBy
	}
	return payload, nil
}

type aiGatewayProviderResourceInfo struct {
	provider *state.AIGatewayProvider
}

func (a *aiGatewayProviderResourceInfo) GetID() string {
	return a.provider.ID
}

func (a *aiGatewayProviderResourceInfo) GetName() string {
	return a.provider.Name
}

func (a *aiGatewayProviderResourceInfo) GetLabels() map[string]string {
	return a.provider.Labels
}

func (a *aiGatewayProviderResourceInfo) GetNormalizedLabels() map[string]string {
	return a.provider.NormalizedLabels
}
