package resources

import (
	"fmt"

	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
)

// PortalResource represents a portal in declarative configuration
type PortalResource struct {
	kkInternalComps.CreatePortal `yaml:",inline"`
	Ref     string       `yaml:"ref"`
	Kongctl *KongctlMeta `yaml:"kongctl,omitempty"`
}

// GetRef returns the reference identifier used for cross-resource references
func (p PortalResource) GetRef() string {
	return p.Ref
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (p PortalResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{
		"default_application_auth_strategy_id": "application_auth_strategy",
	}
}

// Validate ensures the portal resource is valid
func (p PortalResource) Validate() error {
	if p.Ref == "" {
		return fmt.Errorf("portal ref is required")
	}
	return nil
}

// SetDefaults applies default values to portal resource
func (p *PortalResource) SetDefaults() {
	// If Name is not set, use ref as default
	if p.Name == "" {
		p.Name = p.Ref
	}
}