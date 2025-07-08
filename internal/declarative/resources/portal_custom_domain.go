package resources

import (
	"fmt"
	"regexp"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// PortalCustomDomainResource represents a portal custom domain configuration
type PortalCustomDomainResource struct {
	kkComps.PortalCustomDomain `yaml:",inline" json:",inline"`
	Ref    string `yaml:"ref,omitempty" json:"ref,omitempty"`
	Portal string `yaml:"portal,omitempty" json:"portal,omitempty"` // Parent portal reference
}

// GetRef returns the reference identifier
func (d PortalCustomDomainResource) GetRef() string {
	return d.Ref
}

// Validate ensures the portal custom domain resource is valid
func (d PortalCustomDomainResource) Validate() error {
	if d.Ref == "" {
		return fmt.Errorf("custom domain ref is required")
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

// isValidHostname validates hostname format
func isValidHostname(hostname string) bool {
	// Basic hostname validation regex
	pattern := `^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)*` +
		`[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`
	hostnameRegex := regexp.MustCompile(pattern)
	return hostnameRegex.MatchString(hostname)
}