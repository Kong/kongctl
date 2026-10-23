package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/resources"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayProviderAdapter implements ResourceOperations for AI Gateway Model Providers.
type AIGatewayProviderAdapter struct {
	client *state.Client
}

// NewAIGatewayProviderAdapter creates a new AI Gateway Model Provider adapter.
func NewAIGatewayProviderAdapter(client *state.Client) *AIGatewayProviderAdapter {
	return &AIGatewayProviderAdapter{client: client}
}

func (a *AIGatewayProviderAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayModelProviderRequest,
) error {
	payload, err := aiGatewayProviderPayloadFromFields(fields)
	if err != nil {
		return err
	}

	return mapAIGatewaySDKRequest("AI Gateway Model Provider create", payload, create)
}

func (a *AIGatewayProviderAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayModelProviderRequest,
	_ map[string]string,
) error {
	payload, err := aiGatewayProviderPayloadFromFields(fields)
	if err != nil {
		return err
	}

	return mapAIGatewaySDKRequest("AI Gateway Model Provider update", payload, update)
}

func (a *AIGatewayProviderAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayModelProviderRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	payload, err := providerRequestPayload(req)
	if err != nil {
		return "", err
	}
	if err := validateProviderWriteContext(execCtx, payload, namespace); err != nil {
		return "", err
	}
	providers, err := a.client.ListAIGatewayProviders(ctx, gatewayID)
	if err != nil {
		return "", err
	}
	for _, provider := range providers {
		if provider.Name == payload[planner.FieldName] {
			return "", childCreationCollision("AI Gateway provider", provider.Name)
		}
	}
	return a.client.CreateAIGatewayProvider(ctx, gatewayID, req, namespace)
}

func (a *AIGatewayProviderAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayModelProviderRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	payload, err := providerRequestPayload(req)
	if err != nil {
		return "", err
	}
	if err := validateProviderWriteContext(execCtx, payload, namespace); err != nil {
		return "", err
	}
	current, err := a.client.GetAIGatewayProvider(ctx, gatewayID, id)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", fmt.Errorf("AI Gateway provider no longer exists")
	}
	if err := resources.ValidateChildOwnership("AI Gateway provider", current.ManagedBy, namespace); err != nil {
		return "", err
	}
	if err := validateChildPlannedProtection("AI Gateway provider", current.ManagedBy, execCtx.PlannedChange); err != nil {
		return "", err
	}
	if current.Type != payload[planner.FieldType] || current.Name != payload[planner.FieldName] {
		return "", fmt.Errorf("AI Gateway provider name and type are immutable")
	}
	if _, present := payload[planner.FieldLabels]; !present {
		if desired, explicit := execCtx.PlannedChange.Fields[planner.FieldLabels]; explicit {
			payload[planner.FieldLabels] = desired
		} else {
			payload[planner.FieldLabels] = current.Labels
		}
	}
	metadata := providerPayloadMetadata(payload)
	if current.ManagedBy[resources.ChildProtectedKey] == "true" {
		before := map[string]any{
			planner.FieldName: current.Name, planner.FieldType: current.Type,
			planner.FieldDisplayName: current.DisplayName,
			planner.FieldConfig:      current.Config, planner.FieldLabels: current.Labels,
		}
		encoded, err := json.Marshal(before)
		if err != nil {
			return "", fmt.Errorf("cannot compare provider protection payload")
		}
		var observedPayload map[string]any
		if err := json.Unmarshal(encoded, &observedPayload); err != nil {
			return "", fmt.Errorf("cannot compare provider protection payload")
		}
		after := maps.Clone(payload)
		delete(after, planner.FieldManagedBy)
		after, err = providerRequestPayload(after)
		if err != nil {
			return "", err
		}
		if !childUnprotectOnly(execCtx.PlannedChange, current.ManagedBy, metadata) ||
			!reflect.DeepEqual(observedPayload, after) {
			return "", fmt.Errorf("AI Gateway provider is protected; unprotect it in a separate update")
		}
	}
	merged := maps.Clone(current.ManagedBy)
	if merged == nil {
		merged = make(map[string]string)
	}
	maps.Copy(merged, metadata)
	if err := resources.ValidateChildManagedBy(merged, false); err != nil {
		return "", err
	}
	payload[planner.FieldManagedBy] = merged
	if err := mapAIGatewaySDKRequest("AI Gateway provider update", payload, &req); err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayProvider(ctx, gatewayID, id, req, namespace)
}

