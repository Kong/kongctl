package resources

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/tags"
)

func init() {
	registerResourceType(
		ResourceTypeOrganizationSystemAccountTeamMembership,
		func(rs *ResourceSet) *[]OrganizationSystemAccountTeamMembershipResource {
			return &rs.OrganizationSystemAccountTeamMemberships
		},
		AutoExplain[OrganizationSystemAccountTeamMembershipResource](),
	)
	registerResourceType(
		ResourceTypeOrganizationSystemAccountRole,
		func(rs *ResourceSet) *[]OrganizationSystemAccountRoleResource {
			return &rs.OrganizationSystemAccountRoles
		},
		AutoExplain[OrganizationSystemAccountRoleResource](),
	)
}

// OrganizationSystemAccountResource selects an existing Konnect system account and declares assignments.
type OrganizationSystemAccountResource struct {
	Ref     string                                            `yaml:"ref"               json:"ref"`
	Name    string                                            `yaml:"name,omitempty"    json:"name,omitempty"`
	ID      string                                            `yaml:"id,omitempty"      json:"id,omitempty"`
	Kongctl *KongctlMeta                                      `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
	Teams   []OrganizationSystemAccountTeamMembershipResource `yaml:"teams,omitempty"   json:"teams,omitempty"`
	Roles   []OrganizationSystemAccountRoleResource           `yaml:"roles,omitempty"   json:"roles,omitempty"`

	konnectID string `yaml:"-" json:"-"`
}

func (s OrganizationSystemAccountResource) GetRef() string {
	return s.Ref
}

func (s OrganizationSystemAccountResource) Validate() error {
	if err := ValidateRef(s.Ref); err != nil {
		return fmt.Errorf("invalid organization system account ref: %w", err)
	}
	if (s.Name == "") == (s.ID == "") {
		return fmt.Errorf("exactly one of name or id is required")
	}
	for i, team := range s.Teams {
		if err := team.ValidateNested(); err != nil {
			return fmt.Errorf("invalid system account team membership %d: %w", i, err)
		}
	}
	for i, role := range s.Roles {
		if err := role.Validate(); err != nil {
			return fmt.Errorf("invalid system account role %d: %w", i, err)
		}
	}
	return nil
}

func (s OrganizationSystemAccountResource) GetKonnectID() string {
	return s.konnectID
}

func (s *OrganizationSystemAccountResource) SetKonnectID(id string) {
	s.konnectID = id
}

// OrganizationSystemAccountTeamMembershipResource is an internal relation resource.
type OrganizationSystemAccountTeamMembershipResource struct {
	Ref           string `yaml:"ref"  json:"ref"`
	SystemAccount string `yaml:"-"    json:"-"`
	Team          string `yaml:"team" json:"team"`
}

func (r OrganizationSystemAccountTeamMembershipResource) GetType() ResourceType {
	return ResourceTypeOrganizationSystemAccountTeamMembership
}

func (r OrganizationSystemAccountTeamMembershipResource) GetRef() string {
	return r.Ref
}

func (r OrganizationSystemAccountTeamMembershipResource) GetMoniker() string {
	return r.Ref
}

func (r OrganizationSystemAccountTeamMembershipResource) GetDependencies() []ResourceRef {
	return []ResourceRef{{Kind: ResourceTypeOrganizationTeam, Ref: r.Team}}
}

func (r OrganizationSystemAccountTeamMembershipResource) Validate() error {
	if err := r.ValidateNested(); err != nil {
		return err
	}
	if r.SystemAccount == "" {
		return fmt.Errorf("system account is required")
	}
	return nil
}

func (r OrganizationSystemAccountTeamMembershipResource) ValidateNested() error {
	if err := ValidateRef(r.Ref); err != nil {
		return fmt.Errorf("invalid organization system account team membership ref: %w", err)
	}
	if r.Team == "" {
		return fmt.Errorf("team is required")
	}
	return nil
}

func (r *OrganizationSystemAccountTeamMembershipResource) SetDefaults() {}

func (r OrganizationSystemAccountTeamMembershipResource) GetKonnectID() string {
	return ""
}

func (r OrganizationSystemAccountTeamMembershipResource) GetKonnectMonikerFilter() string {
	return ""
}

func (r *OrganizationSystemAccountTeamMembershipResource) TryMatchKonnectResource(_ any) bool {
	return false
}

// OrganizationSystemAccountRoleResource represents an assigned role on a system account.
type OrganizationSystemAccountRoleResource struct {
	Ref string `yaml:"ref" json:"ref"`

	SystemAccount string `yaml:"-" json:"-"`

	RoleName       string `yaml:"role_name"        json:"role_name"`
	EntityID       string `yaml:"entity_id"        json:"entity_id"`
	EntityTypeName string `yaml:"entity_type_name" json:"entity_type_name"`
	EntityRegion   string `yaml:"entity_region"    json:"entity_region"`
}

func (r OrganizationSystemAccountRoleResource) GetType() ResourceType {
	return ResourceTypeOrganizationSystemAccountRole
}

func (r OrganizationSystemAccountRoleResource) GetRef() string {
	return r.Ref
}

func (r OrganizationSystemAccountRoleResource) GetMoniker() string {
	return fmt.Sprintf("%s:%s:%s:%s", r.RoleName, r.EntityID, r.EntityTypeName, r.EntityRegion)
}

func (r OrganizationSystemAccountRoleResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if tags.IsRefPlaceholder(r.EntityID) {
		if ref, _, ok := tags.ParseRefPlaceholder(r.EntityID); ok && ref != "" {
			deps = append(deps, ResourceRef{Kind: ResourceTypeAPI, Ref: ref})
		}
	}
	return deps
}

func (r OrganizationSystemAccountRoleResource) Validate() error {
	if err := ValidateRef(r.Ref); err != nil {
		return fmt.Errorf("invalid organization system account role ref: %w", err)
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

func (r *OrganizationSystemAccountRoleResource) SetDefaults() {}

func (r OrganizationSystemAccountRoleResource) GetKonnectID() string {
	return ""
}

func (r OrganizationSystemAccountRoleResource) GetKonnectMonikerFilter() string {
	return ""
}

func (r *OrganizationSystemAccountRoleResource) TryMatchKonnectResource(_ any) bool {
	return false
}
