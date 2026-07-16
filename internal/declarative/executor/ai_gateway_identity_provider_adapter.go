package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayIdentityProviderAdapter implements ResourceOperations for AI Gateway Identity Providers.
type AIGatewayIdentityProviderAdapter struct {
	client *state.Client
}

// NewAIGatewayIdentityProviderAdapter creates a new AI Gateway Identity Provider adapter.
func NewAIGatewayIdentityProviderAdapter(client *state.Client) *AIGatewayIdentityProviderAdapter {
	return &AIGatewayIdentityProviderAdapter{client: client}
}

func (a *AIGatewayIdentityProviderAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayIdentityProviderRequest,
) error {
	payload, err := aiGatewayIdentityProviderPayloadFromFields(fields)
	if err != nil {
		return err
	}

	return mapAIGatewaySDKRequest("AI Gateway Identity Provider create", payload, create)
}

func (a *AIGatewayIdentityProviderAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayIdentityProviderRequest,
	_ map[string]string,
) error {
	payload, err := aiGatewayIdentityProviderPayloadFromFields(fields)
	if err != nil {
		return err
	}

	return mapAIGatewaySDKRequest("AI Gateway Identity Provider update", payload, update)
}

func (a *AIGatewayIdentityProviderAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayIdentityProviderRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewayIdentityProvider(ctx, gatewayID, req, namespace)
}

func (a *AIGatewayIdentityProviderAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayIdentityProviderRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayIdentityProvider(ctx, gatewayID, id, req, namespace)
}

func (a *AIGatewayIdentityProviderAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayIdentityProvider(ctx, gatewayID, id)
}

func (a *AIGatewayIdentityProviderAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway Identity Providers")
}

func (a *AIGatewayIdentityProviderAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}

	provider, err := a.client.GetAIGatewayIdentityProvider(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, nil
	}
	return &aiGatewayIdentityProviderResourceInfo{provider: provider}, nil
}

func (a *AIGatewayIdentityProviderAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayIdentityProvider
}

func (a *AIGatewayIdentityProviderAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldType, planner.FieldDisplayName, planner.FieldConfig}
}

func (a *AIGatewayIdentityProviderAdapter) SupportsUpdate() bool {
	return true
}

func (a *AIGatewayIdentityProviderAdapter) getAIGatewayIDFromExecutionContext(
	execCtx *ExecutionContext,
) (string, error) {
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
	return "", fmt.Errorf("AI Gateway ID required for AI Gateway Identity Provider operations")
}

func aiGatewayIdentityProviderPayloadFromFields(fields map[string]any) (map[string]any, error) {
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

type aiGatewayIdentityProviderResourceInfo struct {
	provider *state.AIGatewayIdentityProvider
}

func (a *aiGatewayIdentityProviderResourceInfo) GetID() string {
	return a.provider.ID
}

func (a *aiGatewayIdentityProviderResourceInfo) GetName() string {
	return a.provider.Name
}

func (a *aiGatewayIdentityProviderResourceInfo) GetLabels() map[string]string {
	return a.provider.Labels
}

func (a *aiGatewayIdentityProviderResourceInfo) GetNormalizedLabels() map[string]string {
	return a.provider.NormalizedLabels
}
