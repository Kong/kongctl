package executor

import (
	"context"
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayModelAdapter implements ResourceOperations for AI Gateway models.
type AIGatewayModelAdapter struct {
	client *state.Client
}

// NewAIGatewayModelAdapter creates a new AI Gateway model adapter.
func NewAIGatewayModelAdapter(client *state.Client) *AIGatewayModelAdapter {
	return &AIGatewayModelAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateAIGatewayModelRequest.
func (a *AIGatewayModelAdapter) MapCreateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayModelRequest,
) error {
	if err := mapAIGatewaySDKRequest("AI Gateway model create", fields, create); err != nil {
		return err
	}
	if create.AIGatewayModelAPI == nil && create.AIGatewayModelModel == nil {
		return fmt.Errorf("type must be either api or model")
	}
	return validateModelPayloadOwnership(modelCreateMetadata(*create), execCtx)
}

// MapUpdateFields maps planner fields to UpdateAIGatewayModelRequest.
func (a *AIGatewayModelAdapter) MapUpdateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayModelRequest,
	_ map[string]string,
) error {
	if err := mapAIGatewaySDKRequest("AI Gateway model update", fields, update); err != nil {
		return err
	}
	if update.AIGatewayModelAPI == nil && update.AIGatewayModelModel == nil {
		return fmt.Errorf("type must be either api or model")
	}
	return validateModelPayloadOwnership(modelUpdateMetadata(*update), execCtx)
}

// Create creates an AI Gateway model.
func (a *AIGatewayModelAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayModelRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := aiGatewayIDFromExecutionContext(execCtx, "model", true)
	if err != nil {
		return "", err
	}
	if err := validateModelPayloadOwnership(modelCreateMetadata(req), execCtx); err != nil {
		return "", err
	}
	if namespace != execCtx.Namespace {
		return "", fmt.Errorf("AI Gateway model execution namespace mismatch")
	}
	models, err := a.client.ListAIGatewayModels(ctx, gatewayID)
	if err != nil {
		return "", err
	}
	name := ""
	if req.AIGatewayModelAPI != nil {
		name = req.AIGatewayModelAPI.Name
	} else if req.AIGatewayModelModel != nil {
		name = req.AIGatewayModelModel.Name
	}
	for _, model := range models {
		if resources.AIGatewayModelName(model.AIGatewayModel) == name {
			return "", childCreationCollision("AI Gateway model", name)
		}
	}
	return a.client.CreateAIGatewayModel(ctx, gatewayID, req, namespace)
}

// Update updates an AI Gateway model.
func (a *AIGatewayModelAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayModelRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := aiGatewayIDFromExecutionContext(execCtx, "model", true)
	if err != nil {
		return "", err
	}
	if err := validateModelPayloadOwnership(modelUpdateMetadata(req), execCtx); err != nil {
		return "", err
	}
	if namespace != execCtx.Namespace {
		return "", fmt.Errorf("AI Gateway model execution namespace mismatch")
	}
	current, err := a.client.GetAIGatewayModel(ctx, gatewayID, id)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", fmt.Errorf("AI Gateway model no longer exists")
	}
	metadata := resources.AIGatewayModelManagedBy(current.AIGatewayModel)
	if err := resources.ValidateModelOwnership(metadata, namespace); err != nil {
		return "", err
	}
	if err := validateChildPlannedProtection("AI Gateway model", metadata, execCtx.PlannedChange); err != nil {
		return "", err
	}
	if metadata[resources.ModelProtectedKey] == "true" {
		if !childUnprotectOnly(execCtx.PlannedChange, metadata, modelUpdateMetadata(req)) ||
			!modelUnprotectPayloadUnchanged(current.AIGatewayModel, req) {
			return "", fmt.Errorf("AI Gateway model is protected; unprotect it in a separate update")
		}
	}
	requested := modelUpdateMetadata(req)
	canonical := resources.BuildModelManagedBy(
		namespace, resources.IsChildProtected(requested), requested[resources.ManagedByVersionKey],
	)

	if req.AIGatewayModelAPI != nil {
		req.AIGatewayModelAPI.ManagedBy = canonical
	} else {
		req.AIGatewayModelModel.ManagedBy = canonical
	}
	return a.client.UpdateAIGatewayModel(ctx, gatewayID, id, req, namespace)
}

