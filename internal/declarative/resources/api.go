package resources

import (
	"fmt"

	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
)

// APIResource represents an API in declarative configuration
type APIResource struct {
	kkInternalComps.CreateAPIRequest `yaml:",inline"`
	Ref     string       `yaml:"ref"`
	Kongctl *KongctlMeta `yaml:"kongctl,omitempty"`
}

// GetRef returns the reference identifier used for cross-resource references
func (a APIResource) GetRef() string {
	return a.Ref
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