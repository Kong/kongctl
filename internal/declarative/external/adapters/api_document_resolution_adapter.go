package adapters

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/kong/kongctl/internal/declarative/state"
)

// APIDocumentResolutionAdapter handles API document resource resolution
type APIDocumentResolutionAdapter struct {
	*BaseAdapter
}

// NewAPIDocumentResolutionAdapter creates a new API document resolution adapter
func NewAPIDocumentResolutionAdapter(client *state.Client) *APIDocumentResolutionAdapter {
	return &APIDocumentResolutionAdapter{
		BaseAdapter: NewBaseAdapter(client),
	}
}

// GetByID retrieves an API document by ID with parent context
func (a *APIDocumentResolutionAdapter) GetByID(
	ctx context.Context, id string, parent *external.ResolvedParent,
) (interface{}, error) {
	if err := a.ValidateParentContext(parent, "api"); err != nil {
		return nil, err
	}
	
	document, err := a.GetClient().GetAPIDocument(ctx, parent.ID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve API document %s for API %s: %w", id, parent.ID, err)
	}
	
	return document, nil
}

// GetBySelector retrieves API documents by selector fields with parent context
func (a *APIDocumentResolutionAdapter) GetBySelector(
	ctx context.Context, selector map[string]string, parent *external.ResolvedParent,
) ([]interface{}, error) {
	if err := a.ValidateParentContext(parent, "api"); err != nil {
		return nil, err
	}
	
	documents, err := a.GetClient().ListAPIDocumentsWithFilter(ctx, parent.ID, selector)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve API documents for API %s: %w", parent.ID, err)
	}
	
	// FilterBySelector expects exactly one match and returns it
	document, err := a.FilterBySelector(documents, selector, func(resource interface{}, field string) string {
		doc := resource.(*state.APIDocument)
		switch field {
		case "slug":
			return doc.Slug
		case "title":
			return doc.Title
		default:
			return ""
		}
	})
	
	if err != nil {
		return nil, err
	}
	
	return []interface{}{document}, nil
}