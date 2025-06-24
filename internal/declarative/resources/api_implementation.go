package resources

import (
	"fmt"
	"regexp"

	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
)

// APIImplementationResource represents an API implementation in declarative configuration
type APIImplementationResource struct {
	kkInternalComps.APIImplementation `yaml:",inline"`
	Ref     string       `yaml:"ref"`
	Kongctl *KongctlMeta `yaml:"kongctl,omitempty"`
}

// GetRef returns the reference identifier used for cross-resource references
func (i APIImplementationResource) GetRef() string {
	return i.Ref
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (i APIImplementationResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{
		"service.control_plane_id": "control_plane",
		// Note: service.id is external UUID, not ref-based
	}
}

// Validate ensures the API implementation resource is valid
func (i APIImplementationResource) Validate() error {
	if i.Ref == "" {
		return fmt.Errorf("API implementation ref is required")
	}
	
	// Validate service information if present
	if i.Service != nil {
		if i.Service.ID == "" {
			return fmt.Errorf("API implementation service.id is required")
		}
		
		// Validate service.id is a UUID format (external system)
		uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
		if !uuidRegex.MatchString(i.Service.ID) {
			return fmt.Errorf("API implementation service.id must be a valid UUID (external service managed by decK)")
		}
		
		if i.Service.ControlPlaneID == "" {
			return fmt.Errorf("API implementation service.control_plane_id is required")
		}
	}
	
	return nil
}

// SetDefaults applies default values to API implementation resource
func (i *APIImplementationResource) SetDefaults() {
	// API implementations typically don't need default values
}