func modelUnprotectPayloadUnchanged(current kkComps.AIGatewayModel, req kkComps.UpdateAIGatewayModelRequest) bool {
	before, err := resources.AIGatewayModelMutablePayloadMap(current)
	if err != nil {
		return false
	}
	model := resources.AIGatewayModelResource{}
	model.AIGatewayModelAPI = req.AIGatewayModelAPI
	model.AIGatewayModelModel = req.AIGatewayModelModel
	after, err := model.MutablePayloadMap()
	if err != nil {
		return false
	}
	delete(before, planner.FieldManagedBy)
	delete(after, planner.FieldManagedBy)
	return reflect.DeepEqual(before, after)
}

// Delete deletes an AI Gateway model.
func (a *AIGatewayModelAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := aiGatewayIDFromExecutionContext(execCtx, "model", true)
	if err != nil {
		return err
	}
	current, err := a.client.GetAIGatewayModel(ctx, gatewayID, id)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	metadata := resources.AIGatewayModelManagedBy(current.AIGatewayModel)
	if err := resources.ValidateModelOwnership(metadata, execCtx.Namespace); err != nil {
		return err
	}
	if metadata[resources.ModelProtectedKey] == "true" {
		return fmt.Errorf("AI Gateway model is protected and cannot be deleted")
	}
	return a.client.DeleteAIGatewayModel(ctx, gatewayID, id)
}

// GetByID gets an AI Gateway model by ID.
func (a *AIGatewayModelAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := aiGatewayIDFromExecutionContext(execCtx, "model", true)
	if err != nil {
		return nil, err
	}
	model, err := a.client.GetAIGatewayModel(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, nil
	}
	metadata := resources.AIGatewayModelManagedBy(model.AIGatewayModel)
	if err := resources.ValidateModelOwnership(metadata, execCtx.Namespace); err != nil {
		return nil, err
	}
	return &aiGatewayModelResourceInfo{model: model}, nil
}

func modelCreateMetadata(req kkComps.CreateAIGatewayModelRequest) map[string]string {
	if req.AIGatewayModelAPI != nil {
		return req.AIGatewayModelAPI.ManagedBy
	}
	if req.AIGatewayModelModel != nil {
		return req.AIGatewayModelModel.ManagedBy
	}
	return nil
}

func modelUpdateMetadata(req kkComps.UpdateAIGatewayModelRequest) map[string]string {
	if req.AIGatewayModelAPI != nil {
		return req.AIGatewayModelAPI.ManagedBy
	}
	if req.AIGatewayModelModel != nil {
		return req.AIGatewayModelModel.ManagedBy
	}
	return nil
}

func validateModelPayloadOwnership(metadata map[string]string, execCtx *ExecutionContext) error {
	if execCtx == nil {
		return nil
	} // Payload mapping alone is also used for schema inspection.
	return validateChildPayloadOwnership("AI Gateway model", metadata, execCtx.Namespace)
}

// GetByName is not supported without a parent gateway context.
func (a *AIGatewayModelAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway models")
}

// ResourceType returns the resource type.
func (a *AIGatewayModelAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayModel
}

// RequiredFields returns required fields for create.
func (a *AIGatewayModelAdapter) RequiredFields() []string {
	return []string{planner.FieldType, planner.FieldName, planner.FieldDisplayName}
}

// SupportsUpdate indicates update support.
func (a *AIGatewayModelAdapter) SupportsUpdate() bool {
	return true
}

type aiGatewayModelResourceInfo struct {
	model *state.AIGatewayModel
}

func (a *aiGatewayModelResourceInfo) GetID() string {
	return resources.AIGatewayModelID(a.model.AIGatewayModel)
}

func (a *aiGatewayModelResourceInfo) GetName() string {
	return resources.AIGatewayModelName(a.model.AIGatewayModel)
}

func (a *aiGatewayModelResourceInfo) GetLabels() map[string]string {
	return resources.AIGatewayModelLabels(a.model.AIGatewayModel)
}

func (a *aiGatewayModelResourceInfo) GetNormalizedLabels() map[string]string {
	return a.model.NormalizedLabels
}
