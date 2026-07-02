package resources

import (
	"encoding/json"
	"fmt"
	"strings"
)

func init() {
	registerResourceType(
		ResourceTypeOrganizationTeamRole,
		func(rs *ResourceSet) *[]OrganizationTeamRoleResource { return &rs.OrganizationTeamRoles },
		AutoExplain[OrganizationTeamRoleResource](
			WithExplainRecommendedFields("team"),
		),
	)
}

// OrganizationTeamRoleResource represents an assigned role on an organization team.
// This is a child resource (no kongctl metadata support).
type OrganizationTeamRoleResource struct {
	Ref string `yaml:"ref" json:"ref"`

	// Parent team reference.
	Team string `yaml:"team,omitempty" json:"team,omitempty"`

	RoleName       string `yaml:"role_name"        json:"role_name"`
	EntityID       string `yaml:"entity_id"        json:"entity_id"`
	EntityTypeName string `yaml:"entity_type_name" json:"entity_type_name"`
	EntityRegion   string `yaml:"entity_region"    json:"entity_region"`

	konnectID string `yaml:"-" json:"-"`
}

func (r OrganizationTeamRoleResource) MarshalJSON() ([]byte, error) {
	type alias struct {
		Ref            string `json:"ref"`
		Team           string `json:"team,omitempty"`
		RoleName       string `json:"role_name"`
		EntityID       string `json:"entity_id"`
		EntityTypeName string `json:"entity_type_name"`
		EntityRegion   string `json:"entity_region"`
	}

	return json.Marshal(alias{
		Ref:            r.Ref,
		Team:           r.Team,
		RoleName:       r.RoleName,
		EntityID:       r.EntityID,
		EntityTypeName: r.EntityTypeName,
		EntityRegion:   r.EntityRegion,
	})
}

func (r OrganizationTeamRoleResource) MarshalYAML() (any, error) {
	type alias struct {
		Ref            string `json:"ref"              yaml:"ref"`
		Team           string `json:"team,omitempty"   yaml:"team,omitempty"`
		RoleName       string `json:"role_name"        yaml:"role_name"`
		EntityID       string `json:"entity_id"        yaml:"entity_id"`
		EntityTypeName string `json:"entity_type_name" yaml:"entity_type_name"`
		EntityRegion   string `json:"entity_region"    yaml:"entity_region"`
	}

	return alias{
		Ref:            r.Ref,
		Team:           r.Team,
		RoleName:       r.RoleName,
		EntityID:       r.EntityID,
		EntityTypeName: r.EntityTypeName,
		EntityRegion:   r.EntityRegion,
	}, nil
}

func (r OrganizationTeamRoleResource) GetType() ResourceType {
	return ResourceTypeOrganizationTeamRole
}

func (r OrganizationTeamRoleResource) GetRef() string {
	return r.Ref
}

func (r OrganizationTeamRoleResource) GetMoniker() string {
	return fmt.Sprintf("%s:%s:%s:%s", r.RoleName, r.EntityID, r.EntityTypeName, r.EntityRegion)
}

func (r OrganizationTeamRoleResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}

	if r.Team != "" {
		deps = append(deps, ResourceRef{
			Kind: ResourceTypeOrganizationTeam,
			Ref:  r.Team,
		})
	}

	deps = append(deps, roleEntityDependency(r.EntityID, r.EntityTypeName)...)

	return deps
}

func (r OrganizationTeamRoleResource) Validate() error {
	if err := ValidateRef(r.Ref); err != nil {
		return fmt.Errorf("invalid organization team role ref: %w", err)
	}
	if r.Team == "" {
		return fmt.Errorf("team is required")
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

func (r *OrganizationTeamRoleResource) SetDefaults() {}

func (r OrganizationTeamRoleResource) GetKonnectID() string {
	return r.konnectID
}

func (r OrganizationTeamRoleResource) GetKonnectMonikerFilter() string {
	return ""
}

func (r *OrganizationTeamRoleResource) TryMatchKonnectResource(konnectResource any) bool {
	role, ok := konnectResource.(map[string]any)
	if !ok {
		return false
	}

	roleName, _ := role["role_name"].(string)
	entityID, _ := role["entity_id"].(string)
	entityTypeName, _ := role["entity_type_name"].(string)
	entityRegion, _ := role["entity_region"].(string)

	if roleName == r.RoleName &&
		entityID == r.EntityID &&
		entityTypeName == r.EntityTypeName &&
		strings.EqualFold(entityRegion, r.EntityRegion) {
		if id, ok := role["id"].(string); ok {
			r.konnectID = id
		}
		return true
	}

	return false
}

func (r OrganizationTeamRoleResource) GetParentRef() *ResourceRef {
	if r.Team != "" {
		return &ResourceRef{Kind: ResourceTypeOrganizationTeam, Ref: r.Team}
	}
	return nil
}

func (r *OrganizationTeamRoleResource) UnmarshalJSON(data []byte) error {
	var temp struct {
		Ref            string `json:"ref"`
		Team           string `json:"team,omitempty"`
		RoleName       string `json:"role_name"`
		EntityID       string `json:"entity_id"`
		EntityTypeName string `json:"entity_type_name"`
		EntityRegion   string `json:"entity_region"`
		Kongctl        any    `json:"kongctl,omitempty"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	if temp.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on child resources (organization team roles)")
	}

	r.Ref = temp.Ref
	r.Team = temp.Team
	r.RoleName = temp.RoleName
	r.EntityID = temp.EntityID
	r.EntityTypeName = temp.EntityTypeName
	r.EntityRegion = temp.EntityRegion

	return nil
}
