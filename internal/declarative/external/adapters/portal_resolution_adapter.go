package adapters

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/state"
)

// PortalResolutionAdapter handles portal resource resolution
type PortalResolutionAdapter struct {
	*BaseAdapter
}

// NewPortalResolutionAdapter creates a new portal resolution adapter
func NewPortalResolutionAdapter(client *state.Client) *PortalResolutionAdapter {
	return &PortalResolutionAdapter{
		BaseAdapter: NewBaseAdapter(client),
	}
}

// GetByID retrieves a portal by ID
func (p *PortalResolutionAdapter) GetByID(
	ctx context.Context, id string, parent *external.ResolvedParent,
) (interface{}, error) {
	if parent != nil {
		return nil, fmt.Errorf("portal is a top-level resource and cannot have a parent")
	}
	
	portal, err := p.GetClient().GetPortalByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve portal by ID %s: %w", id, err)
	}
	
	return portal, nil
}

// GetBySelector retrieves portals by selector fields
func (p *PortalResolutionAdapter) GetBySelector(
	ctx context.Context, selector map[string]string, parent *external.ResolvedParent,
) ([]interface{}, error) {
	if parent != nil {
		return nil, fmt.Errorf("portal is a top-level resource and cannot have a parent")
	}
	
	portals, err := p.GetClient().ListPortalsWithFilter(ctx, selector)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve portals by selector %v: %w", selector, err)
	}
	
	// FilterBySelector expects exactly one match and returns it
	portal, err := p.FilterBySelector(portals, selector, func(resource interface{}, field string) string {
		p := resource.(*state.Portal)
		switch field {
		case "name":
			return p.Name
		case "description":
			if p.Description != nil {
				return *p.Description
			}
			return ""
		default:
			return ""
		}
	})
	
	if err != nil {
		return nil, err
	}
	
	return []interface{}{portal}, nil
}