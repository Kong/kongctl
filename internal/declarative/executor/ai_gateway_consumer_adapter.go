package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayConsumerAdapter implements ResourceOperations for AI Gateway Consumers.
type AIGatewayConsumerAdapter struct {
	client *state.Client
}

// NewAIGatewayConsumerAdapter creates a new AI Gateway Consumer adapter.
func NewAIGatewayConsumerAdapter(client *state.Client) *AIGatewayConsumerAdapter {
	return &AIGatewayConsumerAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateAIGatewayConsumerRequest.
func (a *AIGatewayConsumerAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayConsumerRequest,
) error {
	if err := mapAIGatewaySDKRequest("AI Gateway Consumer create", fields, create); err != nil {
		return err
	}
	if create.Name == "" || create.DisplayName == "" || create.Type == "" {
		return fmt.Errorf("name, display_name, and type are required")
	}
	return nil
}

// MapUpdateFields maps planner fields to UpdateAIGatewayConsumerRequest.
func (a *AIGatewayConsumerAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayConsumerRequest,
	_ map[string]string,
) error {
	return mapAIGatewaySDKRequest("AI Gateway Consumer update", fields, update)
}

// Create creates an AI Gateway Consumer.
func (a *AIGatewayConsumerAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayConsumerRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewayConsumer(ctx, gatewayID, req, namespace)
}

// Update updates an AI Gateway Consumer.
func (a *AIGatewayConsumerAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayConsumerRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayConsumer(ctx, gatewayID, id, req, namespace)
}

// Delete deletes an AI Gateway Consumer.
func (a *AIGatewayConsumerAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayConsumer(ctx, gatewayID, id)
}

// GetByID gets an AI Gateway Consumer by ID.
func (a *AIGatewayConsumerAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}
	consumer, err := a.client.GetAIGatewayConsumer(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if consumer == nil {
		return nil, nil
	}
	return &aiGatewayConsumerResourceInfo{consumer: consumer}, nil
}

// GetByName is not supported without a parent gateway context.
func (a *AIGatewayConsumerAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway Consumers")
}

// ResourceType returns the resource type.
func (a *AIGatewayConsumerAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayConsumer
}

// RequiredFields returns required fields for create.
func (a *AIGatewayConsumerAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldDisplayName, planner.FieldType}
}

// SupportsUpdate indicates update support.
func (a *AIGatewayConsumerAdapter) SupportsUpdate() bool {
	return true
}

func (a *AIGatewayConsumerAdapter) getAIGatewayIDFromExecutionContext(
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

	return "", fmt.Errorf("AI Gateway ID required for Consumer operations")
}

type aiGatewayConsumerResourceInfo struct {
	consumer *state.AIGatewayConsumer
}

func (a *aiGatewayConsumerResourceInfo) GetID() string {
	return resources.AIGatewayConsumerID(a.consumer.AIGatewayConsumer)
}

func (a *aiGatewayConsumerResourceInfo) GetName() string {
	return resources.AIGatewayConsumerName(a.consumer.AIGatewayConsumer)
}

func (a *aiGatewayConsumerResourceInfo) GetLabels() map[string]string {
	return resources.AIGatewayConsumerLabels(a.consumer.AIGatewayConsumer)
}

func (a *aiGatewayConsumerResourceInfo) GetNormalizedLabels() map[string]string {
	return a.consumer.NormalizedLabels
}
