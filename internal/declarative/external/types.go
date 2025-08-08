package external

import (
	"context"
	"time"
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

// ResolvedResource holds the resolved data for an external resource
type ResolvedResource struct {
	ID           string            // Resolved Konnect ID
	Resource     interface{}       // Full SDK response object
	ResourceType string            // Resource type (portal, api, etc.)
	Ref          string            // Original reference from config
	Parent       *ResolvedResource // Parent resource if applicable
	ResolvedAt   time.Time         // Resolution timestamp
}

// DependencyNode represents a node in the dependency graph
type DependencyNode struct {
	Ref          string   // External resource reference
	ResourceType string   // Resource type
	ParentRef    string   // Parent reference (empty for top-level)
	ChildRefs    []string // Child references
	Resolved     bool     // Resolution status
}

// DependencyGraph manages resolution ordering
type DependencyGraph struct {
	Nodes           map[string]*DependencyNode // All nodes by ref
	ResolutionOrder []string                   // Topologically sorted order
}

// Resource is an interface for external resource types
// This avoids circular imports with the resources package
type Resource interface {
	GetRef() string
	GetResourceType() string
	GetID() *string
	GetSelector() Selector
	GetParent() Parent
	SetResolvedID(id string)
	SetResolvedResource(resource interface{})
	IsResolved() bool
}

// Selector is an interface for external resource selectors
type Selector interface {
	GetMatchFields() map[string]string
}

// Parent is an interface for external resource parents
type Parent interface {
	GetResourceType() string
	GetID() string
	GetRef() string
}