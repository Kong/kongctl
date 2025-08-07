package adapters

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/state"
)

// APIResolutionAdapter handles API resource resolution
type APIResolutionAdapter struct {
	*BaseAdapter
}

// NewAPIResolutionAdapter creates a new API resolution adapter
func NewAPIResolutionAdapter(client *state.Client) *APIResolutionAdapter {
	return &APIResolutionAdapter{
		BaseAdapter: NewBaseAdapter(client),
	}
}

// GetByID retrieves an API by ID
func (a *APIResolutionAdapter) GetByID(
	ctx context.Context, id string, parent *external.ResolvedParent,
) (interface{}, error) {
	if parent != nil {
		return nil, fmt.Errorf("api is a top-level resource and cannot have a parent")
	}
	
	// Use existing GetAPIByID method
	api, err := a.GetClient().GetAPIByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve api by ID %s: %w", id, err)
	}
	
	return api, nil
}

// GetBySelector retrieves APIs by selector fields
func (a *APIResolutionAdapter) GetBySelector(
	ctx context.Context, selector map[string]string, parent *external.ResolvedParent,
) ([]interface{}, error) {
	if parent != nil {
		return nil, fmt.Errorf("api is a top-level resource and cannot have a parent")
	}
	
	apis, err := a.GetClient().ListAPIsWithFilter(ctx, selector)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve apis by selector %v: %w", selector, err)
	}
	
	// FilterBySelector expects exactly one match and returns it
	api, err := a.FilterBySelector(apis, selector, func(resource interface{}, field string) string {
		api := resource.(*state.API)
		switch field {
		case "name":
			return api.Name
		case "description":
			if api.Description != nil {
				return *api.Description
			}
			return ""
		default:
			return ""
		}
	})
	
	if err != nil {
		return nil, err
	}
	
	return []interface{}{api}, nil
}