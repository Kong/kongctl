package resources

import (
	"fmt"

	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
)

// APIVersionResource represents an API version in declarative configuration
type APIVersionResource struct {
	kkInternalComps.CreateAPIVersionRequest `yaml:",inline"`
	Ref     string       `yaml:"ref"`
	APIID   string       `yaml:"api_id"`
	Kongctl *KongctlMeta `yaml:"kongctl,omitempty"`
}

// GetRef returns the reference identifier used for cross-resource references
func (v APIVersionResource) GetRef() string {
	return v.Ref
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (v APIVersionResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{
		"api_id": "api",
	}
}

// Validate ensures the API version resource is valid
func (v APIVersionResource) Validate() error {
	if v.Ref == "" {
		return fmt.Errorf("API version ref is required")
	}
	if v.APIID == "" {
		return fmt.Errorf("API version api_id is required")
	}
	return nil
}

// SetDefaults applies default values to API version resource
func (v *APIVersionResource) SetDefaults() {
	// API versions typically don't need default values
}