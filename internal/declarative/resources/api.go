package resources

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// APIResource represents an API in declarative configuration
type APIResource struct {
	BaseResource
	kkComps.CreateAPIRequest `yaml:",inline" json:",inline"`

	// Nested child resources
	Versions        []APIVersionResource        `yaml:"versions,omitempty"        json:"versions,omitempty"`
	Publications    []APIPublicationResource    `yaml:"publications,omitempty"    json:"publications,omitempty"`
	Implementations []APIImplementationResource `yaml:"implementations,omitempty" json:"implementations,omitempty"`
	Documents       []APIDocumentResource       `yaml:"documents,omitempty"       json:"documents,omitempty"`
}

// GetType returns the resource type
func (a APIResource) GetType() ResourceType {
	return ResourceTypeAPI
}

// GetMoniker returns the resource moniker (for APIs, this is the name)
func (a APIResource) GetMoniker() string {
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
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid API ref: %w", err)
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

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (a APIResource) GetKonnectMonikerFilter() string {
	return a.BaseResource.GetKonnectMonikerFilter(a.Name)
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource.
func (a *APIResource) TryMatchKonnectResource(konnectResource any) bool {
	return a.TryMatchByName(a.Name, konnectResource, matchOptions{sdkType: "APIResponseSchema"})
}
