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

// AIGatewayAgentAdapter implements ResourceOperations for AI Gateway Agents.
type AIGatewayAgentAdapter struct {
	client *state.Client
}

// NewAIGatewayAgentAdapter creates a new AI Gateway Agent adapter.
func NewAIGatewayAgentAdapter(client *state.Client) *AIGatewayAgentAdapter {
	return &AIGatewayAgentAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateAIGatewayAgentRequest.
func (a *AIGatewayAgentAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayAgentRequest,
) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("failed to encode AI Gateway Agent create fields: %w", err)
	}
	if err := json.Unmarshal(data, create); err != nil {
		return fmt.Errorf("failed to decode AI Gateway Agent create fields: %w", err)
	}
	if create.Name == "" || create.DisplayName == "" || create.Type == "" || create.Config.URL == "" {
		return fmt.Errorf("name, display_name, type, and config.url are required")
	}
	return nil
}

// MapUpdateFields maps planner fields to UpdateAIGatewayAgentRequest.
func (a *AIGatewayAgentAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayAgentRequest,
	_ map[string]string,
) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("failed to encode AI Gateway Agent update fields: %w", err)
	}
	if err := json.Unmarshal(data, update); err != nil {
		return fmt.Errorf("failed to decode AI Gateway Agent update fields: %w", err)
	}
	return nil
}

// Create creates an AI Gateway Agent.
func (a *AIGatewayAgentAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayAgentRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewayAgent(ctx, gatewayID, req, namespace)
}

// Update updates an AI Gateway Agent.
func (a *AIGatewayAgentAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayAgentRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayAgent(ctx, gatewayID, id, req, namespace)
}

// Delete deletes an AI Gateway Agent.
func (a *AIGatewayAgentAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayAgent(ctx, gatewayID, id)
}

// GetByID gets an AI Gateway Agent by ID.
func (a *AIGatewayAgentAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}
	agent, err := a.client.GetAIGatewayAgent(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, nil
	}
	return &aiGatewayAgentResourceInfo{agent: agent}, nil
}

// GetByName is not supported without a parent gateway context.
func (a *AIGatewayAgentAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway Agents")
}

// ResourceType returns the resource type.
func (a *AIGatewayAgentAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayAgent
}

// RequiredFields returns required fields for create.
func (a *AIGatewayAgentAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldDisplayName, planner.FieldType, planner.FieldConfig}
}

// SupportsUpdate indicates update support.
func (a *AIGatewayAgentAdapter) SupportsUpdate() bool {
	return true
}

func (a *AIGatewayAgentAdapter) getAIGatewayIDFromExecutionContext(
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

	return "", fmt.Errorf("AI Gateway ID required for Agent operations")
}

type aiGatewayAgentResourceInfo struct {
	agent *state.AIGatewayAgent
}

func (a *aiGatewayAgentResourceInfo) GetID() string {
	return resources.AIGatewayAgentID(a.agent.AIGatewayAgent)
}

func (a *aiGatewayAgentResourceInfo) GetName() string {
	return resources.AIGatewayAgentName(a.agent.AIGatewayAgent)
}

func (a *aiGatewayAgentResourceInfo) GetLabels() map[string]string {
	return resources.AIGatewayAgentLabels(a.agent.AIGatewayAgent)
}

func (a *aiGatewayAgentResourceInfo) GetNormalizedLabels() map[string]string {
	return a.agent.NormalizedLabels
}
