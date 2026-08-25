package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayAuthStrategyAdapter implements ResourceOperations for AI Gateway Auth Strategies.
type AIGatewayAuthStrategyAdapter struct {
	client *state.Client
}

// NewAIGatewayAuthStrategyAdapter creates a new AI Gateway Auth Strategy adapter.
func NewAIGatewayAuthStrategyAdapter(client *state.Client) *AIGatewayAuthStrategyAdapter {
	return &AIGatewayAuthStrategyAdapter{client: client}
}

func (a *AIGatewayAuthStrategyAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayAuthStrategyRequest,
) error {
	payload, err := aiGatewayAuthStrategyPayloadFromFields(fields)
	if err != nil {
		return err
	}

	return mapAIGatewaySDKRequest("AI Gateway Auth Strategy create", payload, create)
}

func (a *AIGatewayAuthStrategyAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayAuthStrategyRequest,
	_ map[string]string,
) error {
	payload, err := aiGatewayAuthStrategyPayloadFromFields(fields)
	if err != nil {
		return err
	}

	return mapAIGatewaySDKRequest("AI Gateway Auth Strategy update", payload, update)
}

func (a *AIGatewayAuthStrategyAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayAuthStrategyRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewayAuthStrategy(ctx, gatewayID, req, namespace)
}

func (a *AIGatewayAuthStrategyAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayAuthStrategyRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayAuthStrategy(ctx, gatewayID, id, req, namespace)
}

func (a *AIGatewayAuthStrategyAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayAuthStrategy(ctx, gatewayID, id)
}

func (a *AIGatewayAuthStrategyAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway Auth Strategies")
}

func (a *AIGatewayAuthStrategyAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}

	provider, err := a.client.GetAIGatewayAuthStrategy(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, nil
	}
	return &aiGatewayAuthStrategyResourceInfo{provider: provider}, nil
}

func (a *AIGatewayAuthStrategyAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayAuthStrategy
}

func (a *AIGatewayAuthStrategyAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldType, planner.FieldDisplayName, planner.FieldConfig}
}

func (a *AIGatewayAuthStrategyAdapter) SupportsUpdate() bool {
	return true
}

func (a *AIGatewayAuthStrategyAdapter) getAIGatewayIDFromExecutionContext(
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
	return "", fmt.Errorf("AI Gateway ID required for AI Gateway Auth Strategy operations")
}

func aiGatewayAuthStrategyPayloadFromFields(fields map[string]any) (map[string]any, error) {
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

type aiGatewayAuthStrategyResourceInfo struct {
	provider *state.AIGatewayAuthStrategy
}

func (a *aiGatewayAuthStrategyResourceInfo) GetID() string {
	return a.provider.ID
}

func (a *aiGatewayAuthStrategyResourceInfo) GetName() string {
	return a.provider.Name
}

func (a *aiGatewayAuthStrategyResourceInfo) GetLabels() map[string]string {
	return a.provider.Labels
}

func (a *aiGatewayAuthStrategyResourceInfo) GetNormalizedLabels() map[string]string {
	return a.provider.NormalizedLabels
}
