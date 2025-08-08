package adapters

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/state"
)

// ApplicationAuthStrategyResolutionAdapter handles application auth strategy resource resolution
type ApplicationAuthStrategyResolutionAdapter struct {
	*BaseAdapter
}

// NewApplicationAuthStrategyResolutionAdapter creates a new application auth strategy resolution adapter
func NewApplicationAuthStrategyResolutionAdapter(client *state.Client) *ApplicationAuthStrategyResolutionAdapter {
	return &ApplicationAuthStrategyResolutionAdapter{
		BaseAdapter: NewBaseAdapter(client),
	}
}

// GetByID retrieves an application auth strategy by ID
func (a *ApplicationAuthStrategyResolutionAdapter) GetByID(
	ctx context.Context, id string, parent *external.ResolvedParent,
) (interface{}, error) {
	if parent != nil {
		return nil, fmt.Errorf("application_auth_strategy is a top-level resource and cannot have a parent")
	}
	
	strategy, err := a.GetClient().GetApplicationAuthStrategyByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve application auth strategy by ID %s: %w", id, err)
	}
	
	return strategy, nil
}

// GetBySelector retrieves application auth strategies by selector fields
func (a *ApplicationAuthStrategyResolutionAdapter) GetBySelector(
	ctx context.Context, selector map[string]string, parent *external.ResolvedParent,
) ([]interface{}, error) {
	if parent != nil {
		return nil, fmt.Errorf("application_auth_strategy is a top-level resource and cannot have a parent")
	}
	
	strategies, err := a.GetClient().ListApplicationAuthStrategiesWithFilter(ctx, selector)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve application auth strategies by selector %v: %w", selector, err)
	}
	
	// FilterBySelector expects exactly one match and returns it
	strategy, err := a.FilterBySelector(strategies, selector, func(resource interface{}, field string) string {
		s := resource.(*state.ApplicationAuthStrategy)
		switch field {
		case "name":
			return s.Name
		case "display_name":
			return s.DisplayName
		default:
			return ""
		}
	})
	
	if err != nil {
		return nil, err
	}
	
	return []interface{}{strategy}, nil
}