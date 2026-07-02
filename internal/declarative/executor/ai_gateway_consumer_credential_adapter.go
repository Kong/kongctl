package executor

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayConsumerCredentialAdapter implements create/delete operations for AI Gateway Consumer Credentials.
type AIGatewayConsumerCredentialAdapter struct {
	client *state.Client
}

// NewAIGatewayConsumerCredentialAdapter creates a new AI Gateway Consumer Credential adapter.
func NewAIGatewayConsumerCredentialAdapter(client *state.Client) *AIGatewayConsumerCredentialAdapter {
	return &AIGatewayConsumerCredentialAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateAIGatewayConsumerCredentialRequest.
func (a *AIGatewayConsumerCredentialAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayConsumerCredentialRequest,
) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("failed to encode AI Gateway Consumer Credential create fields: %w", err)
	}
	if err := json.Unmarshal(data, create); err != nil {
		return fmt.Errorf("failed to decode AI Gateway Consumer Credential create fields: %w", err)
	}
	if create.Name == "" || create.DisplayName == "" || create.Type == "" {
		return fmt.Errorf("name, display_name, and type are required")
	}
	create.APIKey = nil
	return nil
}

// Create creates an AI Gateway Consumer Credential.
func (a *AIGatewayConsumerCredentialAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayConsumerCredentialRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	consumerID, err := a.getAIGatewayConsumerIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewayConsumerCredential(ctx, gatewayID, consumerID, req, namespace)
}

// Delete deletes an AI Gateway Consumer Credential.
func (a *AIGatewayConsumerCredentialAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	consumerID, err := a.getAIGatewayConsumerIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayConsumerCredential(ctx, gatewayID, consumerID, id)
}

// GetByName is not supported without parent gateway and consumer context.
func (a *AIGatewayConsumerCredentialAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway Consumer Credentials")
}

// ResourceType returns the resource type.
func (a *AIGatewayConsumerCredentialAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayConsumerCredential
}

// RequiredFields returns required fields for create.
func (a *AIGatewayConsumerCredentialAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldDisplayName, planner.FieldType}
}

func (a *AIGatewayConsumerCredentialAdapter) getAIGatewayIDFromExecutionContext(
	execCtx *ExecutionContext,
) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context required")
	}

	change := *execCtx.PlannedChange
	if gatewayRef, ok := change.References[planner.FieldAIGatewayID]; ok && !unresolvedReferenceID(gatewayRef.ID) {
		return gatewayRef.ID, nil
	}

	return "", fmt.Errorf("AI Gateway ID required for Consumer Credential operations")
}

func (a *AIGatewayConsumerCredentialAdapter) getAIGatewayConsumerIDFromExecutionContext(
	execCtx *ExecutionContext,
) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context required")
	}

	change := *execCtx.PlannedChange
	if consumerRef, ok := change.References[planner.FieldAIGatewayConsumerID]; ok &&
		!unresolvedReferenceID(consumerRef.ID) {
		return consumerRef.ID, nil
	}
	if change.Parent != nil && !unresolvedReferenceID(change.Parent.ID) {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("AI Gateway Consumer ID required for Consumer Credential operations")
}
