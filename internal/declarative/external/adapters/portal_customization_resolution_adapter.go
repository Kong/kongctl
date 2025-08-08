package adapters

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/state"
)

// PortalCustomizationResolutionAdapter handles portal customization resource resolution
type PortalCustomizationResolutionAdapter struct {
	*BaseAdapter
}

// NewPortalCustomizationResolutionAdapter creates a new portal customization resolution adapter
func NewPortalCustomizationResolutionAdapter(client *state.Client) *PortalCustomizationResolutionAdapter {
	return &PortalCustomizationResolutionAdapter{
		BaseAdapter: NewBaseAdapter(client),
	}
}

// GetByID retrieves a portal customization by ID with parent context
func (p *PortalCustomizationResolutionAdapter) GetByID(
	ctx context.Context, id string, parent *external.ResolvedParent,
) (interface{}, error) {
	if err := p.ValidateParentContext(parent, "portal"); err != nil {
		return nil, err
	}
	
	// Portal customization doesn't have its own ID, use parent portal ID
	customization, err := p.GetClient().GetPortalCustomizationByID(ctx, parent.ID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve portal customization for portal %s: %w", parent.ID, err)
	}
	
	return customization, nil
}

// GetBySelector retrieves portal customizations by selector fields with parent context
func (p *PortalCustomizationResolutionAdapter) GetBySelector(
	ctx context.Context, selector map[string]string, parent *external.ResolvedParent,
) ([]interface{}, error) {
	if err := p.ValidateParentContext(parent, "portal"); err != nil {
		return nil, err
	}
	
	customizations, err := p.GetClient().ListPortalCustomizationsWithFilter(ctx, parent.ID, selector)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve portal customizations for portal %s: %w", parent.ID, err)
	}
	
	// Portal customization is a singleton, so filtering doesn't apply
	// Just return the single customization if it exists
	return customizations, nil
}