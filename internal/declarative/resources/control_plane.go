package resources

import (
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeControlPlane,
		func(rs *ResourceSet) *[]ControlPlaneResource { return &rs.ControlPlanes },
	)
}

// ControlPlaneGroupMember represents a member entry for a control plane group.
type ControlPlaneGroupMember struct {
	ID string `yaml:"id" json:"id"`
}

// ControlPlaneResource represents a control plane in declarative configuration
type ControlPlaneResource struct {
	BaseResource
	kkComps.CreateControlPlaneRequest `                          yaml:",inline"                    json:",inline"`
	External                          *ExternalBlock            `yaml:"_external,omitempty"        json:"_external,omitempty"` //nolint:lll
	Deck                              *DeckConfig               `yaml:"_deck,omitempty"            json:"_deck,omitempty"`
	GatewayServices                   []GatewayServiceResource  `yaml:"gateway_services,omitempty" json:"gateway_services,omitempty"` //nolint:lll
	Members                           []ControlPlaneGroupMember `yaml:"members,omitempty"          json:"members,omitempty"`          //nolint:lll

	deckBaseDir string `yaml:"-" json:"-"`
}

// DeckBaseDir returns the resolved base directory for deck config (if any).
func (c ControlPlaneResource) DeckBaseDir() string {
	return c.deckBaseDir
}

// SetDeckBaseDir sets the resolved base directory for deck config.
func (c *ControlPlaneResource) SetDeckBaseDir(dir string) {
	c.deckBaseDir = strings.TrimSpace(dir)
}

// HasDeckConfig returns true when a deck config is configured on the control plane.
func (c ControlPlaneResource) HasDeckConfig() bool {
	return c.Deck != nil
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (c ControlPlaneResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{} // No outbound references
}

// Validate ensures the control plane resource is valid
func (c ControlPlaneResource) Validate() error {
	if err := ValidateRef(c.Ref); err != nil {
		return fmt.Errorf("invalid control plane ref: %w", err)
	}

	if len(c.GatewayServices) > 0 && c.IsGroup() {
		return fmt.Errorf("control plane group %q cannot define gateway_services", c.Ref)
	}

	if len(c.Members) > 0 && !c.IsGroup() {
		return fmt.Errorf("control plane %q: members are only supported when cluster_type is %q",
			c.Ref, kkComps.CreateControlPlaneRequestClusterTypeClusterTypeControlPlaneGroup)
	}

	seenMemberIDs := make(map[string]struct{})
	for idx, member := range c.Members {
		memberID := strings.TrimSpace(member.ID)
		if memberID == "" {
			return fmt.Errorf("control plane group %q member at index %d: id cannot be empty", c.Ref, idx)
		}
		if _, exists := seenMemberIDs[memberID]; exists {
			return fmt.Errorf("control plane group %q contains duplicate member id %q", c.Ref, memberID)
		}
		seenMemberIDs[memberID] = struct{}{}
	}

	if c.External != nil {
		if err := c.External.Validate(); err != nil {
			return fmt.Errorf("invalid _external block: %w", err)
		}
	}

	if c.Deck != nil {
		if err := c.Deck.Validate(); err != nil {
			return fmt.Errorf("control plane %q: invalid _deck config: %w", c.Ref, err)
		}
	}
	return nil
}

// SetDefaults applies default values to control plane resource
func (c *ControlPlaneResource) SetDefaults() {
	// If Name is not set, use ref as default
	if c.Name == "" {
		c.Name = c.Ref
	}

	for i := range c.GatewayServices {
		c.GatewayServices[i].SetDefaults()
	}
}

// GetType returns the resource type
func (c ControlPlaneResource) GetType() ResourceType {
	return ResourceTypeControlPlane
}

// GetMoniker returns the resource moniker (for control planes, this is the name)
func (c ControlPlaneResource) GetMoniker() string {
	return c.Name
}

// GetDependencies returns references to other resources this control plane depends on
func (c ControlPlaneResource) GetDependencies() []ResourceRef {
	// Control planes don't depend on other resources
	return []ResourceRef{}
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (c ControlPlaneResource) GetKonnectMonikerFilter() string {
	if c.IsExternal() {
		return ""
	}
	return c.BaseResource.GetKonnectMonikerFilter(c.Name)
}

// IsExternal returns true if this control plane is externally managed
func (c *ControlPlaneResource) IsExternal() bool {
	return c.External != nil && c.External.IsExternal()
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (c *ControlPlaneResource) TryMatchKonnectResource(konnectResource any) bool {
	id, ok := tryMatchByNameWithExternal(c.Name, konnectResource, matchOptions{
		sdkType: "ControlPlane",
	}, c.External)

	if ok {
		c.SetKonnectID(id)
	}
	return ok
}

// IsGroup returns true when the control plane represents a control plane group.
func (c *ControlPlaneResource) IsGroup() bool {
	if c == nil || c.ClusterType == nil {
		return false
	}
	return *c.ClusterType == kkComps.CreateControlPlaneRequestClusterTypeClusterTypeControlPlaneGroup
}

// MemberIDs returns the list of member IDs declared for a control plane group.
func (c *ControlPlaneResource) MemberIDs() []string {
	if c == nil || len(c.Members) == 0 {
		return nil
	}

	memberIDs := make([]string, 0, len(c.Members))
	for _, member := range c.Members {
		if member.ID == "" {
			continue
		}
		memberIDs = append(memberIDs, member.ID)
	}
	return memberIDs
}
