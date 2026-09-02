package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

type AIGatewayCertificateAdapter struct{ client *state.Client }

func NewAIGatewayCertificateAdapter(client *state.Client) *AIGatewayCertificateAdapter {
	return &AIGatewayCertificateAdapter{client: client}
}

func (a *AIGatewayCertificateAdapter) MapCreateFields(
	_ context.Context, _ *ExecutionContext, fields map[string]any, request *kkComps.CreateAIGatewayCertificateRequest,
) error {
	return mapAIGatewaySDKRequest("AI Gateway certificate create", fields, request)
}

func (a *AIGatewayCertificateAdapter) MapUpdateFields(
	_ context.Context, _ *ExecutionContext, fields map[string]any,
	request *kkComps.UpdateAIGatewayCertificateRequest, _ map[string]string,
) error {
	return mapAIGatewaySDKRequest("AI Gateway certificate update", fields, request)
}

func (a *AIGatewayCertificateAdapter) Create(
	ctx context.Context, request kkComps.CreateAIGatewayCertificateRequest,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := aiGatewayTLSExecutionGatewayID(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewayCertificate(ctx, gatewayID, request, namespace)
}

func (a *AIGatewayCertificateAdapter) Update(
	ctx context.Context, id string, request kkComps.UpdateAIGatewayCertificateRequest,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := aiGatewayTLSExecutionGatewayID(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayCertificate(ctx, gatewayID, id, request, namespace)
}

func (a *AIGatewayCertificateAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := aiGatewayTLSExecutionGatewayID(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayCertificate(ctx, gatewayID, id)
}

func (a *AIGatewayCertificateAdapter) GetByID(
	ctx context.Context, id string, execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := aiGatewayTLSExecutionGatewayID(execCtx)
	if err != nil {
		return nil, err
	}
	certificate, err := a.client.GetAIGatewayCertificate(ctx, gatewayID, id)
	if err != nil || certificate == nil {
		return nil, err
	}
	return aiGatewayTLSResourceInfo{
		id: certificate.ID, name: certificate.Name, labels: certificate.Labels,
		normalizedLabels: certificate.NormalizedLabels,
	}, nil
}

func (*AIGatewayCertificateAdapter) GetByName(context.Context, string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway certificates")
}

func (*AIGatewayCertificateAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayCertificate
}

func (*AIGatewayCertificateAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldCert, planner.FieldKey}
}
func (*AIGatewayCertificateAdapter) SupportsUpdate() bool { return true }

type AIGatewayCACertificateAdapter struct{ client *state.Client }

func NewAIGatewayCACertificateAdapter(client *state.Client) *AIGatewayCACertificateAdapter {
	return &AIGatewayCACertificateAdapter{client: client}
}

func (a *AIGatewayCACertificateAdapter) MapCreateFields(
	_ context.Context, _ *ExecutionContext, fields map[string]any,
	request *kkComps.CreateAIGatewayCACertificateRequest,
) error {
	return mapAIGatewaySDKRequest("AI Gateway CA certificate create", fields, request)
}

func (a *AIGatewayCACertificateAdapter) MapUpdateFields(
	_ context.Context, _ *ExecutionContext, fields map[string]any,
	request *kkComps.UpdateAIGatewayCACertificateRequest, _ map[string]string,
) error {
	return mapAIGatewaySDKRequest("AI Gateway CA certificate update", fields, request)
}

func (a *AIGatewayCACertificateAdapter) Create(
	ctx context.Context, request kkComps.CreateAIGatewayCACertificateRequest,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := aiGatewayTLSExecutionGatewayID(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewayCACertificate(ctx, gatewayID, request, namespace)
}

func (a *AIGatewayCACertificateAdapter) Update(
	ctx context.Context, id string, request kkComps.UpdateAIGatewayCACertificateRequest,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := aiGatewayTLSExecutionGatewayID(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayCACertificate(ctx, gatewayID, id, request, namespace)
}

func (a *AIGatewayCACertificateAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := aiGatewayTLSExecutionGatewayID(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewayCACertificate(ctx, gatewayID, id)
}

func (a *AIGatewayCACertificateAdapter) GetByID(
	ctx context.Context, id string, execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := aiGatewayTLSExecutionGatewayID(execCtx)
	if err != nil {
		return nil, err
	}
	certificate, err := a.client.GetAIGatewayCACertificate(ctx, gatewayID, id)
	if err != nil || certificate == nil {
		return nil, err
	}
	return aiGatewayTLSResourceInfo{
		id: certificate.ID, name: certificate.Name, labels: certificate.Labels,
		normalizedLabels: certificate.NormalizedLabels,
	}, nil
}

func (*AIGatewayCACertificateAdapter) GetByName(context.Context, string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway CA certificates")
}

func (*AIGatewayCACertificateAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayCACertificate
}

func (*AIGatewayCACertificateAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldCert}
}
func (*AIGatewayCACertificateAdapter) SupportsUpdate() bool { return true }

type AIGatewaySNIAdapter struct{ client *state.Client }

func NewAIGatewaySNIAdapter(client *state.Client) *AIGatewaySNIAdapter {
	return &AIGatewaySNIAdapter{client: client}
}

func (a *AIGatewaySNIAdapter) MapCreateFields(
	_ context.Context, _ *ExecutionContext, fields map[string]any, request *kkComps.CreateAIGatewaySNIRequest,
) error {
	return mapAIGatewaySDKRequest("AI Gateway SNI create", fields, request)
}

func (a *AIGatewaySNIAdapter) MapUpdateFields(
	_ context.Context, _ *ExecutionContext, fields map[string]any,
	request *kkComps.UpdateAIGatewaySNIRequest, _ map[string]string,
) error {
	return mapAIGatewaySDKRequest("AI Gateway SNI update", fields, request)
}

func (a *AIGatewaySNIAdapter) Create(
	ctx context.Context, request kkComps.CreateAIGatewaySNIRequest,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := aiGatewayTLSExecutionGatewayID(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateAIGatewaySNI(ctx, gatewayID, request, namespace)
}

func (a *AIGatewaySNIAdapter) Update(
	ctx context.Context, id string, request kkComps.UpdateAIGatewaySNIRequest,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := aiGatewayTLSExecutionGatewayID(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewaySNI(ctx, gatewayID, id, request, namespace)
}

func (a *AIGatewaySNIAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := aiGatewayTLSExecutionGatewayID(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteAIGatewaySNI(ctx, gatewayID, id)
}

func (a *AIGatewaySNIAdapter) GetByID(
	ctx context.Context, id string, execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := aiGatewayTLSExecutionGatewayID(execCtx)
	if err != nil {
		return nil, err
	}
	sni, err := a.client.GetAIGatewaySNI(ctx, gatewayID, id)
	if err != nil || sni == nil {
		return nil, err
	}
	return aiGatewayTLSResourceInfo{
		id: sni.ID, name: sni.Name, labels: sni.Labels, normalizedLabels: sni.NormalizedLabels,
	}, nil
}

func (*AIGatewaySNIAdapter) GetByName(context.Context, string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway SNIs")
}
func (*AIGatewaySNIAdapter) ResourceType() string { return planner.ResourceTypeAIGatewaySNI }
func (*AIGatewaySNIAdapter) RequiredFields() []string {
	return []string{
		planner.FieldName, planner.FieldDisplayName, planner.FieldHostname, planner.FieldCertificate,
	}
}
func (*AIGatewaySNIAdapter) SupportsUpdate() bool { return true }

func aiGatewayTLSExecutionGatewayID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context required")
	}
	change := execCtx.PlannedChange
	if reference, ok := change.References[planner.FieldAIGatewayID]; ok && !unresolvedReferenceID(reference.ID) {
		return reference.ID, nil
	}
	if change.Parent != nil && !unresolvedReferenceID(change.Parent.ID) {
		return change.Parent.ID, nil
	}
	return "", fmt.Errorf("AI Gateway ID required for TLS resource operations")
}

type aiGatewayTLSResourceInfo struct {
	id, name         string
	labels           map[string]string
	normalizedLabels map[string]string
}

func (a aiGatewayTLSResourceInfo) GetID() string                          { return a.id }
func (a aiGatewayTLSResourceInfo) GetName() string                        { return a.name }
func (a aiGatewayTLSResourceInfo) GetLabels() map[string]string           { return a.labels }
func (a aiGatewayTLSResourceInfo) GetNormalizedLabels() map[string]string { return a.normalizedLabels }
