package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
)

// PortalAuditLogWebhookAdapter implements ResourceOperations for portal audit-log webhooks.
type PortalAuditLogWebhookAdapter struct {
	client *state.Client
}

// NewPortalAuditLogWebhookAdapter creates a new adapter.
func NewPortalAuditLogWebhookAdapter(client *state.Client) *PortalAuditLogWebhookAdapter {
	return &PortalAuditLogWebhookAdapter{client: client}
}

func (a *PortalAuditLogWebhookAdapter) MapCreateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	create *kkComps.UpdatePortalAuditLogWebhook,
) error {
	return a.mapFields(execCtx, fields, create)
}

func (a *PortalAuditLogWebhookAdapter) MapUpdateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdatePortalAuditLogWebhook,
	_ map[string]string,
) error {
	return a.mapFields(execCtx, fields, update)
}

func (a *PortalAuditLogWebhookAdapter) Create(
	ctx context.Context,
	req kkComps.UpdatePortalAuditLogWebhook,
	_ string,
	execCtx *ExecutionContext,
) (string, error) {
	portalID, err := a.portalID(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdatePortalAuditLogWebhook(ctx, portalID, &req)
}

func (a *PortalAuditLogWebhookAdapter) Update(
	ctx context.Context,
	_ string,
	req kkComps.UpdatePortalAuditLogWebhook,
	_ string,
	execCtx *ExecutionContext,
) (string, error) {
	portalID, err := a.portalID(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdatePortalAuditLogWebhook(ctx, portalID, &req)
}

func (a *PortalAuditLogWebhookAdapter) Delete(ctx context.Context, _ string, execCtx *ExecutionContext) error {
	portalID, err := a.portalID(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeletePortalAuditLogWebhook(ctx, portalID)
}

func (a *PortalAuditLogWebhookAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, nil
}

func (a *PortalAuditLogWebhookAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	portalID, err := a.portalIDWithFallback(execCtx, id)
	if err != nil {
		return nil, err
	}

	webhook, err := a.client.GetPortalAuditLogWebhook(ctx, portalID)
	if err != nil || webhook == nil {
		return nil, err
	}

	return &portalAuditLogWebhookInfo{portalID: portalID, webhook: webhook}, nil
}

func (a *PortalAuditLogWebhookAdapter) ResourceType() string {
	return planner.ResourceTypePortalAuditLogWebhook
}

func (a *PortalAuditLogWebhookAdapter) RequiredFields() []string {
	return nil
}

func (a *PortalAuditLogWebhookAdapter) SupportsUpdate() bool {
	return true
}

func (a *PortalAuditLogWebhookAdapter) mapFields(
	execCtx *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdatePortalAuditLogWebhook,
) error {
	if enabled, ok := fields[planner.FieldEnabled].(bool); ok {
		update.Enabled = &enabled
	}

	destinationID, ok, err := auditLogDestinationID(execCtx, fields)
	if err != nil {
		return err
	}
	if ok {
		update.AuditLogDestinationID = &destinationID
	}

	return nil
}

func (a *PortalAuditLogWebhookAdapter) portalID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for portal audit-log webhook operations")
	}

	return a.portalIDWithFallback(execCtx, "")
}

func (a *PortalAuditLogWebhookAdapter) portalIDWithFallback(
	execCtx *ExecutionContext,
	fallback string,
) (string, error) {
	if execCtx != nil && execCtx.PlannedChange != nil {
		change := *execCtx.PlannedChange

		if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID != "" {
			return portalRef.ID, nil
		}

		if change.Parent != nil && change.Parent.ID != "" {
			return change.Parent.ID, nil
		}

		if change.ResourceID != "" {
			return change.ResourceID, nil
		}
	}

	if fallback != "" {
		return fallback, nil
	}

	return "", fmt.Errorf("portal ID is required for portal audit-log webhook operations")
}

func auditLogDestinationID(execCtx *ExecutionContext, fields map[string]any) (string, bool, error) {
	if execCtx != nil && execCtx.PlannedChange != nil {
		if ref, ok := execCtx.PlannedChange.References[planner.FieldAuditLogDestinationID]; ok {
			if ref.ID == "" || ref.ID == resources.UnknownReferenceID {
				return "", false, errUnresolvedRef(planner.FieldAuditLogDestinationID, ref.Ref)
			}
			return ref.ID, true, nil
		}
	}

	raw, ok := fields[planner.FieldAuditLogDestinationID]
	if !ok || raw == nil {
		return "", false, nil
	}

	destinationID, ok := raw.(string)
	if !ok {
		return "", false, fmt.Errorf("audit_log_destination_id must be a string")
	}
	if tags.IsRefPlaceholder(destinationID) {
		return "", false, errUnresolvedRef(planner.FieldAuditLogDestinationID, destinationID)
	}

	return destinationID, true, nil
}

type portalAuditLogWebhookInfo struct {
	portalID string
	webhook  *kkComps.PortalAuditLogWebhook
}

func (i *portalAuditLogWebhookInfo) GetID() string {
	if i == nil {
		return ""
	}
	return i.portalID
}

func (i *portalAuditLogWebhookInfo) GetName() string {
	if i == nil || i.webhook == nil || i.webhook.AuditLogDestinationID == nil {
		return ""
	}
	return *i.webhook.AuditLogDestinationID
}

func (i *portalAuditLogWebhookInfo) GetLabels() map[string]string {
	return nil
}

func (i *portalAuditLogWebhookInfo) GetNormalizedLabels() map[string]string {
	return nil
}
