package resources

import (
	"fmt"
	"strings"
)

func init() {
	registerResourceType(
		ResourceTypePortalAssetLogo,
		func(rs *ResourceSet) *[]PortalAssetLogoResource { return &rs.PortalAssetLogos },
	)
}

// PortalAssetLogoResource represents a portal logo asset
// This is a singleton resource - only UPDATE operations are supported
type PortalAssetLogoResource struct {
	Ref    string  `yaml:"ref"            json:"ref"`
	Portal string  `yaml:"portal"         json:"portal"`         // Parent portal reference
	File   *string `yaml:"file,omitempty" json:"file,omitempty"` // Data URL from !file tag

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetType returns the resource type
func (r PortalAssetLogoResource) GetType() ResourceType {
	return ResourceTypePortalAssetLogo
}

// GetRef returns the reference identifier
func (r PortalAssetLogoResource) GetRef() string {
	return r.Ref
}

// GetMoniker returns the resource moniker (for singletons, this is the ref)
func (r PortalAssetLogoResource) GetMoniker() string {
	return r.Ref // No natural moniker for singleton assets
}

// GetDependencies returns references to other resources this asset depends on
func (r PortalAssetLogoResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if r.Portal != "" {
		deps = append(deps, ResourceRef{
			Kind: "portal",
			Ref:  r.Portal,
		})
	}
	return deps
}

// Validate ensures the portal asset logo resource is valid
func (r PortalAssetLogoResource) Validate() error {
	if err := ValidateRef(r.Ref); err != nil {
		return fmt.Errorf("invalid portal asset logo ref: %w", err)
	}

	if r.Portal == "" {
		return fmt.Errorf("portal reference is required")
	}

	if r.File == nil || *r.File == "" {
		return fmt.Errorf("file is required (use !file tag to load image)")
	}

	// Validate that the file is a data URL (should be from !file tag)
	if !isDataURL(*r.File) {
		return fmt.Errorf("file must be a data URL (did you use !file tag?)")
	}

	return nil
}

// SetDefaults applies default values
func (r *PortalAssetLogoResource) SetDefaults() {
	// No defaults needed for portal asset logo
}

// GetKonnectID returns the resolved Konnect ID if available
func (r PortalAssetLogoResource) GetKonnectID() string {
	return r.konnectID
}

// GetKonnectMonikerFilter returns filter for API lookup
// For singleton assets, there's no moniker-based lookup
func (r PortalAssetLogoResource) GetKonnectMonikerFilter() string {
	return ""
}

// TryMatchKonnectResource matches against Konnect API response
// For singleton assets, matching is done by portal ID, not by resource attributes
func (r *PortalAssetLogoResource) TryMatchKonnectResource(_ any) bool {
	// Assets don't have individual IDs in Konnect, they're part of the portal
	return false
}

// GetParentRef returns the parent resource reference
func (r PortalAssetLogoResource) GetParentRef() *ResourceRef {
	if r.Portal != "" {
		return &ResourceRef{
			Kind: "portal",
			Ref:  r.Portal,
		}
	}
	return nil
}

// isDataURL checks if a string is a data URL
func isDataURL(s string) bool {
	return strings.HasPrefix(s, "data:image/")
}
