package resources

import (
	"encoding/json"
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// PortalEmailConfigResource represents the portal email configuration (singleton child).
type PortalEmailConfigResource struct {
	kkComps.PostPortalEmailConfig `       yaml:",inline"          json:",inline"`
	Ref                           string `yaml:"ref"              json:"ref"`
	Portal                        string `yaml:"portal,omitempty" json:"portal,omitempty"`

	DomainNameSet   bool `yaml:"-" json:"-"`
	FromNameSet     bool `yaml:"-" json:"-"`
	FromEmailSet    bool `yaml:"-" json:"-"`
	ReplyToEmailSet bool `yaml:"-" json:"-"`

	konnectID string `yaml:"-" json:"-"`
}

func (c PortalEmailConfigResource) GetRef() string {
	return c.Ref
}

func (c PortalEmailConfigResource) Validate() error {
	if err := ValidateRef(c.Ref); err != nil {
		return fmt.Errorf("invalid portal email config ref: %w", err)
	}
	return nil
}

func (c *PortalEmailConfigResource) SetDefaults() {}

func (c PortalEmailConfigResource) GetType() ResourceType {
	return ResourceTypePortalEmailConfig
}

func (c PortalEmailConfigResource) GetMoniker() string {
	return c.Ref
}

func (c PortalEmailConfigResource) GetDependencies() []ResourceRef {
	return []ResourceRef{}
}

// GetReferenceFieldMappings returns cross-resource reference mappings for validation.
func (c PortalEmailConfigResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{
		"portal": "portal",
	}
}

func (c PortalEmailConfigResource) GetKonnectID() string {
	return c.konnectID
}

func (c PortalEmailConfigResource) GetKonnectMonikerFilter() string {
	// Singleton child matched via parent.
	return ""
}

func (c *PortalEmailConfigResource) TryMatchKonnectResource(konnectResource any) bool {
	v := reflect.ValueOf(konnectResource)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false
	}

	idField := v.FieldByName("ID")
	if idField.IsValid() && idField.Kind() == reflect.String && idField.String() != "" {
		c.konnectID = idField.String()
		return true
	}
	return false
}

// GetParentRef implements ResourceWithParent for inheritance of namespace and protection.
func (c PortalEmailConfigResource) GetParentRef() *ResourceRef {
	if c.Portal == "" {
		return nil
	}
	return &ResourceRef{Kind: string(ResourceTypePortal), Ref: c.Portal}
}

// UnmarshalJSON rejects kongctl metadata on child resources.
func (c *PortalEmailConfigResource) UnmarshalJSON(data []byte) error {
	var raw map[string]*json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var temp struct {
		Ref     string `json:"ref"`
		Portal  string `json:"portal,omitempty"`
		Kongctl any    `json:"kongctl,omitempty"`
		kkComps.PostPortalEmailConfig
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	if temp.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on portal email config")
	}

	c.Ref = temp.Ref
	c.Portal = temp.Portal
	c.PostPortalEmailConfig = temp.PostPortalEmailConfig

	if _, ok := raw["domain_name"]; ok {
		c.DomainNameSet = true
	}
	if _, ok := raw["from_name"]; ok {
		c.FromNameSet = true
	}
	if _, ok := raw["from_email"]; ok {
		c.FromEmailSet = true
	}
	if _, ok := raw["reply_to_email"]; ok {
		c.ReplyToEmailSet = true
	}

	return nil
}
