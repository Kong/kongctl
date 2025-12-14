package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/state"
)

// PortalEmailConfigAdapter implements ResourceOperations for portal email configs.
type PortalEmailConfigAdapter struct {
	client *state.Client
}

// NewPortalEmailConfigAdapter creates a new adapter.
func NewPortalEmailConfigAdapter(client *state.Client) *PortalEmailConfigAdapter {
	return &PortalEmailConfigAdapter{client: client}
}

func (a *PortalEmailConfigAdapter) MapCreateFields(
	_ context.Context, _ *ExecutionContext, fields map[string]any, create *kkComps.PostPortalEmailConfig,
) error {
	if domain, ok := fields["domain_name"].(string); ok {
		create.DomainName = &domain
	}

	fromName, ok := fields["from_name"].(string)
	if !ok || fromName == "" {
		return fmt.Errorf("from_name is required")
	}
	create.FromName = &fromName

	fromEmail, ok := fields["from_email"].(string)
	if !ok || fromEmail == "" {
		return fmt.Errorf("from_email is required")
	}
	create.FromEmail = &fromEmail

	replyTo, ok := fields["reply_to_email"].(string)
	if !ok || replyTo == "" {
		return fmt.Errorf("reply_to_email is required")
	}
	create.ReplyToEmail = &replyTo

	return nil
}

func (a *PortalEmailConfigAdapter) MapUpdateFields(
	_ context.Context, _ *ExecutionContext, fields map[string]any, update *kkComps.PatchPortalEmailConfig,
	_ map[string]string,
) error {
	if v, ok := fields["domain_name"]; ok {
		if v == nil {
			update.DomainName = nil
		} else if domain, ok := v.(string); ok {
			update.DomainName = &domain
		}
	}
	if v, ok := fields["from_name"]; ok {
		if v == nil {
			update.FromName = nil
		} else if fromName, ok := v.(string); ok {
			update.FromName = &fromName
		}
	}
	if v, ok := fields["from_email"]; ok {
		if v == nil {
			update.FromEmail = nil
		} else if fromEmail, ok := v.(string); ok {
			update.FromEmail = &fromEmail
		}
	}
	if v, ok := fields["reply_to_email"]; ok {
		if v == nil {
			update.ReplyToEmail = nil
		} else if replyTo, ok := v.(string); ok {
			update.ReplyToEmail = &replyTo
		}
	}
	return nil
}

func (a *PortalEmailConfigAdapter) Create(
	ctx context.Context, req kkComps.PostPortalEmailConfig, _ string, execCtx *ExecutionContext,
) (string, error) {
	portalID, err := a.portalID(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreatePortalEmailConfig(ctx, portalID, req)
}

func (a *PortalEmailConfigAdapter) Update(
	ctx context.Context, id string, req kkComps.PatchPortalEmailConfig, _ string, execCtx *ExecutionContext,
) (string, error) {
	portalID, err := a.portalID(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdatePortalEmailConfig(ctx, portalID, &req)
}

func (a *PortalEmailConfigAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	portalID, err := a.portalID(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeletePortalEmailConfig(ctx, portalID)
}

func (a *PortalEmailConfigAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, nil
}

func (a *PortalEmailConfigAdapter) GetByID(_ context.Context, _ string, _ *ExecutionContext) (ResourceInfo, error) {
	return nil, nil
}

func (a *PortalEmailConfigAdapter) ResourceType() string {
	return "portal_email_config"
}

func (a *PortalEmailConfigAdapter) RequiredFields() []string {
	return []string{"from_name", "from_email", "reply_to_email"}
}

func (a *PortalEmailConfigAdapter) SupportsUpdate() bool {
	return true
}

func (a *PortalEmailConfigAdapter) portalID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for portal email config operations")
	}

	change := *execCtx.PlannedChange

	if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID != "" {
		return portalRef.ID, nil
	}

	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("portal ID is required for portal email config operations")
}
