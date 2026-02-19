package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypePortalTeam,
		func(rs *ResourceSet) *[]PortalTeamResource { return &rs.PortalTeams },
	)
}

// PortalTeamResource represents a portal team (developer team)
type PortalTeamResource struct {
	kkComps.PortalCreateTeamRequest `       yaml:",inline"          json:",inline"`
	Ref                             string `yaml:"ref"              json:"ref"`
	// Parent portal reference
	Portal string `yaml:"portal,omitempty" json:"portal,omitempty"`

	// Child resources
	Roles []PortalTeamRoleResource `yaml:"roles,omitempty" json:"roles,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetType returns the resource type
func (p PortalTeamResource) GetType() ResourceType {
	return ResourceTypePortalTeam
}

// GetRef returns the reference identifier
func (p PortalTeamResource) GetRef() string {
	return p.Ref
}

// GetMoniker returns the resource moniker (for portal teams, this is the name)
func (p PortalTeamResource) GetMoniker() string {
	return p.Name
}

// GetDependencies returns references to other resources this team depends on
func (p PortalTeamResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if p.Portal != "" {
		deps = append(deps, ResourceRef{
			Kind: "portal",
			Ref:  p.Portal,
		})
	}
	return deps
}

// Validate ensures the portal team resource is valid
func (p PortalTeamResource) Validate() error {
	if err := ValidateRef(p.Ref); err != nil {
		return fmt.Errorf("invalid team ref: %w", err)
	}

	if p.Name == "" {
		return fmt.Errorf("team name is required")
	}

	roleRefs := make(map[string]bool)
	for i, role := range p.Roles {
		if err := role.Validate(); err != nil {
			return fmt.Errorf("invalid team role %d: %w", i, err)
		}
		if roleRefs[role.GetRef()] {
			return fmt.Errorf("duplicate team role ref: %s", role.GetRef())
		}
		roleRefs[role.GetRef()] = true
	}

	return nil
}

// SetDefaults applies default values to portal team resource
func (p *PortalTeamResource) SetDefaults() {
	// No defaults to apply for portal teams
	for i := range p.Roles {
		p.Roles[i].SetDefaults()
	}
}

// GetKonnectID returns the resolved Konnect ID if available
func (p PortalTeamResource) GetKonnectID() string {
	return p.konnectID
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (p PortalTeamResource) GetKonnectMonikerFilter() string {
	if p.Name == "" {
		return ""
	}
	// Note: Portal teams API doesn't support filtering by name,
	// so we return empty and will filter in code
	return ""
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (p *PortalTeamResource) TryMatchKonnectResource(konnectResource any) bool {
	if id := tryMatchByField(konnectResource, "Name", p.Name); id != "" {
		p.konnectID = id
		return true
	}
	return false
}

// GetParentRef returns the parent portal reference
func (p PortalTeamResource) GetParentRef() *ResourceRef {
	if p.Portal != "" {
		return &ResourceRef{Kind: "portal", Ref: p.Portal}
	}
	return nil
}

// UnmarshalJSON custom unmarshaling to reject kongctl metadata on child resources
func (p *PortalTeamResource) UnmarshalJSON(data []byte) error {
	var temp struct {
		Ref         string                   `json:"ref"`
		Portal      string                   `json:"portal,omitempty"`
		Name        string                   `json:"name"`
		Description *string                  `json:"description,omitempty"`
		Roles       []PortalTeamRoleResource `json:"roles,omitempty"`
		Kongctl     any                      `json:"kongctl,omitempty"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	if temp.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on child resources (portal teams)")
	}

	p.Ref = temp.Ref
	p.Portal = temp.Portal
	p.Name = temp.Name
	p.Description = temp.Description
	p.Roles = temp.Roles

	return nil
}
