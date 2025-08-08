package adapters

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/state"
)

// PortalCustomDomainResolutionAdapter handles portal custom domain resource resolution
type PortalCustomDomainResolutionAdapter struct {
	*BaseAdapter
}

// NewPortalCustomDomainResolutionAdapter creates a new portal custom domain resolution adapter
func NewPortalCustomDomainResolutionAdapter(client *state.Client) *PortalCustomDomainResolutionAdapter {
	return &PortalCustomDomainResolutionAdapter{
		BaseAdapter: NewBaseAdapter(client),
	}
}

// GetByID retrieves a portal custom domain by ID with parent context
func (p *PortalCustomDomainResolutionAdapter) GetByID(
	ctx context.Context, id string, parent *external.ResolvedParent,
) (interface{}, error) {
	if err := p.ValidateParentContext(parent, "portal"); err != nil {
		return nil, err
	}
	
	// Portal custom domain doesn't have its own ID, use parent portal ID
	customDomain, err := p.GetClient().GetPortalCustomDomainByID(ctx, parent.ID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve portal custom domain for portal %s: %w", parent.ID, err)
	}
	
	return customDomain, nil
}

// GetBySelector retrieves portal custom domains by selector fields with parent context
func (p *PortalCustomDomainResolutionAdapter) GetBySelector(
	ctx context.Context, selector map[string]string, parent *external.ResolvedParent,
) ([]interface{}, error) {
	if err := p.ValidateParentContext(parent, "portal"); err != nil {
		return nil, err
	}
	
	customDomains, err := p.GetClient().ListPortalCustomDomainsWithFilter(ctx, parent.ID, selector)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve portal custom domains for portal %s: %w", parent.ID, err)
	}
	
	// Portal custom domain is a singleton, so filtering doesn't apply
	// Just return the single custom domain if it exists
	return customDomains, nil
}