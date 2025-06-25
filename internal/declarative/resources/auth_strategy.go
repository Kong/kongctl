package resources

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// ApplicationAuthStrategyResource represents an application auth strategy in declarative configuration
type ApplicationAuthStrategyResource struct {
	kkComps.CreateAppAuthStrategyRequest `yaml:",inline" json:",inline"`
	Ref     string       `yaml:"ref" json:"ref"`
	Kongctl *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
}

// GetRef returns the reference identifier used for cross-resource references
func (a ApplicationAuthStrategyResource) GetRef() string {
	return a.Ref
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (a ApplicationAuthStrategyResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{} // No outbound references
}

// Validate ensures the application auth strategy resource is valid
func (a ApplicationAuthStrategyResource) Validate() error {
	if a.Ref == "" {
		return fmt.Errorf("application auth strategy ref is required")
	}
	return nil
}

// SetDefaults applies default values to application auth strategy resource
func (a *ApplicationAuthStrategyResource) SetDefaults() {
	// No defaults to set for auth strategies
}