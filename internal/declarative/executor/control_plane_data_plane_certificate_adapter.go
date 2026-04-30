package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

type ControlPlaneDataPlaneCertificateAdapter struct {
	client *state.Client
}

func NewControlPlaneDataPlaneCertificateAdapter(client *state.Client) *ControlPlaneDataPlaneCertificateAdapter {
	return &ControlPlaneDataPlaneCertificateAdapter{client: client}
}

func (a *ControlPlaneDataPlaneCertificateAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.DataPlaneClientCertificateRequest,
) error {
	cert, ok := fields[planner.FieldCert].(string)
	if !ok || cert == "" {
		return fmt.Errorf("cert is required")
	}
	create.Cert = cert
	return nil
}

func (a *ControlPlaneDataPlaneCertificateAdapter) Create(
	ctx context.Context,
	req kkComps.DataPlaneClientCertificateRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	controlPlaneID, err := a.getControlPlaneIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateControlPlaneDataPlaneCertificate(ctx, controlPlaneID, req, namespace)
}

func (a *ControlPlaneDataPlaneCertificateAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	controlPlaneID, err := a.getControlPlaneIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteControlPlaneDataPlaneCertificate(ctx, controlPlaneID, id)
}

func (a *ControlPlaneDataPlaneCertificateAdapter) GetByName(
	_ context.Context,
	_ string,
) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for data plane certificates")
}

func (a *ControlPlaneDataPlaneCertificateAdapter) ResourceType() string {
	return planner.ResourceTypeControlPlaneDataPlaneCertificate
}

func (a *ControlPlaneDataPlaneCertificateAdapter) RequiredFields() []string {
	return []string{planner.FieldCert}
}

func (a *ControlPlaneDataPlaneCertificateAdapter) getControlPlaneIDFromExecutionContext(
	execCtx *ExecutionContext,
) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context required")
	}

	change := *execCtx.PlannedChange

	if controlPlaneRef, ok := change.References[planner.FieldControlPlaneID]; ok && controlPlaneRef.ID != "" {
		return controlPlaneRef.ID, nil
	}

	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("control plane ID required for data plane certificate operations")
}
