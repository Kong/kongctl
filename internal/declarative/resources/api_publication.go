package resources

import (
	"encoding/json"
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// APIPublicationResource represents an API publication in declarative configuration
type APIPublicationResource struct {
	kkComps.APIPublication `yaml:",inline" json:",inline"`
	Ref      string       `yaml:"ref" json:"ref"`
	API      string       `yaml:"api,omitempty" json:"api,omitempty"` // Parent API reference (for root-level definitions)
	PortalID string       `yaml:"portal_id" json:"portal_id"`
	Kongctl  *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
	
	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetKind returns the resource kind
func (p APIPublicationResource) GetKind() string {
	return "api_publication"
}

// GetRef returns the reference identifier used for cross-resource references
func (p APIPublicationResource) GetRef() string {
	return p.Ref
}

// GetMoniker returns the resource moniker (for publications, this is the portal ID)
func (p APIPublicationResource) GetMoniker() string {
	// API publications use portal ID as their identifier
	return p.PortalID
}

// GetDependencies returns references to other resources this API publication depends on
func (p APIPublicationResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if p.API != "" {
		// Dependency on parent API when defined at root level
		deps = append(deps, ResourceRef{Kind: "api", Ref: p.API})
	}
	// Note: Portal dependency is handled through reference field mappings
	return deps
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (p APIPublicationResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{
		"portal_id":         "portal",
		"auth_strategy_ids": "application_auth_strategy",
	}
}

// Validate ensures the API publication resource is valid
func (p APIPublicationResource) Validate() error {
	if err := ValidateRef(p.Ref); err != nil {
		return fmt.Errorf("invalid API publication ref: %w", err)
	}
	if p.PortalID == "" {
		return fmt.Errorf("API publication portal_id is required")
	}
	// Parent API validation happens through dependency system
	return nil
}

// SetDefaults applies default values to API publication resource
func (p *APIPublicationResource) SetDefaults() {
	// API publications typically don't need default values
}

// GetKonnectID returns the resolved Konnect ID if available
func (p APIPublicationResource) GetKonnectID() string {
	return p.konnectID
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (p APIPublicationResource) GetKonnectMonikerFilter() string {
	// API publications are filtered by portal_id
	if p.PortalID == "" {
		return ""
	}
	return fmt.Sprintf("portal_id[eq]=%s", p.PortalID)
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (p *APIPublicationResource) TryMatchKonnectResource(konnectResource interface{}) bool {
	// For API publications, we match by portal ID
	// Use reflection to access fields from state.APIPublication
	v := reflect.ValueOf(konnectResource)
	
	// Handle pointer types
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	
	// Ensure we have a struct
	if v.Kind() != reflect.Struct {
		return false
	}
	
	// Look for PortalID and ID fields
	portalIDField := v.FieldByName("PortalID")
	idField := v.FieldByName("ID")
	
	// Extract values if fields are valid
	if portalIDField.IsValid() && idField.IsValid() && 
	   portalIDField.Kind() == reflect.String && idField.Kind() == reflect.String {
		if portalIDField.String() == p.PortalID {
			p.konnectID = idField.String()
			return true
		}
	}
	
	return false
}

// GetParentRef returns the parent API reference for ResourceWithParent interface
func (p APIPublicationResource) GetParentRef() *ResourceRef {
	if p.API != "" {
		return &ResourceRef{Kind: "api", Ref: p.API}
	}
	return nil
}

// UnmarshalJSON implements custom JSON unmarshaling to handle SDK types
func (p *APIPublicationResource) UnmarshalJSON(data []byte) error {
	// Temporary struct to capture all fields
	var temp struct {
		Ref                      string   `json:"ref"`
		API                      string   `json:"api,omitempty"`
		PortalID                 string   `json:"portal_id"`
		PublishStatus            string   `json:"publish_status,omitempty"`
		AuthStrategyIDs          []string `json:"auth_strategy_ids,omitempty"`
		AutoApproveRegistrations *bool    `json:"auto_approve_registrations,omitempty"`
		Visibility               string   `json:"visibility,omitempty"`
		Kongctl                  *KongctlMeta `json:"kongctl,omitempty"`
	}
	
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	
	// Set our custom fields
	p.Ref = temp.Ref
	p.API = temp.API
	p.PortalID = temp.PortalID
	p.Kongctl = temp.Kongctl
	
	// Map to SDK fields
	p.AuthStrategyIds = temp.AuthStrategyIDs
	p.AutoApproveRegistrations = temp.AutoApproveRegistrations
	
	// Handle visibility enum if present
	if temp.Visibility != "" {
		visibility := kkComps.APIPublicationVisibility(temp.Visibility)
		p.Visibility = &visibility
	}
	
	return nil
}