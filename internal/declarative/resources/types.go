package resources

// ResourceSet contains all declarative resources from configuration files
type ResourceSet struct {
	Portals []PortalResource `yaml:"portals,omitempty" json:"portals,omitempty"`
	// ApplicationAuthStrategies contains auth strategy configurations
	ApplicationAuthStrategies []ApplicationAuthStrategyResource `yaml:"application_auth_strategies,omitempty" json:"application_auth_strategies,omitempty"` //nolint:lll
	// ControlPlanes contains control plane configurations
	ControlPlanes []ControlPlaneResource `yaml:"control_planes,omitempty" json:"control_planes,omitempty"`
	APIs          []APIResource          `yaml:"apis,omitempty" json:"apis,omitempty"`
	// API child resources can be defined at root level (with parent reference) or nested under APIs
	APIVersions        []APIVersionResource        `yaml:"api_versions,omitempty" json:"api_versions,omitempty"`
	APIPublications    []APIPublicationResource    `yaml:"api_publications,omitempty" json:"api_publications,omitempty"`
	APIImplementations []APIImplementationResource `yaml:"api_implementations,omitempty" json:"api_implementations,omitempty"` //nolint:lll
	APIDocuments       []APIDocumentResource       `yaml:"api_documents,omitempty" json:"api_documents,omitempty"`
	// Portal child resources can be defined at root level (with parent reference) or nested under Portals
	PortalCustomizations []PortalCustomizationResource `yaml:"portal_customizations,omitempty" json:"portal_customizations,omitempty"` //nolint:lll
	PortalCustomDomains  []PortalCustomDomainResource  `yaml:"portal_custom_domains,omitempty" json:"portal_custom_domains,omitempty"` //nolint:lll
	PortalPages          []PortalPageResource          `yaml:"portal_pages,omitempty" json:"portal_pages,omitempty"`
	PortalSnippets       []PortalSnippetResource       `yaml:"portal_snippets,omitempty" json:"portal_snippets,omitempty"`
}

// KongctlMeta contains tool-specific metadata for resources
type KongctlMeta struct {
	// Protected prevents accidental deletion of critical resources
	Protected bool `yaml:"protected,omitempty" json:"protected,omitempty"`
	// Namespace for resource isolation and multi-team management
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
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

