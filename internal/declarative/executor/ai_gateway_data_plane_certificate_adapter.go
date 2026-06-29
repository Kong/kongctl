package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayDataPlaneCertificateAdapter implements create/delete operations for AI Gateway data plane certificates.
type AIGatewayDataPlaneCertificateAdapter struct {
	client *state.Client
}

// NewAIGatewayDataPlaneCertificateAdapter creates a new AI Gateway data plane certificate adapter.
func NewAIGatewayDataPlaneCertificateAdapter(client *state.Client) *AIGatewayDataPlaneCertificateAdapter {
	return &AIGatewayDataPlaneCertificateAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateAIGatewayDataPlaneCertificateRequest.
func (a *AIGatewayDataPlaneCertificateAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayDataPlaneCertificateRequest,
) error {
	cert, ok := fields[planner.FieldCert].(string)
	if !ok || cert == "" {
		return fmt.Errorf("cert is required")
	}
	title, ok := fields[planner.FieldTitle].(string)
	if !ok || title == "" {
		return fmt.Errorf("title is required")
	}

	create.Cert = cert
	create.Title = title
	if description, ok := fields[planner.FieldDescription].(string); ok {
		create.Description = &description
	}
	return nil
}

// Create creates an AI Gateway data plane certificate.
func (a *AIGatewayDataPlaneCertificateAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayDataPlaneCertificateRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewayDataPlaneCertificate(ctx, gatewayID, req, namespace)
}

// Delete deletes an AI Gateway data plane certificate.
func (a *AIGatewayDataPlaneCertificateAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayDataPlaneCertificate(ctx, gatewayID, id)
}

// GetByName is not supported without a parent gateway context.
func (a *AIGatewayDataPlaneCertificateAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway data plane certificates")
}

// ResourceType returns the resource type.
func (a *AIGatewayDataPlaneCertificateAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayDataPlaneCertificate
}

// RequiredFields returns required fields for create.
func (a *AIGatewayDataPlaneCertificateAdapter) RequiredFields() []string {
	return []string{planner.FieldCert, planner.FieldTitle}
}

func (a *AIGatewayDataPlaneCertificateAdapter) getAIGatewayIDFromExecutionContext(
	execCtx *ExecutionContext,
) (string, error) {
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

	return "", fmt.Errorf("AI Gateway ID required for data plane certificate operations")
}
