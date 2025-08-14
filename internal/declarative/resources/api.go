package resources

import (
	"fmt"
	"reflect"

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
	
	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetType returns the resource type
func (a APIResource) GetType() ResourceType {
	return ResourceTypeAPI
}

// GetRef returns the reference identifier used for cross-resource references
func (a APIResource) GetRef() string {
	return a.Ref
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

// GetKonnectID returns the resolved Konnect ID if available
func (a APIResource) GetKonnectID() string {
	return a.konnectID
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (a APIResource) GetKonnectMonikerFilter() string {
	if a.Name == "" {
		return ""
	}
	return fmt.Sprintf("name[eq]=%s", a.Name)
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (a *APIResource) TryMatchKonnectResource(konnectResource any) bool {
	// For APIs, we match by name
	// Use reflection to access fields from state.API
	v := reflect.ValueOf(konnectResource)
	
	// Handle pointer types
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	
	// Ensure we have a struct
	if v.Kind() != reflect.Struct {
		return false
	}
	
	// Look for Name and ID fields
	nameField := v.FieldByName("Name")
	idField := v.FieldByName("ID")
	
	if !nameField.IsValid() || !idField.IsValid() {
		// Try accessing embedded APIResponseSchema
		apiField := v.FieldByName("APIResponseSchema")
		if apiField.IsValid() && apiField.Kind() == reflect.Struct {
			nameField = apiField.FieldByName("Name")
			idField = apiField.FieldByName("ID")
		}
	}
	
	// Extract values if fields are valid
	if nameField.IsValid() && idField.IsValid() && 
	   nameField.Kind() == reflect.String && idField.Kind() == reflect.String {
		if nameField.String() == a.Name {
			a.konnectID = idField.String()
			return true
		}
	}
	
	return false
}

