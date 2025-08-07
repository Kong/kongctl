package external

import (
	"context"
)

// ResolutionMetadata contains metadata needed to resolve external resources from Konnect
type ResolutionMetadata struct {
	// Human-readable name
	Name string

	// Supported fields for selector matching
	SelectorFields []string

	// Supported parent resource types
	SupportedParents []string

	// Supported child resource types
	SupportedChildren []string

	// Adapter for resolving resources via SDK
	ResolutionAdapter ResolutionAdapter
}

// ResolutionAdapter defines the interface for resolving external resources via SDK
type ResolutionAdapter interface {
	// GetByID retrieves a resource by its Konnect ID
	GetByID(ctx context.Context, id string, parent *ResolvedParent) (interface{}, error)

	// GetBySelector retrieves resources matching selector criteria
	GetBySelector(ctx context.Context, selector map[string]string, parent *ResolvedParent) ([]interface{}, error)
}

// ResolvedParent contains information about a resolved parent resource
type ResolvedParent struct {
	ResourceType string
	ID           string
	Resource     interface{}
}