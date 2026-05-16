package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeOrganizationTeam,
		func(rs *ResourceSet) *[]OrganizationTeamResource { return &rs.OrganizationTeams },
		AutoExplain[OrganizationTeamResource](),
	)
}

// OrganizationTeamResource represents a team in declarative configuration
type OrganizationTeamResource struct {
	BaseResource
	kkComps.CreateTeam `                               yaml:",inline"             json:",inline"`
	External           *ExternalBlock                 `yaml:"_external,omitempty" json:"_external,omitempty"`
	Roles              []OrganizationTeamRoleResource `yaml:"roles,omitempty"     json:"roles,omitempty"`
}

func (t OrganizationTeamResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.organizationTeamAlias())
}

func (t OrganizationTeamResource) MarshalYAML() (any, error) {
	return t.organizationTeamAlias(), nil
}

type organizationTeamAlias struct {
	Ref         string                         `json:"ref"                   yaml:"ref"`
	Kongctl     *KongctlMeta                   `json:"kongctl,omitempty"     yaml:"kongctl,omitempty"`
	External    *ExternalBlock                 `json:"_external,omitempty"   yaml:"_external,omitempty"`
	Name        string                         `json:"name"                  yaml:"name"`
	Description *string                        `json:"description,omitempty" yaml:"description,omitempty"`
	Labels      map[string]string              `json:"labels,omitempty"      yaml:"labels,omitempty"`
	Roles       []OrganizationTeamRoleResource `json:"roles,omitempty"       yaml:"roles,omitempty"`
}

func (t OrganizationTeamResource) organizationTeamAlias() organizationTeamAlias {
	return organizationTeamAlias{
		Ref:         t.Ref,
		Kongctl:     t.Kongctl,
		External:    t.External,
		Name:        t.Name,
		Description: t.Description,
		Labels:      t.Labels,
		Roles:       t.Roles,
	}
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

	roleRefs := make(map[string]bool)
	for i, role := range t.Roles {
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

// SetDefaults applies default values to team resource
func (t *OrganizationTeamResource) SetDefaults() {
	// If Name is not set, use ref as default
	if t.Name == "" {
		t.Name = t.Ref
	}
	for i := range t.Roles {
		t.Roles[i].SetDefaults()
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
