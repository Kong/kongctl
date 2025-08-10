package resources

import (
	"fmt"
	"sync"
)

// GlobalRefRegistry tracks refs across all resource types to ensure global uniqueness
type GlobalRefRegistry struct {
	refs  map[string]string // ref -> resource_type
	mutex sync.RWMutex
}

// NewGlobalRefRegistry creates a new registry for tracking global refs
func NewGlobalRefRegistry() *GlobalRefRegistry {
	return &GlobalRefRegistry{
		refs: make(map[string]string),
	}
}

// AddRef registers a ref, returning an error if it already exists
func (g *GlobalRefRegistry) AddRef(ref, resourceType string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	if existingType, exists := g.refs[ref]; exists {
		if existingType == resourceType {
			return fmt.Errorf("duplicate ref '%s': already used by another %s resource", ref, resourceType)
		}
		return fmt.Errorf("duplicate ref '%s': already used by %s resource, cannot use for %s resource",
			ref, existingType, resourceType)
	}

	g.refs[ref] = resourceType
	return nil
}

// HasRef checks if a ref exists in the registry
func (g *GlobalRefRegistry) HasRef(ref string) bool {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	_, exists := g.refs[ref]
	return exists
}

// GetResourceType returns the resource type for a given ref
func (g *GlobalRefRegistry) GetResourceType(ref string) (string, bool) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	resourceType, exists := g.refs[ref]
	return resourceType, exists
}

// GetAllRefs returns a copy of all refs and their resource types
func (g *GlobalRefRegistry) GetAllRefs() map[string]string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	result := make(map[string]string)
	for ref, resourceType := range g.refs {
		result[ref] = resourceType
	}
	return result
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

// FileDefaults holds file-level defaults that apply to all resources in the file
type FileDefaults struct {
	Kongctl *KongctlMetaDefaults `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
}

// KongctlMetaDefaults holds default values for kongctl metadata fields
type KongctlMetaDefaults struct {
	Namespace *string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Protected *bool   `yaml:"protected,omitempty" json:"protected,omitempty"`
}

