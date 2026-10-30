package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayAdapter implements ResourceOperations for AI Gateways.
type AIGatewayAdapter struct {
	client *state.Client
}

// NewAIGatewayAdapter creates a new AI Gateway adapter.
func NewAIGatewayAdapter(client *state.Client) *AIGatewayAdapter {
	return &AIGatewayAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateAIGatewayRequest.
func (a *AIGatewayAdapter) MapCreateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayRequest,
) error {
	if err := mapAIGatewaySDKRequest("AI Gateway create", fields, create); err != nil {
		return err
	}
	if create.Name == "" {
		return fmt.Errorf("AI Gateway name is required")
	}

	userLabels := labels.ExtractLabelsFromField(fields[planner.FieldLabels])
	create.Labels = labels.BuildCreateLabels(userLabels, execCtx.Namespace, execCtx.Protection)

	return nil
}

// MapUpdateFields maps planner fields to UpdateAIGatewayRequest.
func (a *AIGatewayAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayRequest,
	_ map[string]string,
) error {
	if err := mapAIGatewaySDKRequest("AI Gateway update", fields, update); err != nil {
		return err
	}
	if update.Name == "" {
		return fmt.Errorf("AI Gateway name is required")
	}
	return nil
}

func (a *AIGatewayAdapter) MapUpdateLabels(
	execCtx *ExecutionContext,
	update *kkComps.UpdateAIGatewayRequest,
	desiredLabels map[string]string,
	currentLabels map[string]string,
) {
	update.Labels = labels.BuildUpdateStringLabels(
		desiredLabels,
		currentLabels,
		execCtx.Namespace,
		execCtx.Protection,
	)
}

// Create creates an AI Gateway.
func (a *AIGatewayAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayRequest,
	namespace string,
	_ *ExecutionContext,
) (string, error) {
	resp, err := a.client.CreateAIGateway(ctx, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Update updates an AI Gateway.
func (a *AIGatewayAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayRequest,
	namespace string,
	_ *ExecutionContext,
) (string, error) {
	resp, err := a.client.UpdateAIGateway(ctx, id, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Delete deletes an AI Gateway.
func (a *AIGatewayAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	if execCtx == nil || execCtx.Namespace == "" {
		return fmt.Errorf("AI Gateway execution namespace required for cascade deletion")
	}
	current, err := a.client.GetAIGatewayByID(ctx, id)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	owner := current.NormalizedLabels[labels.NamespaceKey]
	if owner == "" {
		return fmt.Errorf(
			"AI Gateway is unowned; explicitly adopt it into namespace %q before deleting it", execCtx.Namespace,
		)
	}
	if owner != execCtx.Namespace {
		return fmt.Errorf(
			"AI Gateway belongs to namespace %q, not %q; only its owner can delete the gateway",
			owner, execCtx.Namespace,
		)
	}
	if labels.IsProtectedResource(current.NormalizedLabels) {
		return fmt.Errorf("AI Gateway is protected and cannot be deleted")
	}
	return a.client.DeleteAIGateway(ctx, id)
}

// GetByName fetches an AI Gateway by name.
func (a *AIGatewayAdapter) GetByName(ctx context.Context, name string) (ResourceInfo, error) {
	gateway, err := a.client.GetAIGatewayByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if gateway == nil {
		return nil, nil
	}
	return &aiGatewayResourceInfo{gateway: gateway}, nil
}

// GetByID fetches an AI Gateway by ID.
func (a *AIGatewayAdapter) GetByID(ctx context.Context, id string, _ *ExecutionContext) (ResourceInfo, error) {
	gateway, err := a.client.GetAIGatewayByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if gateway == nil {
		return nil, nil
	}
	return &aiGatewayResourceInfo{gateway: gateway}, nil
}

// ResourceType returns the resource type.
func (a *AIGatewayAdapter) ResourceType() string {
	return planner.ResourceTypeAIGateway
}

// RequiredFields returns required fields for create.
func (a *AIGatewayAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldDisplayName}
}

// SupportsUpdate indicates update support.
func (a *AIGatewayAdapter) SupportsUpdate() bool {
	return true
}

type aiGatewayResourceInfo struct {
	gateway *state.AIGateway
}

func (a *aiGatewayResourceInfo) GetID() string {
	return a.gateway.ID
}

func (a *aiGatewayResourceInfo) GetName() string {
	return a.gateway.Name
}

func (a *aiGatewayResourceInfo) GetLabels() map[string]string {
	return a.gateway.Labels
}

func (a *aiGatewayResourceInfo) GetNormalizedLabels() map[string]string {
	return a.gateway.NormalizedLabels
}

func aiGatewayIDFromExecutionContext(execCtx *ExecutionContext, resource string, useParent bool) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context required")
	}
	change := execCtx.PlannedChange
	if ref, ok := change.References[planner.FieldAIGatewayID]; ok && !unresolvedReferenceID(ref.ID) {
		return ref.ID, nil
	}
	// Consumer credentials have a consumer parent rather than a gateway parent.
	if useParent && change.Parent != nil && !unresolvedReferenceID(change.Parent.ID) {
		return change.Parent.ID, nil
	}
	return "", fmt.Errorf("AI Gateway ID required for %s operations", resource)
}
