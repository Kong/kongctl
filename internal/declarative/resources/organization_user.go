package resources

import (
	"fmt"
)

func init() {
	registerResourceType(
		ResourceTypeOrganizationUserTeamMembership,
		func(rs *ResourceSet) *[]OrganizationUserTeamMembershipResource {
			return &rs.OrganizationUserTeamMemberships
		},
		AutoExplain[OrganizationUserTeamMembershipResource](
			WithExplainRecommendedFields(SchemaFieldUser),
		),
	)
	registerResourceType(
		ResourceTypeOrganizationUserRole,
		func(rs *ResourceSet) *[]OrganizationUserRoleResource { return &rs.OrganizationUserRoles },
		AutoExplain[OrganizationUserRoleResource](
			WithExplainRecommendedFields(SchemaFieldUser),
		),
	)
}

// OrganizationUserResource selects an existing Konnect user and declares user-bound assignments.
type OrganizationUserResource struct {
	Ref     string                                   `yaml:"ref"               json:"ref"`
	Email   string                                   `yaml:"email,omitempty"   json:"email,omitempty"`
	ID      string                                   `yaml:"id,omitempty"      json:"id,omitempty"`
	Kongctl *KongctlMeta                             `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
	Teams   []OrganizationUserTeamMembershipResource `yaml:"teams,omitempty"   json:"teams,omitempty"`
	Roles   []OrganizationUserRoleResource           `yaml:"roles,omitempty"   json:"roles,omitempty"`

	konnectID string `yaml:"-" json:"-"`
}

func (u OrganizationUserResource) GetRef() string {
	return u.Ref
}

func (u OrganizationUserResource) Validate() error {
	if err := ValidateRef(u.Ref); err != nil {
		return fmt.Errorf("invalid organization user ref: %w", err)
	}
	if (u.Email == "") == (u.ID == "") {
		return fmt.Errorf("exactly one of email or id is required")
	}
	for i, team := range u.Teams {
		if err := team.ValidateNested(); err != nil {
			return fmt.Errorf("invalid user team membership %d: %w", i, err)
		}
	}
	for i, role := range u.Roles {
		if err := role.Validate(); err != nil {
			return fmt.Errorf("invalid user role %d: %w", i, err)
		}
	}
	return nil
}

func (u OrganizationUserResource) GetKonnectID() string {
	return u.konnectID
}

func (u *OrganizationUserResource) SetKonnectID(id string) {
	u.konnectID = id
}

// OrganizationUserTeamMembershipResource represents an organization user's team assignment.
type OrganizationUserTeamMembershipResource struct {
	Ref  string `yaml:"ref"  json:"ref"`
	User string `yaml:"user,omitempty" json:"user,omitempty"`
	Team string `yaml:"team" json:"team"`
}

func (r OrganizationUserTeamMembershipResource) GetType() ResourceType {
	return ResourceTypeOrganizationUserTeamMembership
}

func (r OrganizationUserTeamMembershipResource) GetRef() string {
	return r.Ref
}

func (r OrganizationUserTeamMembershipResource) GetMoniker() string {
	return r.Ref
}

func (r OrganizationUserTeamMembershipResource) GetDependencies() []ResourceRef {
	return []ResourceRef{
		{Kind: ResourceTypeOrganizationTeam, Ref: r.Team},
	}
}

func (r OrganizationUserTeamMembershipResource) Validate() error {
	if err := r.ValidateNested(); err != nil {
		return err
	}
	if r.User == "" {
		return fmt.Errorf("user is required")
	}
	return nil
}

func (r OrganizationUserTeamMembershipResource) ValidateNested() error {
	if err := ValidateRef(r.Ref); err != nil {
		return fmt.Errorf("invalid organization user team membership ref: %w", err)
	}
	if r.Team == "" {
		return fmt.Errorf("team is required")
	}
	return nil
}

func (r *OrganizationUserTeamMembershipResource) SetDefaults() {}

func (r OrganizationUserTeamMembershipResource) GetKonnectID() string {
	return ""
}

func (r OrganizationUserTeamMembershipResource) GetKonnectMonikerFilter() string {
	return ""
}

func (r *OrganizationUserTeamMembershipResource) TryMatchKonnectResource(_ any) bool {
	return false
}

// OrganizationUserRoleResource represents an assigned role on an organization user.
type OrganizationUserRoleResource struct {
	Ref string `yaml:"ref" json:"ref"`

	User string `yaml:"user,omitempty" json:"user,omitempty"`

	RoleName       string `yaml:"role_name"        json:"role_name"`
	EntityID       string `yaml:"entity_id"        json:"entity_id"`
	EntityTypeName string `yaml:"entity_type_name" json:"entity_type_name"`
	EntityRegion   string `yaml:"entity_region"    json:"entity_region"`
}

func (r OrganizationUserRoleResource) GetType() ResourceType {
	return ResourceTypeOrganizationUserRole
}

func (r OrganizationUserRoleResource) GetRef() string {
	return r.Ref
}

func (r OrganizationUserRoleResource) GetMoniker() string {
	return fmt.Sprintf("%s:%s:%s:%s", r.RoleName, r.EntityID, r.EntityTypeName, r.EntityRegion)
}

func (r OrganizationUserRoleResource) GetDependencies() []ResourceRef {
	return roleEntityDependency(r.EntityID, r.EntityTypeName)
}

func (r OrganizationUserRoleResource) Validate() error {
	if err := ValidateRef(r.Ref); err != nil {
		return fmt.Errorf("invalid organization user role ref: %w", err)
	}
	if r.RoleName == "" {
		return fmt.Errorf("role_name is required")
	}
	if r.EntityID == "" {
		return fmt.Errorf("entity_id is required")
	}
	if r.EntityTypeName == "" {
		return fmt.Errorf("entity_type_name is required")
	}
	if r.EntityRegion == "" {
		return fmt.Errorf("entity_region is required")
	}
	return nil
}

func (r *OrganizationUserRoleResource) SetDefaults() {}

func (r OrganizationUserRoleResource) GetKonnectID() string {
	return ""
}

func (r OrganizationUserRoleResource) GetKonnectMonikerFilter() string {
	return ""
}

func (r *OrganizationUserRoleResource) TryMatchKonnectResource(_ any) bool {
	return false
}
