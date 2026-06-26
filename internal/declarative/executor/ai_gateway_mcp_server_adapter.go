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

// AIGatewayMCPServerAdapter implements ResourceOperations for AI Gateway MCP Servers.
type AIGatewayMCPServerAdapter struct {
	client *state.Client
}

// NewAIGatewayMCPServerAdapter creates a new AI Gateway MCP Server adapter.
func NewAIGatewayMCPServerAdapter(client *state.Client) *AIGatewayMCPServerAdapter {
	return &AIGatewayMCPServerAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateAIGatewayMCPServerRequest.
func (a *AIGatewayMCPServerAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayMCPServerRequest,
) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("failed to encode AI Gateway MCP Server create fields: %w", err)
	}
	if err := json.Unmarshal(data, create); err != nil {
		return fmt.Errorf("failed to decode AI Gateway MCP Server create fields: %w", err)
	}
	if create.AIGatewayMCPServerConversionOnly == nil &&
		create.AIGatewayMCPServerConversionListener == nil &&
		create.AIGatewayMCPServerListener == nil &&
		create.AIGatewayMCPServerPassthroughListener == nil &&
		create.AIGatewayMCPServerUpstreamServer == nil {
		return fmt.Errorf("type must be a supported AI Gateway MCP Server type")
	}
	return nil
}

// MapUpdateFields maps planner fields to UpdateAIGatewayMCPServerRequest.
func (a *AIGatewayMCPServerAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayMCPServerRequest,
	_ map[string]string,
) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("failed to encode AI Gateway MCP Server update fields: %w", err)
	}
	if err := json.Unmarshal(data, update); err != nil {
		return fmt.Errorf("failed to decode AI Gateway MCP Server update fields: %w", err)
	}
	if update.AIGatewayMCPServerConversionOnly == nil &&
		update.AIGatewayMCPServerConversionListener == nil &&
		update.AIGatewayMCPServerListener == nil &&
		update.AIGatewayMCPServerPassthroughListener == nil &&
		update.AIGatewayMCPServerUpstreamServer == nil {
		return fmt.Errorf("type must be a supported AI Gateway MCP Server type")
	}
	return nil
}

// Create creates an AI Gateway MCP Server.
func (a *AIGatewayMCPServerAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayMCPServerRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewayMCPServer(ctx, gatewayID, req, namespace)
}

// Update updates an AI Gateway MCP Server.
func (a *AIGatewayMCPServerAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayMCPServerRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayMCPServer(ctx, gatewayID, id, req, namespace)
}

// Delete deletes an AI Gateway MCP Server.
func (a *AIGatewayMCPServerAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayMCPServer(ctx, gatewayID, id)
}

// GetByID gets an AI Gateway MCP Server by ID.
func (a *AIGatewayMCPServerAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}
	server, err := a.client.GetAIGatewayMCPServer(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, nil
	}
	return &aiGatewayMCPServerResourceInfo{server: server}, nil
}

// GetByName is not supported without a parent gateway context.
func (a *AIGatewayMCPServerAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway MCP Servers")
}

// ResourceType returns the resource type.
func (a *AIGatewayMCPServerAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayMCPServer
}

// RequiredFields returns required fields for create.
func (a *AIGatewayMCPServerAdapter) RequiredFields() []string {
	return []string{planner.FieldType, planner.FieldName, planner.FieldDisplayName}
}

// SupportsUpdate indicates update support.
func (a *AIGatewayMCPServerAdapter) SupportsUpdate() bool {
	return true
}

func (a *AIGatewayMCPServerAdapter) getAIGatewayIDFromExecutionContext(execCtx *ExecutionContext) (string, error) {
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

	return "", fmt.Errorf("AI Gateway ID required for MCP Server operations")
}

type aiGatewayMCPServerResourceInfo struct {
	server *state.AIGatewayMCPServer
}

func (a *aiGatewayMCPServerResourceInfo) GetID() string {
	return resources.AIGatewayMCPServerID(a.server.AIGatewayMCPServer)
}

func (a *aiGatewayMCPServerResourceInfo) GetName() string {
	return resources.AIGatewayMCPServerName(a.server.AIGatewayMCPServer)
}

func (a *aiGatewayMCPServerResourceInfo) GetLabels() map[string]string {
	return resources.AIGatewayMCPServerLabels(a.server.AIGatewayMCPServer)
}

func (a *aiGatewayMCPServerResourceInfo) GetNormalizedLabels() map[string]string {
	return a.server.NormalizedLabels
}
