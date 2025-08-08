package adapters

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/state"
)

// CEServiceResolutionAdapter handles core entity service resource resolution
// This adapter is for Kong Gateway services within a control plane,
// not to be confused with Service Catalog services
type CEServiceResolutionAdapter struct {
	*BaseAdapter
}

// NewCEServiceResolutionAdapter creates a new core entity service resolution adapter
func NewCEServiceResolutionAdapter(client *state.Client) *CEServiceResolutionAdapter {
	return &CEServiceResolutionAdapter{
		BaseAdapter: NewBaseAdapter(client),
	}
}

// GetByID retrieves a core entity service by ID with parent context
func (c *CEServiceResolutionAdapter) GetByID(
	ctx context.Context, id string, parent *external.ResolvedParent,
) (interface{}, error) {
	if err := c.ValidateParentContext(parent, "control_plane"); err != nil {
		return nil, err
	}
	
	service, err := c.GetClient().GetCoreEntityServiceByID(ctx, parent.ID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve core entity service by ID %s in control plane %s: %w",
			id, parent.ID, err)
	}
	
	return service, nil
}

// GetBySelector retrieves core entity services by selector fields with parent context
func (c *CEServiceResolutionAdapter) GetBySelector(
	ctx context.Context, selector map[string]string, parent *external.ResolvedParent,
) ([]interface{}, error) {
	if err := c.ValidateParentContext(parent, "control_plane"); err != nil {
		return nil, err
	}
	
	services, err := c.GetClient().ListCoreEntityServicesWithFilter(ctx, parent.ID, selector)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve core entity services by selector %v in control plane %s: %w",
			selector, parent.ID, err)
	}
	
	// FilterBySelector expects exactly one match and returns it
	service, err := c.FilterBySelector(services, selector, func(resource interface{}, field string) string {
		svc := resource.(*state.CoreEntityService)
		switch field {
		case "name":
			return svc.Name
		default:
			return ""
		}
	})
	
	if err != nil {
		return nil, err
	}
	
	return []interface{}{service}, nil
}