package resources

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/tags"
)

// PortalTeamRoleResource represents an assigned role on a portal team
// This is a child resource (no kongctl metadata support).
type PortalTeamRoleResource struct {
	Ref string `yaml:"ref" json:"ref"`

	// Parent references
	Portal string `yaml:"portal,omitempty" json:"portal,omitempty"`
	Team   string `yaml:"team,omitempty"   json:"team,omitempty"`

	// Role assignment fields (API expects literal values)
	RoleName       string `yaml:"role_name"        json:"role_name"`
	EntityID       string `yaml:"entity_id"        json:"entity_id"`
	EntityTypeName string `yaml:"entity_type_name" json:"entity_type_name"`
	EntityRegion   string `yaml:"entity_region"    json:"entity_region"`

	// Resolved Konnect assignment ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetType returns the resource type
func (r PortalTeamRoleResource) GetType() ResourceType {
	return ResourceTypePortalTeamRole
}

// GetRef returns the reference identifier
func (r PortalTeamRoleResource) GetRef() string {
	return r.Ref
}

// GetMoniker returns a human-readable moniker for matching purposes
func (r PortalTeamRoleResource) GetMoniker() string {
	return fmt.Sprintf("%s:%s:%s:%s", r.RoleName, r.EntityID, r.EntityTypeName, r.EntityRegion)
}

// GetDependencies returns references to other resources this assignment depends on
func (r PortalTeamRoleResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}

	if r.Portal != "" {
		deps = append(deps, ResourceRef{
			Kind: "portal",
			Ref:  r.Portal,
		})
	}

	if r.Team != "" {
		deps = append(deps, ResourceRef{
			Kind: "portal_team",
			Ref:  r.Team,
		})
	}

	if tags.IsRefPlaceholder(r.EntityID) {
		if ref, _, ok := tags.ParseRefPlaceholder(r.EntityID); ok && ref != "" {
			deps = append(deps, ResourceRef{
				Kind: "api",
				Ref:  ref,
			})
		}
	}

	// EntityID may be a !ref to another resource; we rely on loader reference
	// resolution to inject reference metadata rather than explicit dependency here.
	return deps
}

// Validate ensures the portal team role resource is valid
func (r PortalTeamRoleResource) Validate() error {
	if err := ValidateRef(r.Ref); err != nil {
		return fmt.Errorf("invalid portal team role ref: %w", err)
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

// SetDefaults applies default values (none for team roles)
func (r *PortalTeamRoleResource) SetDefaults() {}

// GetKonnectID returns the resolved Konnect ID if available
func (r PortalTeamRoleResource) GetKonnectID() string {
	return r.konnectID
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (r PortalTeamRoleResource) GetKonnectMonikerFilter() string {
	// Assigned roles list endpoint does not support filtering server-side
	return ""
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (r *PortalTeamRoleResource) TryMatchKonnectResource(konnectResource any) bool {
	role, ok := konnectResource.(map[string]any)
	if !ok {
		return false
	}

	roleName, _ := role["role_name"].(string)
	entityID, _ := role["entity_id"].(string)
	entityTypeName, _ := role["entity_type_name"].(string)
	entityRegion, _ := role["entity_region"].(string)

	// Normalize case for region comparisons
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

// GetParentRef returns the immediate parent reference (portal team)
func (r PortalTeamRoleResource) GetParentRef() *ResourceRef {
	if r.Team != "" {
		return &ResourceRef{Kind: "portal_team", Ref: r.Team}
	}
	return nil
}

// UnmarshalJSON rejects kongctl metadata on child resources
func (r *PortalTeamRoleResource) UnmarshalJSON(data []byte) error {
	var temp struct {
		Ref            string `json:"ref"`
		Portal         string `json:"portal,omitempty"`
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
		return fmt.Errorf("kongctl metadata not supported on child resources (portal team roles)")
	}

	r.Ref = temp.Ref
	r.Portal = temp.Portal
	r.Team = temp.Team
	r.RoleName = temp.RoleName
	r.EntityID = temp.EntityID
	r.EntityTypeName = temp.EntityTypeName
	r.EntityRegion = temp.EntityRegion

	return nil
}
