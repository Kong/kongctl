package resources

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// APIResource represents an API in declarative configuration
type APIResource struct {
	kkComps.CreateAPIRequest `yaml:",inline" json:",inline"`
	Ref     string       `yaml:"ref" json:"ref"`
	Kongctl *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
	
	// Nested child resources
	Versions        []APIVersionResource        `yaml:"versions,omitempty" json:"versions,omitempty"`
	Publications    []APIPublicationResource    `yaml:"publications,omitempty" json:"publications,omitempty"`
	Implementations []APIImplementationResource `yaml:"implementations,omitempty" json:"implementations,omitempty"`
	Documents       []APIDocumentResource       `yaml:"documents,omitempty" json:"documents,omitempty"`
}

// GetKind returns the resource kind
func (a APIResource) GetKind() string {
	return "api"
}

// GetRef returns the reference identifier used for cross-resource references
func (a APIResource) GetRef() string {
	return a.Ref
}

// GetName returns the resource name
func (a APIResource) GetName() string {
	return a.Name
}

// GetDependencies returns references to other resources this API depends on
func (a APIResource) GetDependencies() []ResourceRef {
	// APIs don't depend on other resources
	return []ResourceRef{}
}

// GetLabels returns the labels for this resource
func (a APIResource) GetLabels() map[string]string {
	return a.Labels
}

// SetLabels sets the labels for this resource
func (a *APIResource) SetLabels(labels map[string]string) {
	a.Labels = labels
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (a APIResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{} // No outbound references
}

// Validate ensures the API resource is valid
func (a APIResource) Validate() error {
	if a.Ref == "" {
		return fmt.Errorf("API ref is required")
	}
	return nil
}

// SetDefaults applies default values to API resource
func (a *APIResource) SetDefaults() {
	// If Name is not set, use ref as default
	if a.Name == "" {
		a.Name = a.Ref
	}
}