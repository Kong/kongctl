package resources

import (
	"fmt"
)

func init() {
	registerResourceType(
		ResourceTypeOrganizationTeamUser,
		func(rs *ResourceSet) *[]OrganizationTeamUserResource { return &rs.OrganizationTeamUsers },
	)
}

// OrganizationTeamUserResource represents a user member of an organization team.
// This is a child resource of OrganizationTeamResource.
type OrganizationTeamUserResource struct {
	Ref string `yaml:"ref" json:"ref"`

	// Team is the parent team reference
	Team string `yaml:"team,omitempty" json:"team,omitempty"`

	// Identity lookup fields — specify at least one
	ID    string `yaml:"id,omitempty"    json:"id,omitempty"`
	Name  string `yaml:"name,omitempty"  json:"name,omitempty"`
	Email string `yaml:"email,omitempty" json:"email,omitempty"`

	// Resolved Konnect user ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetType returns the resource type
func (u OrganizationTeamUserResource) GetType() ResourceType {
	return ResourceTypeOrganizationTeamUser
}

// GetRef returns the reference identifier
func (u OrganizationTeamUserResource) GetRef() string {
	return u.Ref
}

// GetMoniker returns a human-readable moniker for matching purposes
func (u OrganizationTeamUserResource) GetMoniker() string {
	if u.Email != "" {
		return u.Email
	}
	if u.Name != "" {
		return u.Name
	}
	return u.ID
}

// GetDependencies returns references to other resources this user depends on
func (u OrganizationTeamUserResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if u.Team != "" {
		deps = append(deps, ResourceRef{
			Kind: string(ResourceTypeOrganizationTeam),
			Ref:  u.Team,
		})
	}
	return deps
}

// Validate ensures the team user resource is valid
func (u OrganizationTeamUserResource) Validate() error {
	if err := ValidateRef(u.Ref); err != nil {
		return fmt.Errorf("invalid team user ref: %w", err)
	}

	if u.ID == "" && u.Name == "" && u.Email == "" {
		return fmt.Errorf("at least one of id, name, or email is required to identify the user")
	}

	return nil
}

// SetDefaults applies default values (none for team users)
func (u *OrganizationTeamUserResource) SetDefaults() {}

// GetKonnectID returns the resolved Konnect ID if available
func (u OrganizationTeamUserResource) GetKonnectID() string {
	return u.konnectID
}

// SetKonnectID sets the resolved Konnect ID
func (u *OrganizationTeamUserResource) SetKonnectID(id string) {
	u.konnectID = id
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (u OrganizationTeamUserResource) GetKonnectMonikerFilter() string {
	return ""
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource by user ID
func (u *OrganizationTeamUserResource) TryMatchKonnectResource(_ any) bool {
	return false
}

// GetParentRef returns the parent team reference
func (u OrganizationTeamUserResource) GetParentRef() *ResourceRef {
	if u.Team != "" {
		return &ResourceRef{Kind: string(ResourceTypeOrganizationTeam), Ref: u.Team}
	}
	return nil
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (u OrganizationTeamUserResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{}
}
