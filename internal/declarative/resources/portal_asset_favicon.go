package resources

import (
	"fmt"
)

func init() {
	registerResourceType(
		ResourceTypePortalAssetFavicon,
		func(rs *ResourceSet) *[]PortalAssetFaviconResource { return &rs.PortalAssetFavicons },
	)
}

// PortalAssetFaviconResource represents a portal favicon asset
// This is a singleton resource - only UPDATE operations are supported
type PortalAssetFaviconResource struct {
	Ref    string  `yaml:"ref"            json:"ref"`
	Portal string  `yaml:"portal"         json:"portal"`         // Parent portal reference
	File   *string `yaml:"file,omitempty" json:"file,omitempty"` // Data URL from !file tag

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetType returns the resource type
func (r PortalAssetFaviconResource) GetType() ResourceType {
	return ResourceTypePortalAssetFavicon
}

// GetRef returns the reference identifier
func (r PortalAssetFaviconResource) GetRef() string {
	return r.Ref
}

// GetMoniker returns the resource moniker (for singletons, this is the ref)
func (r PortalAssetFaviconResource) GetMoniker() string {
	return r.Ref // No natural moniker for singleton assets
}

// GetDependencies returns references to other resources this asset depends on
func (r PortalAssetFaviconResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if r.Portal != "" {
		deps = append(deps, ResourceRef{
			Kind: "portal",
			Ref:  r.Portal,
		})
	}
	return deps
}

// Validate ensures the portal asset favicon resource is valid
func (r PortalAssetFaviconResource) Validate() error {
	if err := ValidateRef(r.Ref); err != nil {
		return fmt.Errorf("invalid portal asset favicon ref: %w", err)
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
func (r *PortalAssetFaviconResource) SetDefaults() {
	// No defaults needed for portal asset favicon
}

// GetKonnectID returns the resolved Konnect ID if available
func (r PortalAssetFaviconResource) GetKonnectID() string {
	return r.konnectID
}

// GetKonnectMonikerFilter returns filter for API lookup
// For singleton assets, there's no moniker-based lookup
func (r PortalAssetFaviconResource) GetKonnectMonikerFilter() string {
	return ""
}

// TryMatchKonnectResource matches against Konnect API response
// For singleton assets, matching is done by portal ID, not by resource attributes
func (r *PortalAssetFaviconResource) TryMatchKonnectResource(_ any) bool {
	// Assets don't have individual IDs in Konnect, they're part of the portal
	return false
}

// GetParentRef returns the parent resource reference
func (r PortalAssetFaviconResource) GetParentRef() *ResourceRef {
	if r.Portal != "" {
		return &ResourceRef{
			Kind: "portal",
			Ref:  r.Portal,
		}
	}
	return nil
}
