package resources

import "fmt"

// matchOptions configures how TryMatchByName performs matching.
type matchOptions struct {
	// sdkType is the embedded struct name for SDK responses (e.g., "Portal", "APIResponseSchema").
	// Used when the SDK response embeds a type with Name/ID fields.
	sdkType string
}

// BaseResource provides the minimal common fields for declarative resources.
// Use this for resources that don't support external resource references.
type BaseResource struct {
	Ref       string       `yaml:"ref" json:"ref"`
	Kongctl   *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
	konnectID string       `yaml:"-" json:"-"`
}

// GetRef returns the reference identifier used for cross-resource references.
func (b BaseResource) GetRef() string {
	return b.Ref
}

// GetKonnectID returns the resolved Konnect ID if available.
func (b BaseResource) GetKonnectID() string {
	return b.konnectID
}

// SetKonnectID sets the resolved Konnect ID.
func (b *BaseResource) SetKonnectID(id string) {
	b.konnectID = id
}

// IsExternal returns false for core resources (no external support).
func (b BaseResource) IsExternal() bool {
	return false
}

// TryMatchByName attempts to match this resource with a Konnect resource by name.
// On successful match, sets konnectID and returns true.
func (b *BaseResource) TryMatchByName(resourceName string, konnectResource any, opts matchOptions) bool {
	name, id := extractNameAndID(konnectResource, opts.sdkType)
	if id == "" {
		return false
	}
	if name == resourceName {
		b.konnectID = id
		return true
	}
	return false
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup.
func (b BaseResource) GetKonnectMonikerFilter(name string) string {
	if name == "" {
		return ""
	}
	return fmt.Sprintf("name[eq]=%s", name)
}
