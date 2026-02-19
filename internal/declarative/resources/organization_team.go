package resources

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// OrganizationTeamResource represents a team in declarative configuration
type OrganizationTeamResource struct {
	BaseResource
	kkComps.CreateTeam `yaml:",inline" json:",inline"`
	External           *ExternalBlock `yaml:"_external,omitempty" json:"_external,omitempty"`
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (t OrganizationTeamResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{} // No outbound references
}

// Validate ensures the team resource is valid
func (t OrganizationTeamResource) Validate() error {
	if err := ValidateRef(t.Ref); err != nil {
		return fmt.Errorf("invalid team ref: %w", err)
	}

	if t.Name == "" {
		return fmt.Errorf("name is required")
	}

	if t.External != nil {
		if err := t.External.Validate(); err != nil {
			return fmt.Errorf("invalid _external block: %w", err)
		}
	}
	return nil
}

// SetDefaults applies default values to team resource
func (t *OrganizationTeamResource) SetDefaults() {
	// If Name is not set, use ref as default
	if t.Name == "" {
		t.Name = t.Ref
	}
}

// GetType returns the resource type
func (t OrganizationTeamResource) GetType() ResourceType {
	return ResourceTypeOrganizationTeam
}

// GetMoniker returns the resource moniker (for teams, this is the name)
func (t OrganizationTeamResource) GetMoniker() string {
	return t.Name
}

// GetDependencies returns references to other resources this team depends on
func (t OrganizationTeamResource) GetDependencies() []ResourceRef {
	// Teams don't depend on other resources
	return []ResourceRef{}
}

// GetLabels returns the labels for this resource
func (t OrganizationTeamResource) GetLabels() map[string]string {
	return t.Labels
}

// SetLabels sets the labels for this resource
func (t *OrganizationTeamResource) SetLabels(labels map[string]string) {
	t.Labels = labels
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (t OrganizationTeamResource) GetKonnectMonikerFilter() string {
	return t.BaseResource.GetKonnectMonikerFilter(t.Name)
}

// IsExternal returns true if this team is externally managed
func (t *OrganizationTeamResource) IsExternal() bool {
	return t.External != nil && t.External.IsExternal()
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (t *OrganizationTeamResource) TryMatchKonnectResource(konnectResource any) bool {
	return t.TryMatchByName(t.Name, konnectResource, matchOptions{})
}
