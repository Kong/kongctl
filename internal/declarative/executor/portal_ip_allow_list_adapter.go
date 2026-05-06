package executor

import (
	"context"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// PortalIPAllowListAdapter implements ResourceOperations for portal IP allow lists.
type PortalIPAllowListAdapter struct {
	client *state.Client
}

// NewPortalIPAllowListAdapter creates a new portal IP allow list adapter.
func NewPortalIPAllowListAdapter(client *state.Client) *PortalIPAllowListAdapter {
	return &PortalIPAllowListAdapter{client: client}
}

func (p *PortalIPAllowListAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreatePortalSourceIPRestriction,
) error {
	allowedIPs, err := portalIPAllowListAllowedIPs(fields)
	if err != nil {
		return err
	}
	create.AllowedIps = allowedIPs
	return nil
}

func (p *PortalIPAllowListAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdatePortalSourceIPRestriction,
	_ map[string]string,
) error {
	allowedIPs, err := portalIPAllowListAllowedIPs(fields)
	if err != nil {
		return err
	}
	update.AllowedIps = allowedIPs
	return nil
}

func (p *PortalIPAllowListAdapter) Create(
	ctx context.Context,
	req kkComps.CreatePortalSourceIPRestriction,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	portalID, err := p.portalIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return p.client.CreatePortalIPAllowList(ctx, portalID, req, namespace)
}

func (p *PortalIPAllowListAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdatePortalSourceIPRestriction,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	portalID, err := p.portalIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	return p.client.UpdatePortalIPAllowList(ctx, portalID, id, req, namespace)
}

func (p *PortalIPAllowListAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	portalID, err := p.portalIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	return p.client.DeletePortalIPAllowList(ctx, portalID, id)
}

func (p *PortalIPAllowListAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, nil
}

func (p *PortalIPAllowListAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	if id == "" {
		return nil, nil
	}

	portalID, err := p.portalIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}
	entry, err := p.client.GetPortalIPAllowList(ctx, portalID, id)
	if err != nil || entry == nil {
		return nil, err
	}

	return &PortalIPAllowListResourceInfo{
		id:         entry.ID,
		allowedIPs: entry.AllowedIPs,
	}, nil
}

func (p *PortalIPAllowListAdapter) ResourceType() string {
	return planner.ResourceTypePortalIPAllowList
}

func (p *PortalIPAllowListAdapter) RequiredFields() []string {
	return []string{planner.FieldAllowedIPs}
}

func (p *PortalIPAllowListAdapter) SupportsUpdate() bool {
	return true
}

func (p *PortalIPAllowListAdapter) portalIDFromExecutionContext(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for portal IP allow list operations")
	}

	change := *execCtx.PlannedChange
	if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID != "" {
		return portalRef.ID, nil
	}
	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("portal ID is required for portal IP allow list operations")
}

func portalIPAllowListAllowedIPs(fields map[string]any) ([]string, error) {
	raw, ok := fields[planner.FieldAllowedIPs]
	if !ok {
		return nil, fmt.Errorf("allowed_ips is required")
	}

	var values []string
	switch typed := raw.(type) {
	case []string:
		values = append(values, typed...)
	case []any:
		values = make([]string, 0, len(typed))
		for i, item := range typed {
			value, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("allowed_ips[%d] must be a string", i)
			}
			values = append(values, value)
		}
	default:
		return nil, fmt.Errorf("allowed_ips must be a list of strings")
	}

	normalized := make([]string, 0, len(values))
	for i, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, fmt.Errorf("allowed_ips[%d] cannot be empty", i)
		}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("allowed_ips must contain at least one IP address or CIDR block")
	}

	return normalized, nil
}

// PortalIPAllowListResourceInfo implements ResourceInfo for portal IP allow lists.
type PortalIPAllowListResourceInfo struct {
	id         string
	allowedIPs []string
}

func (p *PortalIPAllowListResourceInfo) GetID() string {
	return p.id
}

func (p *PortalIPAllowListResourceInfo) GetName() string {
	return strings.Join(p.allowedIPs, ",")
}

func (p *PortalIPAllowListResourceInfo) GetLabels() map[string]string {
	return make(map[string]string)
}

func (p *PortalIPAllowListResourceInfo) GetNormalizedLabels() map[string]string {
	return make(map[string]string)
}
