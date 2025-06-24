package resources

// ResourceSet contains all declarative resources from configuration files
type ResourceSet struct {
	Portals                   []PortalResource                  `yaml:"portals,omitempty"`
	ApplicationAuthStrategies []ApplicationAuthStrategyResource `yaml:"application_auth_strategies,omitempty"`
	APIs                      []APIResource                     `yaml:"apis,omitempty"`
	APIVersions               []APIVersionResource              `yaml:"api_versions,omitempty"`
	APIPublications           []APIPublicationResource          `yaml:"api_publications,omitempty"`
	APIImplementations        []APIImplementationResource       `yaml:"api_implementations,omitempty"`
}

// KongctlMeta contains tool-specific metadata for resources
type KongctlMeta struct {
	// Protected prevents accidental deletion of critical resources
	Protected bool `yaml:"protected,omitempty"`
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

// Placeholder resource types - will be implemented in subsequent steps
type PortalResource struct{}
type ApplicationAuthStrategyResource struct{}
type APIResource struct{}
type APIVersionResource struct{}
type APIPublicationResource struct{}
type APIImplementationResource struct{}