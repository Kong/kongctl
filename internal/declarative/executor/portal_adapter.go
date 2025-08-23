package executor

import (
	"context"

	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// PortalAdapter implements ResourceOperations for portals
type PortalAdapter struct {
	client *state.Client
}

// NewPortalAdapter creates a new portal adapter
func NewPortalAdapter(client *state.Client) *PortalAdapter {
	return &PortalAdapter{client: client}
}

// MapCreateFields maps fields to CreatePortal request
func (p *PortalAdapter) MapCreateFields(_ context.Context, execCtx *ExecutionContext, fields map[string]any,
	create *kkComps.CreatePortal) error {
	// Extract namespace and protection from execution context
	namespace := execCtx.Namespace
	protection := execCtx.Protection

	// Map required fields
	create.Name = common.ExtractResourceName(fields)

	// Map optional fields using utilities (SDK uses double pointers)
	common.MapOptionalStringFieldToPtr(&create.Description, fields, "description")
	common.MapOptionalStringFieldToPtr(&create.DisplayName, fields, "display_name")
	common.MapOptionalBoolFieldToPtr(&create.AuthenticationEnabled, fields, "authentication_enabled")
	common.MapOptionalBoolFieldToPtr(&create.RbacEnabled, fields, "rbac_enabled")
	common.MapOptionalBoolFieldToPtr(&create.AutoApproveDevelopers, fields, "auto_approve_developers")
	common.MapOptionalBoolFieldToPtr(&create.AutoApproveApplications, fields, "auto_approve_applications")

	if defaultAPIVisibility, ok := fields["default_api_visibility"].(string); ok {
		visibility := kkComps.DefaultAPIVisibility(defaultAPIVisibility)
		create.DefaultAPIVisibility = &visibility
	}

	if defaultPageVisibility, ok := fields["default_page_visibility"].(string); ok {
		visibility := kkComps.DefaultPageVisibility(defaultPageVisibility)
		create.DefaultPageVisibility = &visibility
	}

	if defaultAppAuthStrategyID, ok := fields["default_application_auth_strategy_id"].(string); ok {
		create.DefaultApplicationAuthStrategyID = &defaultAppAuthStrategyID
	}

	// Handle labels using centralized helper
	userLabels := labels.ExtractLabelsFromField(fields["labels"])
	portalLabels := labels.BuildCreateLabels(userLabels, namespace, protection)

	// Convert to pointer map since portals use map[string]*string
	create.Labels = labels.ConvertStringMapToPointerMap(portalLabels)

	return nil
}

// MapUpdateFields maps fields to UpdatePortal request
func (p *PortalAdapter) MapUpdateFields(_ context.Context, execCtx *ExecutionContext, fields map[string]any,
	update *kkComps.UpdatePortal, currentLabels map[string]string) error {
	// Extract namespace and protection from execution context
	namespace := execCtx.Namespace
	protection := execCtx.Protection

	// Only include fields that are in the fields map
	// These represent actual changes detected by the planner
	for field, value := range fields {
		switch field {
		case "name":
			if name, ok := value.(string); ok {
				update.Name = &name
			}
		case "description":
			if desc, ok := value.(string); ok {
				update.Description = &desc
			}
		case "display_name":
			if displayName, ok := value.(string); ok {
				update.DisplayName = &displayName
			}
		case "authentication_enabled":
			if authEnabled, ok := value.(bool); ok {
				update.AuthenticationEnabled = &authEnabled
			}
		case "rbac_enabled":
			if rbacEnabled, ok := value.(bool); ok {
				update.RbacEnabled = &rbacEnabled
			}
		case "auto_approve_developers":
			if autoApproveDevelopers, ok := value.(bool); ok {
				update.AutoApproveDevelopers = &autoApproveDevelopers
			}
		case "auto_approve_applications":
			if autoApproveApplications, ok := value.(bool); ok {
				update.AutoApproveApplications = &autoApproveApplications
			}
		case "default_application_auth_strategy_id":
			if authID, ok := value.(string); ok {
				update.DefaultApplicationAuthStrategyID = &authID
			}
		case "default_api_visibility":
			if visibility, ok := value.(string); ok {
				vis := kkComps.UpdatePortalDefaultAPIVisibility(visibility)
				update.DefaultAPIVisibility = &vis
			}
		case "default_page_visibility":
			if visibility, ok := value.(string); ok {
				vis := kkComps.UpdatePortalDefaultPageVisibility(visibility)
				update.DefaultPageVisibility = &vis
			}
		// Skip "labels" as they're handled separately below
		}
	}

	// Handle labels using centralized helper
	desiredLabels := labels.ExtractLabelsFromField(fields["labels"])
	if desiredLabels != nil {
		// Get current labels if passed from planner
		plannerCurrentLabels := labels.ExtractLabelsFromField(fields[planner.FieldCurrentLabels])
		if plannerCurrentLabels != nil {
			currentLabels = plannerCurrentLabels
		}

		// Build update labels with removal support
		update.Labels = labels.BuildUpdateLabels(desiredLabels, currentLabels, namespace, protection)
	} else if currentLabels != nil {
		// If no labels in change, preserve existing labels with updated protection
		update.Labels = labels.BuildUpdateLabels(currentLabels, currentLabels, namespace, protection)
	}

	return nil
}

// Create creates a new portal
func (p *PortalAdapter) Create(ctx context.Context, req kkComps.CreatePortal, 
	namespace string, _ *ExecutionContext) (string, error) {
	resp, err := p.client.CreatePortal(ctx, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Update updates an existing portal
func (p *PortalAdapter) Update(ctx context.Context, id string, req kkComps.UpdatePortal,
	namespace string, _ *ExecutionContext) (string, error) {
	resp, err := p.client.UpdatePortal(ctx, id, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Delete deletes a portal
func (p *PortalAdapter) Delete(ctx context.Context, id string, _ *ExecutionContext) error {
	// Delete the portal with force=true to cascade delete child resources
	return p.client.DeletePortal(ctx, id, true)
}

// GetByName gets a portal by name
func (p *PortalAdapter) GetByName(ctx context.Context, name string) (ResourceInfo, error) {
	portal, err := p.client.GetPortalByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if portal == nil {
		return nil, nil
	}
	return &PortalResourceInfo{portal: portal}, nil
}

// GetByID gets a portal by ID
func (p *PortalAdapter) GetByID(_ context.Context, _ string, _ *ExecutionContext) (ResourceInfo, error) {
	// Portals don't have a direct GetByID method - the planner handles ID lookups
	return nil, nil
}

// ResourceType returns the resource type name
func (p *PortalAdapter) ResourceType() string {
	return "portal"
}

// RequiredFields returns the required fields for creation
func (p *PortalAdapter) RequiredFields() []string {
	return []string{"name"}
}

// SupportsUpdate returns true as portals support updates
func (p *PortalAdapter) SupportsUpdate() bool {
	return true
}

// PortalResourceInfo wraps a Portal to implement ResourceInfo
type PortalResourceInfo struct {
	portal *state.Portal
}

func (p *PortalResourceInfo) GetID() string {
	return p.portal.ID
}

func (p *PortalResourceInfo) GetName() string {
	return p.portal.Name
}

func (p *PortalResourceInfo) GetLabels() map[string]string {
	// Portal.Labels is already map[string]string from the SDK
	return p.portal.Labels
}

func (p *PortalResourceInfo) GetNormalizedLabels() map[string]string {
	return p.portal.NormalizedLabels
}