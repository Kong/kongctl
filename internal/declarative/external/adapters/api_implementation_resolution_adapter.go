package adapters

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/state"
)

// APIImplementationResolutionAdapter handles API implementation resource resolution
type APIImplementationResolutionAdapter struct {
	*BaseAdapter
}

// NewAPIImplementationResolutionAdapter creates a new API implementation resolution adapter
func NewAPIImplementationResolutionAdapter(client *state.Client) *APIImplementationResolutionAdapter {
	return &APIImplementationResolutionAdapter{
		BaseAdapter: NewBaseAdapter(client),
	}
}

// GetByID retrieves an API implementation by ID with parent context
func (a *APIImplementationResolutionAdapter) GetByID(
	ctx context.Context, id string, parent *external.ResolvedParent,
) (interface{}, error) {
	if err := a.ValidateParentContext(parent, "api"); err != nil {
		return nil, err
	}
	
	impl, err := a.GetClient().GetAPIImplementationByID(ctx, parent.ID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve API implementation %s for API %s: %w", id, parent.ID, err)
	}
	
	return impl, nil
}

// GetBySelector retrieves API implementations by selector fields with parent context
func (a *APIImplementationResolutionAdapter) GetBySelector(
	ctx context.Context, selector map[string]string, parent *external.ResolvedParent,
) ([]interface{}, error) {
	if err := a.ValidateParentContext(parent, "api"); err != nil {
		return nil, err
	}
	
	impls, err := a.GetClient().ListAPIImplementationsWithFilter(ctx, parent.ID, selector)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve API implementations for API %s: %w", parent.ID, err)
	}
	
	// Note: API implementations don't have many filterable fields
	// Filtering will be done if selector fields are supported
	return impls, nil
}