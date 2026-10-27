package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayDatastoreAdapter implements ResourceOperations for AI Gateway Datastores.
type AIGatewayDatastoreAdapter struct {
	client *state.Client
}

// NewAIGatewayDatastoreAdapter creates a new AI Gateway Datastore adapter.
func NewAIGatewayDatastoreAdapter(client *state.Client) *AIGatewayDatastoreAdapter {
	return &AIGatewayDatastoreAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateAIGatewayDatastoreRequest.
func (a *AIGatewayDatastoreAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayDatastoreRequest,
) error {
	if err := mapAIGatewaySDKRequest("AI Gateway Datastore create", fields, create); err != nil {
		return err
	}
	if create.RedisCEDatastore == nil && create.RedisEEDatastore == nil &&
		create.VectorDBDatastore == nil &&
		create.TokenVaultDatastore == nil {
		return fmt.Errorf("type must be a supported AI Gateway Datastore type")
	}
	return nil
}

// MapUpdateFields maps planner fields to UpdateAIGatewayDatastoreRequest.
func (a *AIGatewayDatastoreAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayDatastoreRequest,
	_ map[string]string,
) error {
	if err := mapAIGatewaySDKRequest("AI Gateway Datastore update", fields, update); err != nil {
		return err
	}
	if update.RedisCEDatastore == nil && update.RedisEEDatastore == nil &&
		update.VectorDBDatastore == nil &&
		update.TokenVaultDatastore == nil {
		return fmt.Errorf("type must be a supported AI Gateway Datastore type")
	}
	return nil
}

// Create creates an AI Gateway Datastore.
func (a *AIGatewayDatastoreAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayDatastoreRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewayDatastore(ctx, gatewayID, req, namespace)
}

// Update updates an AI Gateway Datastore.
func (a *AIGatewayDatastoreAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayDatastoreRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayDatastore(ctx, gatewayID, id, req, namespace)
}

// Delete deletes an AI Gateway Datastore.
func (a *AIGatewayDatastoreAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayDatastore(ctx, gatewayID, id)
}

// GetByID gets an AI Gateway Datastore by ID.
func (a *AIGatewayDatastoreAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}
	datastore, err := a.client.GetAIGatewayDatastore(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if datastore == nil {
		return nil, nil
	}
	return &aiGatewayDatastoreResourceInfo{datastore: datastore}, nil
}

// GetByName is not supported without a parent gateway context.
func (a *AIGatewayDatastoreAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway Datastores")
}

// ResourceType returns the resource type.
func (a *AIGatewayDatastoreAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayDatastore
}

// RequiredFields returns required fields for create.
func (a *AIGatewayDatastoreAdapter) RequiredFields() []string {
	return []string{
		planner.FieldType,
		planner.FieldName,
		planner.FieldDisplayName,
		planner.FieldConfig,
	}
}

// SupportsUpdate indicates update support.
func (a *AIGatewayDatastoreAdapter) SupportsUpdate() bool {
	return true
}

func (a *AIGatewayDatastoreAdapter) getAIGatewayIDFromExecutionContext(
	execCtx *ExecutionContext,
) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context required")
	}

	change := *execCtx.PlannedChange
	if gatewayRef, ok := change.References[planner.FieldAIGatewayID]; ok &&
		!unresolvedReferenceID(gatewayRef.ID) {
		return gatewayRef.ID, nil
	}
	if change.Parent != nil && !unresolvedReferenceID(change.Parent.ID) {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("AI Gateway ID required for Datastore operations")
}

type aiGatewayDatastoreResourceInfo struct {
	datastore *state.AIGatewayDatastore
}

func (a *aiGatewayDatastoreResourceInfo) GetID() string {
	return resources.AIGatewayDatastoreID(a.datastore.AIGatewayDatastore)
}

func (a *aiGatewayDatastoreResourceInfo) GetName() string {
	return resources.AIGatewayDatastoreName(a.datastore.AIGatewayDatastore)
}

func (a *aiGatewayDatastoreResourceInfo) GetLabels() map[string]string {
	return resources.AIGatewayDatastoreLabels(a.datastore.AIGatewayDatastore)
}

func (a *aiGatewayDatastoreResourceInfo) GetNormalizedLabels() map[string]string {
	return a.datastore.NormalizedLabels
}
