package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayConsumerCredentialAdapter implements create/delete operations for AI Gateway Consumer Credentials.
type AIGatewayConsumerCredentialAdapter struct {
	client                     *state.Client
	initiallyProtectedConsumer func(string) bool
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
	if err := mapAIGatewaySDKRequest("AI Gateway Consumer Credential create", fields, create); err != nil {
		return err
	}
	if create.Name == "" || create.DisplayName == "" || create.Type == "" {
		return fmt.Errorf("name, display_name, and type are required")
	}
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
	if err := a.validateConsumerWrite(ctx, gatewayID, consumerID, namespace, true); err != nil {
		return "", err
	}
	credentials, err := a.client.ListAIGatewayConsumerCredentials(ctx, gatewayID, consumerID)
	if err != nil {
		return "", err
	}
	for _, credential := range credentials {
		if credential.Name == req.Name {
			return "", childCreationCollision("AI Gateway consumer credential", req.Name)
		}
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
	if err := a.validateConsumerWrite(ctx, gatewayID, consumerID, execCtx.Namespace, false); err != nil {
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

func (a *AIGatewayConsumerCredentialAdapter) validateConsumerWrite(
	ctx context.Context,
	gatewayID, consumerID, namespace string,
	allowInitial bool,
) error {
	consumer, err := a.client.GetAIGatewayConsumer(ctx, gatewayID, consumerID)
	if err != nil {
		return err
	}
	if consumer == nil {
		return fmt.Errorf("AI Gateway consumer no longer exists")
	}
	if err := resources.ValidateChildOwnership("AI Gateway consumer", consumer.ManagedBy, namespace); err != nil {
		return err
	}
	if resources.IsChildProtected(consumer.ManagedBy) &&
		(!allowInitial || a.initiallyProtectedConsumer == nil || !a.initiallyProtectedConsumer(consumerID)) {
		return fmt.Errorf("AI Gateway consumer is protected; credential writes are not allowed")
	}
	return nil
}
