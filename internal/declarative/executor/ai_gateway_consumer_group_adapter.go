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

// AIGatewayConsumerGroupAdapter implements ResourceOperations for AI Gateway Consumer Groups.
type AIGatewayConsumerGroupAdapter struct {
	client *state.Client
}

// NewAIGatewayConsumerGroupAdapter creates a new AI Gateway Consumer Group adapter.
func NewAIGatewayConsumerGroupAdapter(client *state.Client) *AIGatewayConsumerGroupAdapter {
	return &AIGatewayConsumerGroupAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateAIGatewayConsumerGroupRequest.
func (a *AIGatewayConsumerGroupAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayConsumerGroupRequest,
) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("failed to encode AI Gateway Consumer Group create fields: %w", err)
	}
	if err := json.Unmarshal(data, create); err != nil {
		return fmt.Errorf("failed to decode AI Gateway Consumer Group create fields: %w", err)
	}
	if create.Name == "" || create.DisplayName == "" {
		return fmt.Errorf("name and display_name are required")
	}
	return nil
}

// MapUpdateFields maps planner fields to UpdateAIGatewayConsumerGroupRequest.
func (a *AIGatewayConsumerGroupAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayConsumerGroupRequest,
	_ map[string]string,
) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("failed to encode AI Gateway Consumer Group update fields: %w", err)
	}
	if err := json.Unmarshal(data, update); err != nil {
		return fmt.Errorf("failed to decode AI Gateway Consumer Group update fields: %w", err)
	}
	return nil
}

// Create creates an AI Gateway Consumer Group.
func (a *AIGatewayConsumerGroupAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayConsumerGroupRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewayConsumerGroup(ctx, gatewayID, req, namespace)
}

// Update updates an AI Gateway Consumer Group.
func (a *AIGatewayConsumerGroupAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayConsumerGroupRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayConsumerGroup(ctx, gatewayID, id, req, namespace)
}

// Delete deletes an AI Gateway Consumer Group.
func (a *AIGatewayConsumerGroupAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayConsumerGroup(ctx, gatewayID, id)
}

// GetByID gets an AI Gateway Consumer Group by ID.
func (a *AIGatewayConsumerGroupAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}
	group, err := a.client.GetAIGatewayConsumerGroup(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, nil
	}
	return &aiGatewayConsumerGroupResourceInfo{group: group}, nil
}

// GetByName is not supported without a parent gateway context.
func (a *AIGatewayConsumerGroupAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway Consumer Groups")
}

// ResourceType returns the resource type.
func (a *AIGatewayConsumerGroupAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayConsumerGroup
}

// RequiredFields returns required fields for create.
func (a *AIGatewayConsumerGroupAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldDisplayName}
}

// SupportsUpdate indicates update support.
func (a *AIGatewayConsumerGroupAdapter) SupportsUpdate() bool {
	return true
}

func (a *AIGatewayConsumerGroupAdapter) getAIGatewayIDFromExecutionContext(
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

	return "", fmt.Errorf("AI Gateway ID required for Consumer Group operations")
}

type aiGatewayConsumerGroupResourceInfo struct {
	group *state.AIGatewayConsumerGroup
}

func (a *aiGatewayConsumerGroupResourceInfo) GetID() string {
	return resources.AIGatewayConsumerGroupID(a.group.AIGatewayConsumerGroup)
}

func (a *aiGatewayConsumerGroupResourceInfo) GetName() string {
	return resources.AIGatewayConsumerGroupName(a.group.AIGatewayConsumerGroup)
}

func (a *aiGatewayConsumerGroupResourceInfo) GetLabels() map[string]string {
	return resources.AIGatewayConsumerGroupLabels(a.group.AIGatewayConsumerGroup)
}

func (a *aiGatewayConsumerGroupResourceInfo) GetNormalizedLabels() map[string]string {
	return a.group.NormalizedLabels
}
