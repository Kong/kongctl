package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// PortalSnippetAdapter implements ResourceOperations for portal snippets
type PortalSnippetAdapter struct {
	client *state.Client
}

// NewPortalSnippetAdapter creates a new portal snippet adapter
func NewPortalSnippetAdapter(client *state.Client) *PortalSnippetAdapter {
	return &PortalSnippetAdapter{client: client}
}

// MapCreateFields maps fields to CreatePortalSnippetRequest
func (p *PortalSnippetAdapter) MapCreateFields(_ context.Context, fields map[string]interface{},
	create *kkComps.CreatePortalSnippetRequest) error {
	// Required fields
	name, ok := fields["name"].(string)
	if !ok {
		return fmt.Errorf("name is required")
	}
	create.Name = name
	
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
		visibility := kkComps.SnippetVisibilityStatus(visibilityStr)
		create.Visibility = &visibility
	}
	
	if statusStr, ok := fields["status"].(string); ok {
		status := kkComps.PublishedStatus(statusStr)
		create.Status = &status
	}
	
	if description, ok := fields["description"].(string); ok {
		create.Description = &description
	}
	
	return nil
}

// MapUpdateFields maps fields to UpdatePortalSnippetRequest
func (p *PortalSnippetAdapter) MapUpdateFields(_ context.Context, fields map[string]interface{},
	update *kkComps.UpdatePortalSnippetRequest, _ map[string]string) error {
	// Optional fields
	if name, ok := fields["name"].(string); ok {
		update.Name = &name
	}
	
	if content, ok := fields["content"].(string); ok {
		update.Content = &content
	}
	
	if title, ok := fields["title"].(string); ok {
		update.Title = &title
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
	
	return nil
}

// Create creates a new portal snippet
func (p *PortalSnippetAdapter) Create(ctx context.Context, req kkComps.CreatePortalSnippetRequest,
	_ string) (string, error) {
	// Get portal ID from context
	portalID, err := p.getPortalID(ctx)
	if err != nil {
		return "", err
	}
	
	return p.client.CreatePortalSnippet(ctx, portalID, req)
}

// Update updates an existing portal snippet
func (p *PortalSnippetAdapter) Update(ctx context.Context, id string, req kkComps.UpdatePortalSnippetRequest,
	_ string) (string, error) {
	// Get portal ID from context
	portalID, err := p.getPortalID(ctx)
	if err != nil {
		return "", err
	}
	
	err = p.client.UpdatePortalSnippet(ctx, portalID, id, req)
	if err != nil {
		return "", err
	}
	return id, nil
}

// Delete deletes a portal snippet
func (p *PortalSnippetAdapter) Delete(ctx context.Context, id string) error {
	// Get portal ID from context
	portalID, err := p.getPortalID(ctx)
	if err != nil {
		return err
	}
	
	return p.client.DeletePortalSnippet(ctx, portalID, id)
}

// GetByName gets a portal snippet by name
func (p *PortalSnippetAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	// Portal snippets are looked up by the planner from the list
	// No direct "get by name" API available
	return nil, nil
}

// GetByID gets a portal snippet by ID
func (p *PortalSnippetAdapter) GetByID(ctx context.Context, id string) (ResourceInfo, error) {
	// Get portal ID from context
	portalID, err := p.getPortalID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get portal ID for snippet lookup: %w", err)
	}
	
	snippet, err := p.client.GetPortalSnippet(ctx, portalID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get portal snippet: %w", err)
	}
	
	if snippet == nil {
		return nil, nil
	}
	
	return &PortalSnippetResourceInfo{snippet: snippet}, nil
}

// ResourceType returns the resource type name
func (p *PortalSnippetAdapter) ResourceType() string {
	return "portal_snippet"
}

// RequiredFields returns the required fields for creation
func (p *PortalSnippetAdapter) RequiredFields() []string {
	return []string{"name", "content"}
}

// SupportsUpdate returns true as snippets support updates
func (p *PortalSnippetAdapter) SupportsUpdate() bool {
	return true
}

// getPortalID extracts the portal ID from the context
func (p *PortalSnippetAdapter) getPortalID(ctx context.Context) (string, error) {
	// Get the planned change from context
	change, ok := ctx.Value(contextKeyPlannedChange).(planner.PlannedChange)
	if !ok {
		return "", fmt.Errorf("planned change not found in context")
	}
	
	// Get portal ID from references
	if portalRef, ok := change.References["portal_id"]; ok {
		if portalRef.ID != "" {
			return portalRef.ID, nil
		}
	}
	
	return "", fmt.Errorf("portal ID is required for snippet operations")
}

// PortalSnippetResourceInfo implements ResourceInfo for portal snippets
type PortalSnippetResourceInfo struct {
	snippet *state.PortalSnippet
}

func (p *PortalSnippetResourceInfo) GetID() string {
	return p.snippet.ID
}

func (p *PortalSnippetResourceInfo) GetName() string {
	return p.snippet.Name
}

func (p *PortalSnippetResourceInfo) GetLabels() map[string]string {
	// Portal snippets don't support labels in the SDK
	return p.snippet.NormalizedLabels
}

func (p *PortalSnippetResourceInfo) GetNormalizedLabels() map[string]string {
	return p.snippet.NormalizedLabels
}