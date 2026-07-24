package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayConfigStoreAdapter implements Config Store operations.
type AIGatewayConfigStoreAdapter struct {
	client *state.Client
}

func NewAIGatewayConfigStoreAdapter(client *state.Client) *AIGatewayConfigStoreAdapter {
	return &AIGatewayConfigStoreAdapter{client: client}
}

func (a *AIGatewayConfigStoreAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayConfigStoreRequest,
) error {
	return mapAIGatewaySDKRequest("AI Gateway Config Store create", fields, create)
}

func (a *AIGatewayConfigStoreAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayConfigStoreRequest,
	_ map[string]string,
) error {
	return mapAIGatewaySDKRequest("AI Gateway Config Store update", fields, update)
}

func (a *AIGatewayConfigStoreAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayConfigStoreRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := aiGatewayConfigStoreGatewayID(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewayConfigStore(ctx, gatewayID, req, namespace)
}

func (a *AIGatewayConfigStoreAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayConfigStoreRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := aiGatewayConfigStoreGatewayID(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayConfigStore(ctx, gatewayID, id, req, namespace)
}

func (a *AIGatewayConfigStoreAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	gatewayID, err := aiGatewayConfigStoreGatewayID(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayConfigStore(ctx, gatewayID, id)
}

func (a *AIGatewayConfigStoreAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := aiGatewayConfigStoreGatewayID(execCtx)
	if err != nil {
		return nil, err
	}
	store, err := a.client.GetAIGatewayConfigStore(ctx, gatewayID, id)
	if err != nil || store == nil {
		return nil, err
	}
	return &aiGatewayConfigStoreResourceInfo{store: store}, nil
}

func (a *AIGatewayConfigStoreAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway Config Stores")
}

func (a *AIGatewayConfigStoreAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayConfigStore
}

func (a *AIGatewayConfigStoreAdapter) RequiredFields() []string {
	return []string{planner.FieldName}
}

func (a *AIGatewayConfigStoreAdapter) SupportsUpdate() bool {
	return true
}

func aiGatewayConfigStoreGatewayID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context required")
	}
	change := execCtx.PlannedChange
	if gateway, ok := change.References[planner.FieldAIGatewayID]; ok && !unresolvedReferenceID(gateway.ID) {
		return gateway.ID, nil
	}
	if change.Parent != nil && !unresolvedReferenceID(change.Parent.ID) {
		return change.Parent.ID, nil
	}
	return "", fmt.Errorf("AI Gateway ID required for Config Store operations")
}

type aiGatewayConfigStoreResourceInfo struct {
	store *state.AIGatewayConfigStore
}

func (a *aiGatewayConfigStoreResourceInfo) GetID() string {
	return a.store.ID
}

func (a *aiGatewayConfigStoreResourceInfo) GetName() string {
	return a.store.Name
}

func (a *aiGatewayConfigStoreResourceInfo) GetLabels() map[string]string {
	return nil
}

func (a *aiGatewayConfigStoreResourceInfo) GetNormalizedLabels() map[string]string {
	return nil
}
