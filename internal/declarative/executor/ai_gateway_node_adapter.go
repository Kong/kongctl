package executor

import (
	"context"
	"fmt"
	"maps"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayNodeAdapter implements ResourceOperations for AI Gateway data plane nodes.
type AIGatewayNodeAdapter struct {
	client *state.Client
}

// NewAIGatewayNodeAdapter creates a new AI Gateway node adapter.
func NewAIGatewayNodeAdapter(client *state.Client) *AIGatewayNodeAdapter {
	return &AIGatewayNodeAdapter{client: client}
}

// MapCreateFields maps planner fields to AIGatewayNodeRequest.
func (a *AIGatewayNodeAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *state.AIGatewayNodeRequest,
) error {
	create.Payload = copyAIGatewayNodeFields(fields)
	return nil
}

// MapUpdateFields maps planner fields to AIGatewayNodeRequest.
func (a *AIGatewayNodeAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *state.AIGatewayNodeRequest,
	_ map[string]string,
) error {
	update.Payload = copyAIGatewayNodeFields(fields)
	return nil
}

// Create upserts an AI Gateway data plane node.
func (a *AIGatewayNodeAdapter) Create(
	ctx context.Context,
	req state.AIGatewayNodeRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	nodeID, err := a.getNodeIDFromRequest(req)
	if err != nil {
		return "", err
	}
	return a.client.UpsertAIGatewayNode(ctx, gatewayID, nodeID, req, namespace)
}

// Update upserts an AI Gateway data plane node.
func (a *AIGatewayNodeAdapter) Update(
	ctx context.Context,
	id string,
	req state.AIGatewayNodeRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	if id == "" {
		id, err = a.getNodeIDFromRequest(req)
		if err != nil {
			return "", err
		}
	}
	return a.client.UpsertAIGatewayNode(ctx, gatewayID, id, req, namespace)
}

// Delete deletes an AI Gateway data plane node.
func (a *AIGatewayNodeAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayNode(ctx, gatewayID, id)
}

// GetByID gets an AI Gateway data plane node by ID.
func (a *AIGatewayNodeAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}
	node, err := a.client.GetAIGatewayNode(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, nil
	}
	return &aiGatewayNodeResourceInfo{node: node}, nil
}

// GetByName is not supported for AI Gateway Nodes.
func (a *AIGatewayNodeAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway Nodes")
}

// ResourceType returns the resource type.
func (a *AIGatewayNodeAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayNode
}

// RequiredFields returns required fields for create.
func (a *AIGatewayNodeAdapter) RequiredFields() []string {
	return []string{planner.FieldID, planner.FieldVersion, planner.FieldHostname, planner.FieldType}
}

// SupportsUpdate indicates update support.
func (a *AIGatewayNodeAdapter) SupportsUpdate() bool {
	return true
}

func (a *AIGatewayNodeAdapter) getAIGatewayIDFromExecutionContext(execCtx *ExecutionContext) (string, error) {
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

	return "", fmt.Errorf("AI Gateway ID required for Node operations")
}

func (a *AIGatewayNodeAdapter) getNodeIDFromRequest(req state.AIGatewayNodeRequest) (string, error) {
	nodeID, ok := req.Payload[planner.FieldID].(string)
	if !ok || nodeID == "" {
		return "", fmt.Errorf("id is required for AI Gateway Node operations")
	}
	return nodeID, nil
}

func copyAIGatewayNodeFields(fields map[string]any) map[string]any {
	payload := make(map[string]any, len(fields))
	maps.Copy(payload, fields)
	return payload
}

type aiGatewayNodeResourceInfo struct {
	node *state.AIGatewayNode
}

func (a *aiGatewayNodeResourceInfo) GetID() string {
	return resources.AIGatewayNodeID(a.node.AIGatewayDataPlaneNode)
}

func (a *aiGatewayNodeResourceInfo) GetName() string {
	return resources.AIGatewayNodeID(a.node.AIGatewayDataPlaneNode)
}

func (a *aiGatewayNodeResourceInfo) GetLabels() map[string]string {
	return map[string]string{}
}

func (a *aiGatewayNodeResourceInfo) GetNormalizedLabels() map[string]string {
	return a.node.NormalizedLabels
}
