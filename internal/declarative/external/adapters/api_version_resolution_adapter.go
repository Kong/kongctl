package adapters

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/state"
)

// APIVersionResolutionAdapter handles API version resource resolution
type APIVersionResolutionAdapter struct {
	*BaseAdapter
}

// NewAPIVersionResolutionAdapter creates a new API version resolution adapter
func NewAPIVersionResolutionAdapter(client *state.Client) *APIVersionResolutionAdapter {
	return &APIVersionResolutionAdapter{
		BaseAdapter: NewBaseAdapter(client),
	}
}

// GetByID retrieves an API version by ID with parent context
func (a *APIVersionResolutionAdapter) GetByID(
	ctx context.Context, id string, parent *external.ResolvedParent,
) (interface{}, error) {
	if err := a.ValidateParentContext(parent, "api"); err != nil {
		return nil, err
	}
	
	version, err := a.GetClient().GetAPIVersionByID(ctx, parent.ID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve API version %s for API %s: %w", id, parent.ID, err)
	}
	
	return version, nil
}

// GetBySelector retrieves API versions by selector fields with parent context
func (a *APIVersionResolutionAdapter) GetBySelector(
	ctx context.Context, selector map[string]string, parent *external.ResolvedParent,
) ([]interface{}, error) {
	if err := a.ValidateParentContext(parent, "api"); err != nil {
		return nil, err
	}
	
	versions, err := a.GetClient().ListAPIVersionsWithFilter(ctx, parent.ID, selector)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve API versions for API %s: %w", parent.ID, err)
	}
	
	// FilterBySelector expects exactly one match and returns it
	version, err := a.FilterBySelector(versions, selector, func(resource interface{}, field string) string {
		v := resource.(*state.APIVersion)
		switch field {
		case "version":
			return v.Version
		default:
			return ""
		}
	})
	
	if err != nil {
		return nil, err
	}
	
	return []interface{}{version}, nil
}