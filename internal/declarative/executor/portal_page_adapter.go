package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// PortalPageAdapter implements ResourceOperations for portal pages
type PortalPageAdapter struct {
	client *state.Client
}

// NewPortalPageAdapter creates a new portal page adapter
func NewPortalPageAdapter(client *state.Client) *PortalPageAdapter {
	return &PortalPageAdapter{client: client}
}

// MapCreateFields maps fields to CreatePortalPageRequest
func (p *PortalPageAdapter) MapCreateFields(_ context.Context, execCtx *ExecutionContext, fields map[string]any,
	create *kkComps.CreatePortalPageRequest) error {
	
	// Required fields
	slug, ok := fields["slug"].(string)
	if !ok {
		return fmt.Errorf("slug is required")
	}
	create.Slug = slug
	
	content, ok := fields["content"].(string)
	if !ok {
		return fmt.Errorf("content is required")
	}
	create.Content = content
	
	// Optional fields
	if title, ok := fields["title"].(string); ok {
		create.Title = &title
	}
	
	if visibilityStr, ok := fields["visibility"].(string); ok {
		visibility := kkComps.PageVisibilityStatus(visibilityStr)
		create.Visibility = &visibility
	}
	
	if statusStr, ok := fields["status"].(string); ok {
		status := kkComps.PublishedStatus(statusStr)
		create.Status = &status
	}
	
	if description, ok := fields["description"].(string); ok {
		create.Description = &description
	}
	
	// Handle parent page reference
	change := *execCtx.PlannedChange
	if parentPageRef, ok := change.References["parent_page_id"]; ok {
		if parentPageRef.ID != "" {
			create.ParentPageID = &parentPageRef.ID
		}
		// Parent page resolution will be handled by the executor if ID is empty
	} else if parentPageID, ok := fields["parent_page_id"].(string); ok {
		create.ParentPageID = &parentPageID
	}
	
	return nil
}

// MapUpdateFields maps fields to UpdatePortalPageRequest
func (p *PortalPageAdapter) MapUpdateFields(_ context.Context, execCtx *ExecutionContext, fields map[string]any,
	update *kkComps.UpdatePortalPageRequest, _ map[string]string) error {
	// Optional fields
	if slug, ok := fields["slug"].(string); ok {
		update.Slug = &slug
	}
	
	if title, ok := fields["title"].(string); ok {
		update.Title = &title
	}
	
	if content, ok := fields["content"].(string); ok {
		update.Content = &content
	}
	
	if visibilityStr, ok := fields["visibility"].(string); ok {
		visibility := kkComps.VisibilityStatus(visibilityStr)
		update.Visibility = &visibility
	}
	
	if statusStr, ok := fields["status"].(string); ok {
		status := kkComps.PublishedStatus(statusStr)
		update.Status = &status
	}
	
	if description, ok := fields["description"].(string); ok {
		update.Description = &description
	}
	
	// Handle parent page reference
	change := *execCtx.PlannedChange
	if parentPageRef, ok := change.References["parent_page_id"]; ok {
		if parentPageRef.ID != "" {
			update.ParentPageID = &parentPageRef.ID
		}
		// Parent page resolution will be handled by the executor if needed
	} else if parentPageID, ok := fields["parent_page_id"].(string); ok {
		update.ParentPageID = &parentPageID
	}
	
	return nil
}

// Create creates a new portal page
func (p *PortalPageAdapter) Create(ctx context.Context, req kkComps.CreatePortalPageRequest,
	_ string, execCtx *ExecutionContext) (string, error) {
	// Get portal ID from execution context
	portalID, err := p.getPortalIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	
	return p.client.CreatePortalPage(ctx, portalID, req)
}

// Update updates an existing portal page
func (p *PortalPageAdapter) Update(ctx context.Context, id string, req kkComps.UpdatePortalPageRequest,
	_ string, execCtx *ExecutionContext) (string, error) {
	// Get portal ID from execution context
	portalID, err := p.getPortalIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	
	err = p.client.UpdatePortalPage(ctx, portalID, id, req)
	if err != nil {
		return "", err
	}
	return id, nil
}

// Delete deletes a portal page
func (p *PortalPageAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	// Get portal ID from execution context
	portalID, err := p.getPortalIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	
	return p.client.DeletePortalPage(ctx, portalID, id)
}

// GetByName gets a portal page by slug
func (p *PortalPageAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	// Portal pages don't have a direct "get by name" method
	// The planner handles this by searching through the list
	return nil, nil
}

// ResourceType returns the resource type name
func (p *PortalPageAdapter) ResourceType() string {
	return "portal_page"
}

// RequiredFields returns the required fields for creation
func (p *PortalPageAdapter) RequiredFields() []string {
	return []string{"slug", "content"}
}

// SupportsUpdate returns true as pages support updates
func (p *PortalPageAdapter) SupportsUpdate() bool {
	return true
}


// getPortalIDFromExecutionContext extracts the portal ID from ExecutionContext parameter (used for Delete operations)
func (p *PortalPageAdapter) getPortalIDFromExecutionContext(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for page operations")
	}
	
	change := *execCtx.PlannedChange
	
	// Priority 1: Check References (for Create operations)
	if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID != "" {
		return portalRef.ID, nil
	}
	
	// Priority 2: Check Parent field (for Delete operations)
	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}
	
	return "", fmt.Errorf("portal ID is required for page operations")
}

// GetByID gets a portal page by ID using portal context
func (p *PortalPageAdapter) GetByID(ctx context.Context, id string, execCtx *ExecutionContext) (ResourceInfo, error) {
	portalID, err := p.getPortalIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get portal ID for page lookup: %w", err)
	}

	page, err := p.client.GetPortalPage(ctx, portalID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get portal page: %w", err)
	}
	if page == nil {
		return nil, nil
	}
	return &PortalPageResourceInfo{page: page}, nil
}

// PortalPageResourceInfo implements ResourceInfo for portal pages
type PortalPageResourceInfo struct {
	page *state.PortalPage
}

func (p *PortalPageResourceInfo) GetID() string {
	return p.page.ID
}

func (p *PortalPageResourceInfo) GetName() string {
	return p.page.Slug
}

func (p *PortalPageResourceInfo) GetLabels() map[string]string {
	// Portal pages don't support labels in the SDK
	return p.page.NormalizedLabels
}

func (p *PortalPageResourceInfo) GetNormalizedLabels() map[string]string {
	return p.page.NormalizedLabels
}