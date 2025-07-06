package resources

import (
	"encoding/json"
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

// GetKind returns the resource kind
func (a APIResource) GetKind() string {
	return "api"
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
func (a *APIResource) TryMatchKonnectResource(konnectResource interface{}) bool {
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

// UnmarshalJSON implements custom JSON unmarshaling to preserve empty labels
func (a *APIResource) UnmarshalJSON(data []byte) error {
	// Temporary struct to capture all fields including raw labels
	var temp struct {
		Ref              string                       `json:"ref"`
		Name             string                       `json:"name"`
		Description      *string                      `json:"description,omitempty"`
		Version          *string                      `json:"version,omitempty"`
		Slug             *string                      `json:"slug,omitempty"`
		Labels           json.RawMessage              `json:"labels,omitempty"`
		Attributes       any                          `json:"attributes,omitempty"`
		SpecContent      *string                      `json:"spec_content,omitempty"`
		Kongctl          *KongctlMeta                 `json:"kongctl,omitempty"`
		Versions         []APIVersionResource         `json:"versions,omitempty"`
		Publications     []APIPublicationResource     `json:"publications,omitempty"`
		Implementations  []APIImplementationResource  `json:"implementations,omitempty"`
		Documents        []APIDocumentResource        `json:"documents,omitempty"`
	}
	
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	
	// Set our fields
	a.Ref = temp.Ref
	a.Kongctl = temp.Kongctl
	a.Name = temp.Name
	a.Description = temp.Description
	a.Version = temp.Version
	a.Slug = temp.Slug
	a.Attributes = temp.Attributes
	a.SpecContent = temp.SpecContent
	
	// Handle nested resources
	a.Versions = temp.Versions
	a.Publications = temp.Publications
	a.Implementations = temp.Implementations
	a.Documents = temp.Documents
	
	// Handle labels specially to preserve empty map vs nil
	if len(temp.Labels) > 0 {
		// Check if labels is null (happens when all values are commented out)
		if string(temp.Labels) == "null" {
			// Treat null as empty map - user wants to remove all labels
			a.Labels = make(map[string]string)
		} else {
			// Parse labels - if it's an empty object {}, we want to preserve that
			var labels map[string]string
			if err := json.Unmarshal(temp.Labels, &labels); err != nil {
				return fmt.Errorf("failed to unmarshal labels: %w", err)
			}
			a.Labels = labels
		}
	}
	// If labels field was not present in JSON, a.Labels remains nil
	
	return nil
}