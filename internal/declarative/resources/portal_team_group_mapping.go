package resources

import (
	"encoding/json"
	"fmt"
)

func init() {
	registerResourceType(
		ResourceTypePortalTeamGroupMapping,
		func(rs *ResourceSet) *[]PortalTeamGroupMappingResource { return &rs.PortalTeamGroupMappings },
		AutoExplain[PortalTeamGroupMappingResource](),
	)
}

// PortalTeamGroupMappingResource maps a portal team to IdP groups.
type PortalTeamGroupMappingResource struct {
	Ref    string   `yaml:"ref"              json:"ref"`
	Portal string   `yaml:"portal,omitempty" json:"portal,omitempty"`
	Team   string   `yaml:"team"             json:"team"`
	Groups []string `yaml:"groups"           json:"groups"`
}

// MarshalJSON keeps kongctl metadata fields stable.
func (p PortalTeamGroupMappingResource) MarshalJSON() ([]byte, error) {
	type alias PortalTeamGroupMappingResource
	return json.Marshal(alias(p))
}

// GetType returns the resource type.
func (p PortalTeamGroupMappingResource) GetType() ResourceType {
	return ResourceTypePortalTeamGroupMapping
}

// GetRef returns the reference identifier.
func (p PortalTeamGroupMappingResource) GetRef() string {
	return p.Ref
}

// GetMoniker returns the resource moniker.
func (p PortalTeamGroupMappingResource) GetMoniker() string {
	return p.Team
}

// GetDependencies returns parent and team dependencies.
func (p PortalTeamGroupMappingResource) GetDependencies() []ResourceRef {
	var deps []ResourceRef
	if p.Portal != "" {
		deps = append(deps, ResourceRef{Kind: ResourceTypePortal, Ref: p.Portal})
	}
	if p.Team != "" {
		deps = append(deps, ResourceRef{Kind: ResourceTypePortalTeam, Ref: p.Team})
	}
	return deps
}

// Validate ensures the mapping has the required declarative identifiers.
func (p PortalTeamGroupMappingResource) Validate() error {
	if err := ValidateRef(p.Ref); err != nil {
		return fmt.Errorf("invalid portal team group mapping ref: %w", err)
	}
	if p.Groups == nil {
		return fmt.Errorf("groups is required")
	}

	seen := make(map[string]struct{}, len(p.Groups))
	for _, group := range p.Groups {
		if _, ok := seen[group]; ok {
			return fmt.Errorf("duplicate group name %q", group)
		}
		seen[group] = struct{}{}
	}

	return nil
}

// SetDefaults applies default values to the resource.
func (p *PortalTeamGroupMappingResource) SetDefaults() {}

// GetKonnectID returns the resolved Konnect ID if available.
func (p PortalTeamGroupMappingResource) GetKonnectID() string {
	return ""
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup.
func (p PortalTeamGroupMappingResource) GetKonnectMonikerFilter() string {
	return ""
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource.
func (p *PortalTeamGroupMappingResource) TryMatchKonnectResource(_ any) bool {
	return false
}

// GetParentRef implements ResourceWithParent.
func (p PortalTeamGroupMappingResource) GetParentRef() *ResourceRef {
	if p.Portal == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypePortal, Ref: p.Portal}
}

// UnmarshalJSON rejects unsupported metadata on child resources.
func (p *PortalTeamGroupMappingResource) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	allowedKeys := map[string]struct{}{
		"ref":     {},
		"portal":  {},
		"team":    {},
		"groups":  {},
		"kongctl": {},
	}
	for key := range raw {
		if _, ok := allowedKeys[key]; !ok {
			return fmt.Errorf("json: unknown field %q", key)
		}
	}

	if v, ok := raw["kongctl"]; ok {
		var kongctl any
		if err := json.Unmarshal(v, &kongctl); err != nil {
			return err
		}
		if kongctl != nil {
			return fmt.Errorf("kongctl metadata not supported on portal team group mappings")
		}
	}
	type alias PortalTeamGroupMappingResource
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*p = PortalTeamGroupMappingResource(decoded)
	return nil
}
