package adapters

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/state"
)

// APIPublicationResolutionAdapter handles API publication resource resolution
type APIPublicationResolutionAdapter struct {
	*BaseAdapter
}

// NewAPIPublicationResolutionAdapter creates a new API publication resolution adapter
func NewAPIPublicationResolutionAdapter(client *state.Client) *APIPublicationResolutionAdapter {
	return &APIPublicationResolutionAdapter{
		BaseAdapter: NewBaseAdapter(client),
	}
}

// GetByID retrieves an API publication by ID with parent context
func (a *APIPublicationResolutionAdapter) GetByID(
	ctx context.Context, id string, parent *external.ResolvedParent,
) (interface{}, error) {
	if err := a.ValidateParentContext(parent, "api"); err != nil {
		return nil, err
	}
	
	// API publication ID is the portal ID
	publication, err := a.GetClient().GetAPIPublicationByID(ctx, parent.ID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve API publication for API %s and portal %s: %w", parent.ID, id, err)
	}
	
	return publication, nil
}

// GetBySelector retrieves API publications by selector fields with parent context
func (a *APIPublicationResolutionAdapter) GetBySelector(
	ctx context.Context, selector map[string]string, parent *external.ResolvedParent,
) ([]interface{}, error) {
	if err := a.ValidateParentContext(parent, "api"); err != nil {
		return nil, err
	}
	
	publications, err := a.GetClient().ListAPIPublicationsWithFilter(ctx, parent.ID, selector)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve API publications for API %s: %w", parent.ID, err)
	}
	
	// FilterBySelector expects exactly one match and returns it
	publication, err := a.FilterBySelector(publications, selector, func(resource interface{}, field string) string {
		pub := resource.(*state.APIPublication)
		switch field {
		case "portal_id":
			return pub.PortalID
		default:
			return ""
		}
	})
	
	if err != nil {
		return nil, err
	}
	
	return []interface{}{publication}, nil
}