package resources

import (
	"fmt"

	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
)

// APIPublicationResource represents an API publication in declarative configuration
type APIPublicationResource struct {
	kkInternalComps.APIPublication `yaml:",inline"`
	Ref      string       `yaml:"ref"`
	PortalID string       `yaml:"portal_id"`
	Kongctl  *KongctlMeta `yaml:"kongctl,omitempty"`
	// Note: api_id removed - implicit from parent API structure
}

// GetRef returns the reference identifier used for cross-resource references
func (p APIPublicationResource) GetRef() string {
	return p.Ref
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (p APIPublicationResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{
		"portal_id":         "portal",
		// Note: api_id removed - implicit from parent API structure
		"auth_strategy_ids": "application_auth_strategy",
	}
}

// Validate ensures the API publication resource is valid
func (p APIPublicationResource) Validate() error {
	if p.Ref == "" {
		return fmt.Errorf("API publication ref is required")
	}
	if p.PortalID == "" {
		return fmt.Errorf("API publication portal_id is required")
	}
	// Note: api_id validation removed - implicit from parent API structure
	return nil
}

// SetDefaults applies default values to API publication resource
func (p *APIPublicationResource) SetDefaults() {
	// API publications typically don't need default values
}