package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayConfigStoreSecretAdapter implements write-only Config Store secret operations.
type AIGatewayConfigStoreSecretAdapter struct {
	client *state.Client
}

func NewAIGatewayConfigStoreSecretAdapter(client *state.Client) *AIGatewayConfigStoreSecretAdapter {
	return &AIGatewayConfigStoreSecretAdapter{client: client}
}

func (a *AIGatewayConfigStoreSecretAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayConfigStoreSecretRequest,
) error {
	return mapAIGatewaySDKRequest("AI Gateway Config Store secret create", fields, create)
}

func (a *AIGatewayConfigStoreSecretAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayConfigStoreSecretRequest,
	_ map[string]string,
) error {
	return mapAIGatewaySDKRequest("AI Gateway Config Store secret update", fields, update)
}

func (a *AIGatewayConfigStoreSecretAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayConfigStoreSecretRequest,
	_ string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, storeID, err := aiGatewayConfigStoreSecretParentIDs(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewayConfigStoreSecret(ctx, gatewayID, storeID, req)
}

func (a *AIGatewayConfigStoreSecretAdapter) Update(
	ctx context.Context,
	key string,
	req kkComps.UpdateAIGatewayConfigStoreSecretRequest,
	_ string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, storeID, err := aiGatewayConfigStoreSecretParentIDs(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayConfigStoreSecret(ctx, gatewayID, storeID, key, req)
}

func (a *AIGatewayConfigStoreSecretAdapter) Delete(
	ctx context.Context,
	key string,
	execCtx *ExecutionContext,
) error {
	gatewayID, storeID, err := aiGatewayConfigStoreSecretParentIDs(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayConfigStoreSecret(ctx, gatewayID, storeID, key)
}

func (a *AIGatewayConfigStoreSecretAdapter) GetByID(
	ctx context.Context,
	key string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, storeID, err := aiGatewayConfigStoreSecretParentIDs(execCtx)
	if err != nil {
		return nil, err
	}
	secret, err := a.client.GetAIGatewayConfigStoreSecret(ctx, gatewayID, storeID, key)
	if err != nil || secret == nil {
		return nil, err
	}
	return &aiGatewayConfigStoreSecretResourceInfo{secret: secret}, nil
}

func (a *AIGatewayConfigStoreSecretAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway Config Store secrets")
}

func (a *AIGatewayConfigStoreSecretAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayConfigStoreSecret
}

func (a *AIGatewayConfigStoreSecretAdapter) RequiredFields() []string {
	return []string{planner.FieldKey, planner.FieldValue}
}

func (a *AIGatewayConfigStoreSecretAdapter) SupportsUpdate() bool { return true }

func aiGatewayConfigStoreSecretParentIDs(execCtx *ExecutionContext) (string, string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", "", fmt.Errorf("execution context required")
	}
	change := execCtx.PlannedChange
	gateway, ok := change.References[planner.FieldAIGatewayID]
	if !ok || unresolvedReferenceID(gateway.ID) {
		return "", "", fmt.Errorf("AI Gateway ID required for Config Store secret operations")
	}
	if change.Parent != nil && !unresolvedReferenceID(change.Parent.ID) {
		return gateway.ID, change.Parent.ID, nil
	}
	store, ok := change.References[planner.FieldConfigStoreID]
	if ok && !unresolvedReferenceID(store.ID) {
		return gateway.ID, store.ID, nil
	}
	return "", "", fmt.Errorf("AI Gateway Config Store ID required for secret operations")
}

type aiGatewayConfigStoreSecretResourceInfo struct {
	secret *state.AIGatewayConfigStoreSecret
}

func (a *aiGatewayConfigStoreSecretResourceInfo) GetID() string { return a.secret.Key }

func (a *aiGatewayConfigStoreSecretResourceInfo) GetName() string { return a.secret.Key }

func (a *aiGatewayConfigStoreSecretResourceInfo) GetLabels() map[string]string { return nil }

func (a *aiGatewayConfigStoreSecretResourceInfo) GetNormalizedLabels() map[string]string { return nil }
