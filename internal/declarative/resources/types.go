package resources

// ResourceType represents the type of a declarative resource
type ResourceType string

// Resource type constants
const (
	ResourceTypePortal                 ResourceType = "portal"
	ResourceTypeApplicationAuthStrategy ResourceType = "application_auth_strategy"
	ResourceTypeControlPlane           ResourceType = "control_plane"
	ResourceTypeAPI                    ResourceType = "api"
	ResourceTypeAPIVersion             ResourceType = "api_version"
	ResourceTypeAPIPublication         ResourceType = "api_publication"
	ResourceTypeAPIImplementation      ResourceType = "api_implementation"
	ResourceTypeAPIDocument            ResourceType = "api_document"
	ResourceTypePortalCustomization    ResourceType = "portal_customization"
	ResourceTypePortalCustomDomain     ResourceType = "portal_custom_domain"
	ResourceTypePortalPage             ResourceType = "portal_page"
	ResourceTypePortalSnippet          ResourceType = "portal_snippet"
)

// ResourceRef represents a reference to another resource
type ResourceRef struct {
	Kind string `json:"kind" yaml:"kind"`
	Ref  string `json:"ref" yaml:"ref"`
}

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
	
	// DefaultNamespace tracks namespace from _defaults when no resources are present
	// This is used by the planner to determine which namespace to check for deletions
	DefaultNamespace string `yaml:"-" json:"-"`
}

// KongctlMeta contains tool-specific metadata for resources
type KongctlMeta struct {
	// Protected prevents accidental deletion of critical resources
	Protected *bool `yaml:"protected,omitempty" json:"protected,omitempty"`
	// Namespace for resource isolation and multi-team management
	Namespace *string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
}

// FileDefaults holds file-level defaults that apply to all resources in the file
type FileDefaults struct {
	Kongctl *KongctlMetaDefaults `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
}

// KongctlMetaDefaults holds default values for kongctl metadata fields
type KongctlMetaDefaults struct {
	Namespace *string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Protected *bool   `yaml:"protected,omitempty" json:"protected,omitempty"`
}

// HasRef checks if a ref exists globally across all resource types
func (rs *ResourceSet) HasRef(ref string) bool {
	_, found := rs.GetResourceByRef(ref)
	return found
}

// GetResourceByRef returns the resource for a given ref
func (rs *ResourceSet) GetResourceByRef(ref string) (Resource, bool) {
	// Check Portals
	for i := range rs.Portals {
		if rs.Portals[i].GetRef() == ref {
			return &rs.Portals[i], true
		}
	}
	
	// Check ApplicationAuthStrategies
	for i := range rs.ApplicationAuthStrategies {
		if rs.ApplicationAuthStrategies[i].GetRef() == ref {
			return &rs.ApplicationAuthStrategies[i], true
		}
	}
	
	// Check ControlPlanes
	for i := range rs.ControlPlanes {
		if rs.ControlPlanes[i].GetRef() == ref {
			return &rs.ControlPlanes[i], true
		}
	}
	
	// Check APIs
	for i := range rs.APIs {
		if rs.APIs[i].GetRef() == ref {
			return &rs.APIs[i], true
		}
	}
	
	// Check API child resources
	for i := range rs.APIVersions {
		if rs.APIVersions[i].GetRef() == ref {
			return &rs.APIVersions[i], true
		}
	}
	
	for i := range rs.APIPublications {
		if rs.APIPublications[i].GetRef() == ref {
			return &rs.APIPublications[i], true
		}
	}
	
	for i := range rs.APIImplementations {
		if rs.APIImplementations[i].GetRef() == ref {
			return &rs.APIImplementations[i], true
		}
	}
	
	for i := range rs.APIDocuments {
		if rs.APIDocuments[i].GetRef() == ref {
			return &rs.APIDocuments[i], true
		}
	}
	
	// Check Portal child resources
	for i := range rs.PortalCustomizations {
		if rs.PortalCustomizations[i].GetRef() == ref {
			return &rs.PortalCustomizations[i], true
		}
	}
	
	for i := range rs.PortalCustomDomains {
		if rs.PortalCustomDomains[i].GetRef() == ref {
			return &rs.PortalCustomDomains[i], true
		}
	}
	
	for i := range rs.PortalPages {
		if rs.PortalPages[i].GetRef() == ref {
			return &rs.PortalPages[i], true
		}
	}
	
	for i := range rs.PortalSnippets {
		if rs.PortalSnippets[i].GetRef() == ref {
			return &rs.PortalSnippets[i], true
		}
	}
	
	return nil, false
}

// GetResourceTypeByRef returns the resource type for a given ref
func (rs *ResourceSet) GetResourceTypeByRef(ref string) (ResourceType, bool) {
	res, ok := rs.GetResourceByRef(ref)
	if !ok || res == nil {
		return "", false
	}
	return res.GetType(), true
}

