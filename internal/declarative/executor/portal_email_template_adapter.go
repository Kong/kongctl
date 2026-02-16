package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/state"
)

// PortalEmailTemplateAdapter implements ResourceOperations for portal email templates.
type PortalEmailTemplateAdapter struct {
	client *state.Client
}

// NewPortalEmailTemplateAdapter constructs a new adapter.
func NewPortalEmailTemplateAdapter(client *state.Client) *PortalEmailTemplateAdapter {
	return &PortalEmailTemplateAdapter{client: client}
}

func (a *PortalEmailTemplateAdapter) MapCreateFields(
	_ context.Context, _ *ExecutionContext, fields map[string]any, req *kkOps.UpdatePortalCustomEmailTemplateRequest,
) error {
	return mapPortalEmailTemplateFields(fields, req)
}

func (a *PortalEmailTemplateAdapter) MapUpdateFields(
	_ context.Context, _ *ExecutionContext, fields map[string]any, req *kkOps.UpdatePortalCustomEmailTemplateRequest,
	_ map[string]string,
) error {
	return mapPortalEmailTemplateFields(fields, req)
}

func (a *PortalEmailTemplateAdapter) Create(
	ctx context.Context, req kkOps.UpdatePortalCustomEmailTemplateRequest, _ string, execCtx *ExecutionContext,
) (string, error) {
	portalID, err := a.portalID(execCtx)
	if err != nil {
		return "", err
	}
	req.PortalID = portalID
	return a.callUpdate(ctx, req)
}

func (a *PortalEmailTemplateAdapter) Update(
	ctx context.Context,
	_ string,
	req kkOps.UpdatePortalCustomEmailTemplateRequest,
	_ string,
	execCtx *ExecutionContext,
) (string, error) {
	portalID, err := a.portalID(execCtx)
	if err != nil {
		return "", err
	}
	req.PortalID = portalID
	return a.callUpdate(ctx, req)
}

func (a *PortalEmailTemplateAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	portalID, err := a.portalID(execCtx)
	if err != nil {
		return err
	}
	templateName := kkComps.EmailTemplateName(id)
	return a.client.DeletePortalEmailTemplate(ctx, portalID, templateName)
}

func (a *PortalEmailTemplateAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	// Portal email templates are scoped to a portal; lookup by name alone is ambiguous.
	return nil, nil
}

func (a *PortalEmailTemplateAdapter) GetByID(
	ctx context.Context, id string, execCtx *ExecutionContext,
) (ResourceInfo, error) {
	portalID, err := a.portalID(execCtx)
	if err != nil {
		return nil, err
	}

	templateName := kkComps.EmailTemplateName(id)
	tpl, err := a.client.GetPortalCustomEmailTemplate(ctx, portalID, templateName)
	if err != nil {
		return nil, err
	}
	if tpl == nil {
		return nil, nil
	}

	return &portalEmailTemplateInfo{tpl: tpl}, nil
}

func (a *PortalEmailTemplateAdapter) ResourceType() string {
	return "portal_email_template"
}

func (a *PortalEmailTemplateAdapter) RequiredFields() []string {
	return []string{"name"}
}

func (a *PortalEmailTemplateAdapter) SupportsUpdate() bool {
	return true
}

func (a *PortalEmailTemplateAdapter) portalID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for portal email template operations")
	}

	change := *execCtx.PlannedChange

	if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID != "" {
		return portalRef.ID, nil
	}

	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("portal ID is required for portal email template operations")
}

func (a *PortalEmailTemplateAdapter) callUpdate(
	ctx context.Context, req kkOps.UpdatePortalCustomEmailTemplateRequest,
) (string, error) {
	if req.TemplateName == "" {
		return "", fmt.Errorf("template name is required")
	}
	id, err := a.client.UpdatePortalEmailTemplate(ctx, req.PortalID, req.TemplateName,
		req.PatchCustomPortalEmailTemplatePayload)
	if err != nil {
		return "", err
	}
	if id == "" {
		id = string(req.TemplateName)
	}
	return id, nil
}

func mapPortalEmailTemplateFields(
	fields map[string]any,
	req *kkOps.UpdatePortalCustomEmailTemplateRequest,
) error {
	nameVal, ok := fields["name"]
	if !ok {
		return fmt.Errorf("name is required for portal email template")
	}
	switch v := nameVal.(type) {
	case string:
		req.TemplateName = kkComps.EmailTemplateName(v)
	case kkComps.EmailTemplateName:
		req.TemplateName = v
	default:
		return fmt.Errorf("name must be a string")
	}

	if enabledVal, ok := fields["enabled"]; ok {
		switch v := enabledVal.(type) {
		case bool:
			req.PatchCustomPortalEmailTemplatePayload.Enabled = &v
		case *bool:
			if v != nil {
				req.PatchCustomPortalEmailTemplatePayload.Enabled = v
			}
		default:
			return fmt.Errorf("enabled must be a boolean")
		}
	}

	if contentVal, ok := fields["content"]; ok {
		if contentVal == nil {
			req.PatchCustomPortalEmailTemplatePayload.Content = nil
		} else {
			contentMap, ok := contentVal.(map[string]any)
			if !ok {
				return fmt.Errorf("content must be a map")
			}
			content := kkComps.EmailTemplateContent{}

			if v, ok := contentMap["subject"]; ok {
				subject, err := parseOptionalString(v)
				if err != nil {
					return fmt.Errorf("invalid subject: %w", err)
				}
				content.Subject = subject
			}

			if v, ok := contentMap["title"]; ok {
				title, err := parseOptionalString(v)
				if err != nil {
					return fmt.Errorf("invalid title: %w", err)
				}
				content.Title = title
			}

			if v, ok := contentMap["body"]; ok {
				body, err := parseOptionalString(v)
				if err != nil {
					return fmt.Errorf("invalid body: %w", err)
				}
				content.Body = body
			}

			if v, ok := contentMap["button_label"]; ok {
				label, err := parseOptionalString(v)
				if err != nil {
					return fmt.Errorf("invalid button_label: %w", err)
				}
				content.ButtonLabel = label
			}

			req.PatchCustomPortalEmailTemplatePayload.Content = &content
		}
	}

	return nil
}

func parseOptionalString(value any) (*string, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case string:
		return &v, nil
	case *string:
		return v, nil
	default:
		return nil, fmt.Errorf("must be a string or null")
	}
}

type portalEmailTemplateInfo struct {
	tpl *state.PortalEmailTemplate
}

func (i *portalEmailTemplateInfo) GetID() string {
	if i == nil || i.tpl == nil {
		return ""
	}
	return i.tpl.Name
}

func (i *portalEmailTemplateInfo) GetName() string {
	if i == nil || i.tpl == nil {
		return ""
	}
	return i.tpl.Name
}

func (i *portalEmailTemplateInfo) GetLabels() map[string]string {
	return nil
}

func (i *portalEmailTemplateInfo) GetNormalizedLabels() map[string]string {
	return nil
}
