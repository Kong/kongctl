package resources

// Resource is the common interface for all declarative resources
type Resource interface {
	GetKind() string
	GetRef() string
	GetMoniker() string // Generic identifier (name, username, version, etc.)
	GetDependencies() []ResourceRef
	Validate() error
	SetDefaults()
	
	// Identity resolution methods
	GetKonnectID() string // Returns the Konnect ID if resolved, empty otherwise
	GetKonnectMonikerFilter() string // Returns filter string for Konnect API lookup
	TryMatchKonnectResource(konnectResource interface{}) bool // Matches against Konnect resource
}

// ResourceRef represents a reference to another resource
type ResourceRef struct {
	Kind string `json:"kind" yaml:"kind"`
	Ref  string `json:"ref" yaml:"ref"`
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