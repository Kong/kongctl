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

// Global lookup methods - search across all namespaces

// GetPortalByRef returns a portal resource by its ref from any namespace
func (rs *ResourceSet) GetPortalByRef(ref string) *PortalResource {
	for i := range rs.Portals {
		if rs.Portals[i].GetRef() == ref {
			return &rs.Portals[i]
		}
	}
	return nil
}

// GetAPIByRef returns an API resource by its ref from any namespace
func (rs *ResourceSet) GetAPIByRef(ref string) *APIResource {
	for i := range rs.APIs {
		if rs.APIs[i].GetRef() == ref {
			return &rs.APIs[i]
		}
	}
	return nil
}

// GetAuthStrategyByRef returns an auth strategy resource by its ref from any namespace
func (rs *ResourceSet) GetAuthStrategyByRef(ref string) *ApplicationAuthStrategyResource {
	for i := range rs.ApplicationAuthStrategies {
		if rs.ApplicationAuthStrategies[i].GetRef() == ref {
			return &rs.ApplicationAuthStrategies[i]
		}
	}
	return nil
}

// Namespace-filtered access methods

// GetPortalsByNamespace returns all portal resources from the specified namespace
func (rs *ResourceSet) GetPortalsByNamespace(namespace string) []PortalResource {
	var filtered []PortalResource
	for _, portal := range rs.Portals {
		if GetNamespace(portal.Kongctl) == namespace {
			filtered = append(filtered, portal)
		}
	}
	return filtered
}

// GetAPIsByNamespace returns all API resources from the specified namespace
func (rs *ResourceSet) GetAPIsByNamespace(namespace string) []APIResource {
	var filtered []APIResource
	for _, api := range rs.APIs {
		if GetNamespace(api.Kongctl) == namespace {
			filtered = append(filtered, api)
		}
	}
	return filtered
}

// GetAuthStrategiesByNamespace returns all auth strategy resources from the specified namespace
func (rs *ResourceSet) GetAuthStrategiesByNamespace(namespace string) []ApplicationAuthStrategyResource {
	var filtered []ApplicationAuthStrategyResource
	for _, strategy := range rs.ApplicationAuthStrategies {
		if GetNamespace(strategy.Kongctl) == namespace {
			filtered = append(filtered, strategy)
		}
	}
	return filtered
}

// GetAPIVersionsByNamespace returns all API version resources from the specified namespace
func (rs *ResourceSet) GetAPIVersionsByNamespace(namespace string) []APIVersionResource {
	var filtered []APIVersionResource
	for _, version := range rs.APIVersions {
		// Check if parent API is in the namespace
		if api := rs.GetAPIByRef(version.API); api != nil && GetNamespace(api.Kongctl) == namespace {
			filtered = append(filtered, version)
		}
	}
	return filtered
}

// GetAPIPublicationsByNamespace returns all API publication resources from the specified namespace
func (rs *ResourceSet) GetAPIPublicationsByNamespace(namespace string) []APIPublicationResource {
	var filtered []APIPublicationResource
	for _, pub := range rs.APIPublications {
		// Check if parent API is in the namespace
		if api := rs.GetAPIByRef(pub.API); api != nil && GetNamespace(api.Kongctl) == namespace {
			filtered = append(filtered, pub)
		}
	}
	return filtered
}

// GetAPIImplementationsByNamespace returns all API implementation resources from the specified namespace
func (rs *ResourceSet) GetAPIImplementationsByNamespace(namespace string) []APIImplementationResource {
	var filtered []APIImplementationResource
	for _, impl := range rs.APIImplementations {
		// Check if parent API is in the namespace
		if api := rs.GetAPIByRef(impl.API); api != nil && GetNamespace(api.Kongctl) == namespace {
			filtered = append(filtered, impl)
		}
	}
	return filtered
}

// GetAPIDocumentsByNamespace returns all API document resources from the specified namespace
func (rs *ResourceSet) GetAPIDocumentsByNamespace(namespace string) []APIDocumentResource {
	var filtered []APIDocumentResource
	for _, doc := range rs.APIDocuments {
		// Check if parent API is in the namespace
		if api := rs.GetAPIByRef(doc.API); api != nil && GetNamespace(api.Kongctl) == namespace {
			filtered = append(filtered, doc)
		}
	}
	return filtered
}

// GetPortalCustomizationsByNamespace returns all portal customization resources from the specified namespace
func (rs *ResourceSet) GetPortalCustomizationsByNamespace(namespace string) []PortalCustomizationResource {
	var filtered []PortalCustomizationResource
	for _, custom := range rs.PortalCustomizations {
		// Check if parent portal is in the namespace
		if portal := rs.GetPortalByRef(custom.Portal); portal != nil && GetNamespace(portal.Kongctl) == namespace {
			filtered = append(filtered, custom)
		}
	}
	return filtered
}

// GetPortalCustomDomainsByNamespace returns all portal custom domain resources from the specified namespace
func (rs *ResourceSet) GetPortalCustomDomainsByNamespace(namespace string) []PortalCustomDomainResource {
	var filtered []PortalCustomDomainResource
	for _, domain := range rs.PortalCustomDomains {
		// Check if parent portal is in the namespace
		if portal := rs.GetPortalByRef(domain.Portal); portal != nil && GetNamespace(portal.Kongctl) == namespace {
			filtered = append(filtered, domain)
		}
	}
	return filtered
}

// GetPortalPagesByNamespace returns all portal page resources from the specified namespace
func (rs *ResourceSet) GetPortalPagesByNamespace(namespace string) []PortalPageResource {
	var filtered []PortalPageResource
	for _, page := range rs.PortalPages {
		// Check if parent portal is in the namespace
		if portal := rs.GetPortalByRef(page.Portal); portal != nil && GetNamespace(portal.Kongctl) == namespace {
			filtered = append(filtered, page)
		}
	}
	return filtered
}

// GetPortalSnippetsByNamespace returns all portal snippet resources from the specified namespace
func (rs *ResourceSet) GetPortalSnippetsByNamespace(namespace string) []PortalSnippetResource {
	var filtered []PortalSnippetResource
	for _, snippet := range rs.PortalSnippets {
		// Check if parent portal is in the namespace
		if portal := rs.GetPortalByRef(snippet.Portal); portal != nil && GetNamespace(portal.Kongctl) == namespace {
			filtered = append(filtered, snippet)
		}
	}
	return filtered
}

// GetNamespace safely extracts namespace from kongctl metadata
func GetNamespace(kongctl *KongctlMeta) string {
	if kongctl == nil || kongctl.Namespace == nil {
		return "default"
	}
	return *kongctl.Namespace
}

