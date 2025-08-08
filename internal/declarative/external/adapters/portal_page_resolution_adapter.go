package adapters

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/state"
)

// PortalPageResolutionAdapter handles portal page resource resolution
type PortalPageResolutionAdapter struct {
	*BaseAdapter
}

// NewPortalPageResolutionAdapter creates a new portal page resolution adapter
func NewPortalPageResolutionAdapter(client *state.Client) *PortalPageResolutionAdapter {
	return &PortalPageResolutionAdapter{
		BaseAdapter: NewBaseAdapter(client),
	}
}

// GetByID retrieves a portal page by ID with parent context
func (p *PortalPageResolutionAdapter) GetByID(
	ctx context.Context, id string, parent *external.ResolvedParent,
) (interface{}, error) {
	if err := p.ValidateParentContext(parent, "portal"); err != nil {
		return nil, err
	}
	
	page, err := p.GetClient().GetPortalPage(ctx, parent.ID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve portal page %s for portal %s: %w", id, parent.ID, err)
	}
	
	return page, nil
}

// GetBySelector retrieves portal pages by selector fields with parent context
func (p *PortalPageResolutionAdapter) GetBySelector(
	ctx context.Context, selector map[string]string, parent *external.ResolvedParent,
) ([]interface{}, error) {
	if err := p.ValidateParentContext(parent, "portal"); err != nil {
		return nil, err
	}
	
	pages, err := p.GetClient().ListPortalPagesWithFilter(ctx, parent.ID, selector)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve portal pages for portal %s: %w", parent.ID, err)
	}
	
	// FilterBySelector expects exactly one match and returns it
	page, err := p.FilterBySelector(pages, selector, func(resource interface{}, field string) string {
		pg := resource.(*state.PortalPage)
		switch field {
		case "slug":
			return pg.Slug
		case "title":
			return pg.Title
		default:
			return ""
		}
	})
	
	if err != nil {
		return nil, err
	}
	
	return []interface{}{page}, nil
}