func (a *AIGatewayProviderAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	current, err := a.client.GetAIGatewayProvider(ctx, gatewayID, id)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	if err := resources.ValidateChildOwnership("AI Gateway provider", current.ManagedBy, execCtx.Namespace); err != nil {
		return err
	}
	if current.ManagedBy[resources.ChildProtectedKey] == "true" {
		return fmt.Errorf("AI Gateway provider is protected and cannot be deleted")
	}
	return a.client.DeleteAIGatewayProvider(ctx, gatewayID, id)
}

func (a *AIGatewayProviderAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway Model Providers")
}

func (a *AIGatewayProviderAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}

	provider, err := a.client.GetAIGatewayProvider(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, nil
	}
	return &aiGatewayProviderResourceInfo{provider: provider}, nil
}

func (a *AIGatewayProviderAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayProvider
}

func (a *AIGatewayProviderAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldType, planner.FieldDisplayName, planner.FieldConfig}
}

func (a *AIGatewayProviderAdapter) SupportsUpdate() bool {
	return true
}

func (a *AIGatewayProviderAdapter) getAIGatewayIDFromExecutionContext(
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
	return "", fmt.Errorf("AI Gateway ID required for AI Gateway Model Provider operations")
}

func aiGatewayProviderPayloadFromFields(fields map[string]any) (map[string]any, error) {
	name, ok := fields[planner.FieldName].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("name is required")
	}
	providerType, ok := fields[planner.FieldType].(string)
	if !ok || providerType == "" {
		return nil, fmt.Errorf("type is required")
	}
	displayName, ok := fields[planner.FieldDisplayName].(string)
	if !ok || displayName == "" {
		return nil, fmt.Errorf("display_name is required")
	}
	config, ok := fields[planner.FieldConfig]
	if !ok || config == nil {
		return nil, fmt.Errorf("config is required")
	}

	payload := map[string]any{
		planner.FieldName:        name,
		planner.FieldType:        providerType,
		planner.FieldDisplayName: displayName,
		planner.FieldConfig:      config,
	}
	if labels, ok := fields[planner.FieldLabels]; ok {
		payload[planner.FieldLabels] = labels
	}
	if managedBy, ok := fields[planner.FieldManagedBy]; ok {
		payload[planner.FieldManagedBy] = managedBy
	}
	return payload, nil
}

type aiGatewayProviderResourceInfo struct {
	provider *state.AIGatewayProvider
}

func (a *aiGatewayProviderResourceInfo) GetID() string {
	return a.provider.ID
}

func (a *aiGatewayProviderResourceInfo) GetName() string {
	return a.provider.Name
}

func (a *aiGatewayProviderResourceInfo) GetLabels() map[string]string {
	return a.provider.Labels
}

func (a *aiGatewayProviderResourceInfo) GetNormalizedLabels() map[string]string {
	return a.provider.NormalizedLabels
}

func providerRequestPayload(request any) (map[string]any, error) {
	encoded, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("cannot encode provider payload")
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return nil, fmt.Errorf("cannot decode provider payload")
	}
	return payload, nil
}

func providerPayloadMetadata(payload map[string]any) map[string]string {
	metadata := make(map[string]string)
	if values, ok := payload[planner.FieldManagedBy].(map[string]any); ok {
		for k, v := range values {
			if text, ok := v.(string); ok {
				metadata[k] = text
			}
		}
	}
	return metadata
}

func validateProviderWriteContext(execCtx *ExecutionContext, payload map[string]any, namespace string) error {
	if execCtx == nil || execCtx.PlannedChange == nil || namespace != execCtx.Namespace {
		return fmt.Errorf("AI Gateway provider execution namespace mismatch")
	}
	return validateChildPayloadOwnership("AI Gateway provider", providerPayloadMetadata(payload), namespace)
}
