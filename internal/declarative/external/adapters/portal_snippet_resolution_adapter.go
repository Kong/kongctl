package adapters

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/state"
)

// PortalSnippetResolutionAdapter handles portal snippet resource resolution
type PortalSnippetResolutionAdapter struct {
	*BaseAdapter
}

// NewPortalSnippetResolutionAdapter creates a new portal snippet resolution adapter
func NewPortalSnippetResolutionAdapter(client *state.Client) *PortalSnippetResolutionAdapter {
	return &PortalSnippetResolutionAdapter{
		BaseAdapter: NewBaseAdapter(client),
	}
}

// GetByID retrieves a portal snippet by ID with parent context
func (p *PortalSnippetResolutionAdapter) GetByID(
	ctx context.Context, id string, parent *external.ResolvedParent,
) (interface{}, error) {
	if err := p.ValidateParentContext(parent, "portal"); err != nil {
		return nil, err
	}
	
	snippet, err := p.GetClient().GetPortalSnippet(ctx, parent.ID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve portal snippet %s for portal %s: %w", id, parent.ID, err)
	}
	
	return snippet, nil
}

// GetBySelector retrieves portal snippets by selector fields with parent context
func (p *PortalSnippetResolutionAdapter) GetBySelector(
	ctx context.Context, selector map[string]string, parent *external.ResolvedParent,
) ([]interface{}, error) {
	if err := p.ValidateParentContext(parent, "portal"); err != nil {
		return nil, err
	}
	
	snippets, err := p.GetClient().ListPortalSnippetsWithFilter(ctx, parent.ID, selector)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve portal snippets for portal %s: %w", parent.ID, err)
	}
	
	// FilterBySelector expects exactly one match and returns it
	snippet, err := p.FilterBySelector(snippets, selector, func(resource interface{}, field string) string {
		sn := resource.(*state.PortalSnippet)
		switch field {
		case "name":
			return sn.Name
		case "title":
			return sn.Title
		default:
			return ""
		}
	})
	
	if err != nil {
		return nil, err
	}
	
	return []interface{}{snippet}, nil
}