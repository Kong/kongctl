package resources

import (
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// TeamResource represents a team in declarative configuration
type TeamResource struct {
	kkComps.CreateTeam `yaml:",inline" json:",inline"`
	Ref                string         `yaml:"ref" json:"ref"`
	Kongctl            *KongctlMeta   `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
	External           *ExternalBlock `yaml:"_external,omitempty" json:"_external,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetRef returns the reference identifier used for cross-resource references
func (t TeamResource) GetRef() string {
	return t.Ref
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (t TeamResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{} // No outbound references
}

// Validate ensures the team resource is valid
func (t TeamResource) Validate() error {
	if err := ValidateRef(t.Ref); err != nil {
		return fmt.Errorf("invalid team ref: %w", err)
	}

	if t.Name == "" {
		return fmt.Errorf("name is required")
	}

	if t.Description == nil || *t.Description == "" {
		return fmt.Errorf("description is required")
	}

	if t.External != nil {
		if err := t.External.Validate(); err != nil {
			return fmt.Errorf("invalid _external block: %w", err)
		}
	}
	return nil
}

// SetDefaults applies default values to team resource
func (t *TeamResource) SetDefaults() {
	// If Name is not set, use ref as default
	if t.Name == "" {
		t.Name = t.Ref
	}
}

// GetType returns the resource type
func (t TeamResource) GetType() ResourceType {
	return ResourceTypeTeam
}

// GetMoniker returns the resource moniker (for teams, this is the name)
func (t TeamResource) GetMoniker() string {
	return t.Name
}

// GetDependencies returns references to other resources this team depends on
func (t TeamResource) GetDependencies() []ResourceRef {
	// Teams don't depend on other resources
	return []ResourceRef{}
}

// GetKonnectID returns the resolved Konnect ID if available
func (t TeamResource) GetKonnectID() string {
	return t.konnectID
}

// GetLabels returns the labels for this resource
func (t TeamResource) GetLabels() map[string]string {
	return t.Labels
}

// SetLabels sets the labels for this resource
func (t *TeamResource) SetLabels(labels map[string]string) {
	t.Labels = labels
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (t TeamResource) GetKonnectMonikerFilter() string {
	if t.IsExternal() {
		return ""
	}

	return fmt.Sprintf("name[eq]=%s", t.Name)
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (t *TeamResource) TryMatchKonnectResource(konnectResource any) bool {
	// Use reflection to access fields from state.ControlPlane
	v := reflect.ValueOf(konnectResource)

	// Handle pointer types
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Ensure we have a struct
	if v.Kind() != reflect.Struct {
		return false
	}

	// Look for ID field for matching
	idField := v.FieldByName("ID")
	if !idField.IsValid() {
		return false
	}

	if t.IsExternal() && t.External != nil {
		matched := false
		if t.External.ID != "" {
			matched = (idField.String() == t.External.ID)
		} else if t.External.Selector != nil {
			matched = t.External.Selector.Match(konnectResource)
		}

		if matched {
			t.konnectID = idField.String()
			return true
		}

		return false
	}

	// Non-external teams match by name
	nameField := v.FieldByName("Name")
	if nameField.IsValid() && nameField.Kind() == reflect.String &&
		nameField.String() == t.Name {
		t.konnectID = idField.String()
		return true
	}

	return false
}

// IsExternal returns true if this team is externally managed
func (t *TeamResource) IsExternal() bool {
	return t.External != nil && t.External.IsExternal()
}
