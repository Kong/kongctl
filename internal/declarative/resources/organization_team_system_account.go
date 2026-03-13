package resources

import (
	"fmt"
)

func init() {
	registerResourceType(
		ResourceTypeOrganizationTeamSystemAccount,
		func(rs *ResourceSet) *[]OrganizationTeamSystemAccountResource {
			return &rs.OrganizationTeamSystemAccounts
		},
	)
}

// OrganizationTeamSystemAccountResource represents a system account member of an organization team.
// This is a child resource of OrganizationTeamResource.
type OrganizationTeamSystemAccountResource struct {
	Ref string `yaml:"ref" json:"ref"`

	// Team is the parent team reference
	Team string `yaml:"team,omitempty" json:"team,omitempty"`

	// Identity lookup fields — specify at least one
	ID   string `yaml:"id,omitempty"   json:"id,omitempty"`
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Resolved Konnect system account ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetType returns the resource type
func (s OrganizationTeamSystemAccountResource) GetType() ResourceType {
	return ResourceTypeOrganizationTeamSystemAccount
}

// GetRef returns the reference identifier
func (s OrganizationTeamSystemAccountResource) GetRef() string {
	return s.Ref
}

// GetMoniker returns a human-readable moniker
func (s OrganizationTeamSystemAccountResource) GetMoniker() string {
	if s.Name != "" {
		return s.Name
	}
	return s.ID
}

// GetDependencies returns references to other resources this system account depends on
func (s OrganizationTeamSystemAccountResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if s.Team != "" {
		deps = append(deps, ResourceRef{
			Kind: string(ResourceTypeOrganizationTeam),
			Ref:  s.Team,
		})
	}
	return deps
}

// Validate ensures the team system account resource is valid
func (s OrganizationTeamSystemAccountResource) Validate() error {
	if err := ValidateRef(s.Ref); err != nil {
		return fmt.Errorf("invalid team system account ref: %w", err)
	}

	if s.ID == "" && s.Name == "" {
		return fmt.Errorf("at least one of id or name is required to identify the system account")
	}

	return nil
}

// SetDefaults applies default values (none for team system accounts)
func (s *OrganizationTeamSystemAccountResource) SetDefaults() {}

// GetKonnectID returns the resolved Konnect ID if available
func (s OrganizationTeamSystemAccountResource) GetKonnectID() string {
	return s.konnectID
}

// SetKonnectID sets the resolved Konnect ID
func (s *OrganizationTeamSystemAccountResource) SetKonnectID(id string) {
	s.konnectID = id
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (s OrganizationTeamSystemAccountResource) GetKonnectMonikerFilter() string {
	return ""
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (s *OrganizationTeamSystemAccountResource) TryMatchKonnectResource(_ any) bool {
	return false
}

// GetParentRef returns the parent team reference
func (s OrganizationTeamSystemAccountResource) GetParentRef() *ResourceRef {
	if s.Team != "" {
		return &ResourceRef{Kind: string(ResourceTypeOrganizationTeam), Ref: s.Team}
	}
	return nil
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (s OrganizationTeamSystemAccountResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{}
}
