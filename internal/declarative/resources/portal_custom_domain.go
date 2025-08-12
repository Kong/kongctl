package resources

import (
	"fmt"
	"reflect"
	"regexp"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// PortalCustomDomainResource represents a portal custom domain configuration
type PortalCustomDomainResource struct {
	// Use CreatePortalCustomDomainRequest which contains only user-configurable fields
	// This aligns with the pattern used by other resources (API, APIVersion, etc.)
	kkComps.CreatePortalCustomDomainRequest `yaml:",inline" json:",inline"`
	Ref    string `yaml:"ref,omitempty" json:"ref,omitempty"`
	Portal string `yaml:"portal,omitempty" json:"portal,omitempty"` // Parent portal reference
	
	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetRef returns the reference identifier
func (d PortalCustomDomainResource) GetRef() string {
	return d.Ref
}

// Validate ensures the portal custom domain resource is valid
func (d PortalCustomDomainResource) Validate() error {
	if err := ValidateRef(d.Ref); err != nil {
		return fmt.Errorf("invalid custom domain ref: %w", err)
	}
	
	if d.Hostname == "" {
		return fmt.Errorf("custom domain hostname is required")
	}
	
	// Validate hostname format
	if !isValidHostname(d.Hostname) {
		return fmt.Errorf("invalid hostname format: %s", d.Hostname)
	}
	
	// SSL validation would go here once we understand the actual SSL structure
	// For now, just validate the hostname
	
	return nil
}

// SetDefaults applies default values
func (d *PortalCustomDomainResource) SetDefaults() {
	// No defaults needed for custom domains currently
}

// GetType returns the resource type
func (d PortalCustomDomainResource) GetType() ResourceType {
	return ResourceTypePortalCustomDomain
}

// GetMoniker returns the resource moniker (for custom domains, this is the hostname)
func (d PortalCustomDomainResource) GetMoniker() string {
	return d.Hostname
}

// GetDependencies returns references to other resources this custom domain depends on
func (d PortalCustomDomainResource) GetDependencies() []ResourceRef {
	// Portal custom domains don't have dependencies
	return []ResourceRef{}
}

// GetKonnectID returns the resolved Konnect ID if available
func (d PortalCustomDomainResource) GetKonnectID() string {
	return d.konnectID
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (d PortalCustomDomainResource) GetKonnectMonikerFilter() string {
	if d.Hostname == "" {
		return ""
	}
	return fmt.Sprintf("hostname[eq]=%s", d.Hostname)
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (d *PortalCustomDomainResource) TryMatchKonnectResource(konnectResource interface{}) bool {
	// For custom domains, we match by hostname
	// Use reflection to access fields from state.PortalCustomDomain
	v := reflect.ValueOf(konnectResource)
	
	// Handle pointer types
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	
	// Ensure we have a struct
	if v.Kind() != reflect.Struct {
		return false
	}
	
	// Look for Hostname and ID fields
	hostnameField := v.FieldByName("Hostname")
	idField := v.FieldByName("ID")
	
	// Extract values if fields are valid
	if hostnameField.IsValid() && idField.IsValid() && 
	   hostnameField.Kind() == reflect.String && idField.Kind() == reflect.String {
		if hostnameField.String() == d.Hostname {
			d.konnectID = idField.String()
			return true
		}
	}
	return false
}

// isValidHostname validates hostname format
func isValidHostname(hostname string) bool {
	// Basic hostname validation regex
	pattern := `^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)*` +
		`[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`
	hostnameRegex := regexp.MustCompile(pattern)
	return hostnameRegex.MatchString(hostname)
}