package resources

import (
	"encoding/json"
	"fmt"

	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
)

// APIPublicationResource represents an API publication in declarative configuration
type APIPublicationResource struct {
	kkInternalComps.APIPublication `yaml:",inline" json:",inline"`
	Ref      string       `yaml:"ref" json:"ref"`
	PortalID string       `yaml:"portal_id" json:"portal_id"`
	Kongctl  *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
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

// UnmarshalJSON implements custom JSON unmarshaling to handle SDK types
func (p *APIPublicationResource) UnmarshalJSON(data []byte) error {
	// Temporary struct to capture all fields
	var temp struct {
		Ref                      string   `json:"ref"`
		PortalID                 string   `json:"portal_id"`
		PublishStatus            string   `json:"publish_status,omitempty"`
		AuthStrategyIds          []string `json:"auth_strategy_ids,omitempty"`
		AutoApproveRegistrations *bool    `json:"auto_approve_registrations,omitempty"`
		Visibility               string   `json:"visibility,omitempty"`
		Kongctl                  *KongctlMeta `json:"kongctl,omitempty"`
	}
	
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	
	// Set our custom fields
	p.Ref = temp.Ref
	p.PortalID = temp.PortalID
	p.Kongctl = temp.Kongctl
	
	// Map to SDK fields
	p.AuthStrategyIds = temp.AuthStrategyIds
	p.AutoApproveRegistrations = temp.AutoApproveRegistrations
	
	// Handle visibility enum if present
	if temp.Visibility != "" {
		visibility := kkInternalComps.APIPublicationVisibility(temp.Visibility)
		p.Visibility = &visibility
	}
	
	return nil
}