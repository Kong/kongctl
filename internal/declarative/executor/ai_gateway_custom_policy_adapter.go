package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayCustomPolicyAdapter implements ResourceOperations for AI Gateway Custom Policies.
type AIGatewayCustomPolicyAdapter struct {
	client *state.Client
}

// NewAIGatewayCustomPolicyAdapter creates a new AI Gateway Custom Policy adapter.
func NewAIGatewayCustomPolicyAdapter(client *state.Client) *AIGatewayCustomPolicyAdapter {
	return &AIGatewayCustomPolicyAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateAIGatewayCustomPolicyRequest.
func (a *AIGatewayCustomPolicyAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayCustomPolicyRequest,
) error {
	if err := mapAIGatewaySDKRequest("AI Gateway Custom Policy create", fields, create); err != nil {
		return err
	}
	return nil
}

// MapUpdateFields maps planner fields to UpdateAIGatewayCustomPolicyRequest.
func (a *AIGatewayCustomPolicyAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayCustomPolicyRequest,
	_ map[string]string,
) error {
	return mapAIGatewaySDKRequest("AI Gateway Custom Policy update", fields, update)
}

// Create creates an AI Gateway Custom Policy.
func (a *AIGatewayCustomPolicyAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayCustomPolicyRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewayCustomPolicy(ctx, gatewayID, req, namespace)
}

// Update updates an AI Gateway Custom Policy.
func (a *AIGatewayCustomPolicyAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayCustomPolicyRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayCustomPolicy(ctx, gatewayID, id, req, namespace)
}

// Delete deletes an AI Gateway Custom Policy.
func (a *AIGatewayCustomPolicyAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayCustomPolicy(ctx, gatewayID, id)
}

// GetByID gets an AI Gateway Custom Policy by ID.
func (a *AIGatewayCustomPolicyAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}
	policy, err := a.client.GetAIGatewayCustomPolicy(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, nil
	}
	return &aiGatewayCustomPolicyResourceInfo{policy: policy}, nil
}

// GetByName is not supported without a parent gateway context.
func (a *AIGatewayCustomPolicyAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway Custom Policies")
}

// ResourceType returns the resource type.
func (a *AIGatewayCustomPolicyAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayCustomPolicy
}

// RequiredFields returns required fields for create.
func (a *AIGatewayCustomPolicyAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldType, planner.FieldDisplayName, planner.FieldSchema}
}

// SupportsUpdate indicates update support.
func (a *AIGatewayCustomPolicyAdapter) SupportsUpdate() bool {
	return true
}

func (a *AIGatewayCustomPolicyAdapter) getAIGatewayIDFromExecutionContext(execCtx *ExecutionContext) (string, error) {
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

	return "", fmt.Errorf("AI Gateway ID required for Policy operations")
}

type aiGatewayCustomPolicyResourceInfo struct {
	policy *state.AIGatewayCustomPolicy
}

func (a *aiGatewayCustomPolicyResourceInfo) GetID() string {
	return resources.AIGatewayCustomPolicyID(a.policy.AIGatewayCustomPolicy)
}

func (a *aiGatewayCustomPolicyResourceInfo) GetName() string {
	return resources.AIGatewayCustomPolicyName(a.policy.AIGatewayCustomPolicy)
}

func (a *aiGatewayCustomPolicyResourceInfo) GetLabels() map[string]string {
	return nil
}

func (a *aiGatewayCustomPolicyResourceInfo) GetNormalizedLabels() map[string]string {
	return a.policy.NormalizedLabels
}
