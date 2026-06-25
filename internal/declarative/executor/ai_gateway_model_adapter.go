package executor

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayModelAdapter implements ResourceOperations for AI Gateway models.
type AIGatewayModelAdapter struct {
	client *state.Client
}

// NewAIGatewayModelAdapter creates a new AI Gateway model adapter.
func NewAIGatewayModelAdapter(client *state.Client) *AIGatewayModelAdapter {
	return &AIGatewayModelAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateAIGatewayModelRequest.
func (a *AIGatewayModelAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayModelRequest,
) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("failed to encode AI Gateway model create fields: %w", err)
	}
	if err := json.Unmarshal(data, create); err != nil {
		return fmt.Errorf("failed to decode AI Gateway model create fields: %w", err)
	}
	if create.AIGatewayModelAPI == nil && create.AIGatewayModelModel == nil {
		return fmt.Errorf("type must be either api or model")
	}
	return nil
}

// MapUpdateFields maps planner fields to UpdateAIGatewayModelRequest.
func (a *AIGatewayModelAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayModelRequest,
	_ map[string]string,
) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("failed to encode AI Gateway model update fields: %w", err)
	}
	if err := json.Unmarshal(data, update); err != nil {
		return fmt.Errorf("failed to decode AI Gateway model update fields: %w", err)
	}
	if update.AIGatewayModelAPI == nil && update.AIGatewayModelModel == nil {
		return fmt.Errorf("type must be either api or model")
	}
	return nil
}

// Create creates an AI Gateway model.
func (a *AIGatewayModelAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayModelRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewayModel(ctx, gatewayID, req, namespace)
}

// Update updates an AI Gateway model.
func (a *AIGatewayModelAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayModelRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayModel(ctx, gatewayID, id, req, namespace)
}

// Delete deletes an AI Gateway model.
func (a *AIGatewayModelAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayModel(ctx, gatewayID, id)
}

// GetByID gets an AI Gateway model by ID.
func (a *AIGatewayModelAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}
	model, err := a.client.GetAIGatewayModel(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, nil
	}
	return &aiGatewayModelResourceInfo{model: model}, nil
}

// GetByName is not supported without a parent gateway context.
func (a *AIGatewayModelAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway models")
}

// ResourceType returns the resource type.
func (a *AIGatewayModelAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayModel
}

// RequiredFields returns required fields for create.
func (a *AIGatewayModelAdapter) RequiredFields() []string {
	return []string{planner.FieldType, planner.FieldName, planner.FieldDisplayName}
}

// SupportsUpdate indicates update support.
func (a *AIGatewayModelAdapter) SupportsUpdate() bool {
	return true
}

func (a *AIGatewayModelAdapter) getAIGatewayIDFromExecutionContext(execCtx *ExecutionContext) (string, error) {
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

	return "", fmt.Errorf("AI Gateway ID required for model operations")
}

type aiGatewayModelResourceInfo struct {
	model *state.AIGatewayModel
}

func (a *aiGatewayModelResourceInfo) GetID() string {
	return resources.AIGatewayModelID(a.model.AIGatewayModel)
}

func (a *aiGatewayModelResourceInfo) GetName() string {
	return resources.AIGatewayModelName(a.model.AIGatewayModel)
}

func (a *aiGatewayModelResourceInfo) GetLabels() map[string]string {
	return resources.AIGatewayModelLabels(a.model.AIGatewayModel)
}

func (a *aiGatewayModelResourceInfo) GetNormalizedLabels() map[string]string {
	return a.model.NormalizedLabels
}
