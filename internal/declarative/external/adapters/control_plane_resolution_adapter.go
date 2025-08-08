package adapters

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/state"
)

// ControlPlaneResolutionAdapter handles control plane resource resolution
type ControlPlaneResolutionAdapter struct {
	*BaseAdapter
}

// NewControlPlaneResolutionAdapter creates a new control plane resolution adapter
func NewControlPlaneResolutionAdapter(client *state.Client) *ControlPlaneResolutionAdapter {
	return &ControlPlaneResolutionAdapter{
		BaseAdapter: NewBaseAdapter(client),
	}
}

// GetByID retrieves a control plane by ID
func (c *ControlPlaneResolutionAdapter) GetByID(
	ctx context.Context, id string, parent *external.ResolvedParent,
) (interface{}, error) {
	if parent != nil {
		return nil, fmt.Errorf("control_plane is a top-level resource and cannot have a parent")
	}
	
	controlPlane, err := c.GetClient().GetControlPlaneByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve control plane by ID %s: %w", id, err)
	}
	
	return controlPlane, nil
}

// GetBySelector retrieves control planes by selector fields
func (c *ControlPlaneResolutionAdapter) GetBySelector(
	ctx context.Context, selector map[string]string, parent *external.ResolvedParent,
) ([]interface{}, error) {
	if parent != nil {
		return nil, fmt.Errorf("control_plane is a top-level resource and cannot have a parent")
	}
	
	controlPlanes, err := c.GetClient().ListControlPlanesWithFilter(ctx, selector)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve control planes by selector %v: %w", selector, err)
	}
	
	// FilterBySelector expects exactly one match and returns it
	controlPlane, err := c.FilterBySelector(controlPlanes, selector, func(resource interface{}, field string) string {
		cp := resource.(*state.ControlPlane)
		switch field {
		case "name":
			return cp.Name
		case "description":
			if cp.Description != nil {
				return *cp.Description
			}
			return ""
		default:
			return ""
		}
	})
	
	if err != nil {
		return nil, err
	}
	
	return []interface{}{controlPlane}, nil
}