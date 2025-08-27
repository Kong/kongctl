package resources

// Resource is the common interface for all declarative resources
type Resource interface {
	GetType() ResourceType
	GetRef() string
	GetMoniker() string // Generic identifier (name, username, version, etc.)
	GetDependencies() []ResourceRef
	Validate() error
	SetDefaults()

	// Identity resolution methods
	GetKonnectID() string                             // Returns the Konnect ID if resolved, empty otherwise
	GetKonnectMonikerFilter() string                  // Returns filter string for Konnect API lookup
	TryMatchKonnectResource(konnectResource any) bool // Matches against Konnect resource
}

// RefReader provides read-only access to resources by ref
type RefReader interface {
	// HasRef checks if a ref exists globally across all resource types
	HasRef(ref string) bool

	// GetResourceByRef returns the resource for a given ref
	// Returns nil and false if not found
	GetResourceByRef(ref string) (Resource, bool)

	// GetResourceTypeByRef returns the resource type for a given ref
	// This is a convenience method that uses GetResourceByRef
	GetResourceTypeByRef(ref string) (ResourceType, bool)
}

// ResourceValidator interface for common validation behavior
type ResourceValidator interface {
	Validate() error
}

// ReferencedResource interface for resources that can be referenced
type ReferencedResource interface {
	GetRef() string
}

// ReferenceMapping interface for resources that have reference fields
type ReferenceMapping interface {
	GetReferenceFieldMappings() map[string]string
}

// ResourceWithParent represents resources that have a parent
type ResourceWithParent interface {
	Resource
	GetParentRef() *ResourceRef
}

// ResourceWithLabels represents resources that support labels
type ResourceWithLabels interface {
	Resource
	GetLabels() map[string]string
	SetLabels(map[string]string)
}
