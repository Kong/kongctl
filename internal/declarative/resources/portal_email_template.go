package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypePortalEmailTemplate,
		func(rs *ResourceSet) *[]PortalEmailTemplateResource { return &rs.PortalEmailTemplates },
	)
}

// PortalEmailTemplateResource represents a customizable portal email template.
type PortalEmailTemplateResource struct {
	Ref     string                      `yaml:"ref"               json:"ref"`
	Portal  string                      `yaml:"portal,omitempty"  json:"portal,omitempty"`
	Name    kkComps.EmailTemplateName   `yaml:"name,omitempty"    json:"name,omitempty"`
	Content *PortalEmailTemplateContent `yaml:"content,omitempty" json:"content,omitempty"`
	Enabled *bool                       `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	NameSet    bool `yaml:"-" json:"-"`
	ContentSet bool `yaml:"-" json:"-"`
	EnabledSet bool `yaml:"-" json:"-"`

	konnectID string `yaml:"-" json:"-"`
}

// PortalEmailTemplateContent captures the customizable fields of an email template.
type PortalEmailTemplateContent struct {
	Subject     *string `yaml:"subject,omitempty"      json:"subject,omitempty"`
	Title       *string `yaml:"title,omitempty"        json:"title,omitempty"`
	Body        *string `yaml:"body,omitempty"         json:"body,omitempty"`
	ButtonLabel *string `yaml:"button_label,omitempty" json:"button_label,omitempty"`

	SubjectSet     bool `yaml:"-" json:"-"`
	TitleSet       bool `yaml:"-" json:"-"`
	BodySet        bool `yaml:"-" json:"-"`
	ButtonLabelSet bool `yaml:"-" json:"-"`
}

// UnmarshalJSON tracks which fields were explicitly set, including nulls.
func (c *PortalEmailTemplateContent) UnmarshalJSON(data []byte) error {
	var raw map[string]*json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	type alias PortalEmailTemplateContent
	var tmp alias
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	*c = PortalEmailTemplateContent(tmp)

	if _, ok := raw["subject"]; ok {
		c.SubjectSet = true
	}
	if _, ok := raw["title"]; ok {
		c.TitleSet = true
	}
	if _, ok := raw["body"]; ok {
		c.BodySet = true
	}
	if _, ok := raw["button_label"]; ok {
		c.ButtonLabelSet = true
	}

	return nil
}

func (t PortalEmailTemplateResource) GetRef() string {
	return t.Ref
}

func (t PortalEmailTemplateResource) Validate() error {
	if err := ValidateRef(t.Ref); err != nil {
		return fmt.Errorf("invalid portal email template ref: %w", err)
	}
	if t.Portal == "" {
		return fmt.Errorf("portal is required for portal_email_template %q", t.Ref)
	}
	if t.Name == "" {
		return fmt.Errorf("name is required for portal_email_template %q", t.Ref)
	}
	if _, ok := validPortalEmailTemplateNames[t.Name]; !ok {
		return fmt.Errorf("name %q is not a supported portal email template", t.Name)
	}
	if t.EnabledSet && t.Enabled == nil {
		return fmt.Errorf("enabled cannot be null for portal_email_template %q", t.Ref)
	}
	return nil
}

func (t *PortalEmailTemplateResource) SetDefaults() {
	if t.Name == "" && t.Ref != "" {
		t.Name = kkComps.EmailTemplateName(t.Ref)
	}
	if t.Ref == "" && t.Name != "" {
		t.Ref = string(t.Name)
	}
}

func (t PortalEmailTemplateResource) GetType() ResourceType {
	return ResourceTypePortalEmailTemplate
}

func (t PortalEmailTemplateResource) GetMoniker() string {
	return string(t.Name)
}

func (t PortalEmailTemplateResource) GetDependencies() []ResourceRef {
	return []ResourceRef{}
}

// GetReferenceFieldMappings returns cross-resource reference mappings for validation.
func (t PortalEmailTemplateResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{
		"portal": "portal",
	}
}

func (t PortalEmailTemplateResource) GetKonnectID() string {
	if t.konnectID != "" {
		return t.konnectID
	}
	return string(t.Name)
}

func (t PortalEmailTemplateResource) GetKonnectMonikerFilter() string {
	if t.Name == "" {
		return ""
	}
	return fmt.Sprintf("name[eq]=%s", t.Name)
}

func (t *PortalEmailTemplateResource) TryMatchKonnectResource(konnectResource any) bool {
	if id := tryMatchByField(konnectResource, "Name", string(t.Name)); id != "" {
		t.konnectID = id
		return true
	}
	return false
}

// GetParentRef implements ResourceWithParent for inheritance of namespace and protection.
func (t PortalEmailTemplateResource) GetParentRef() *ResourceRef {
	if t.Portal == "" {
		return nil
	}
	return &ResourceRef{Kind: string(ResourceTypePortal), Ref: t.Portal}
}

// UnmarshalJSON rejects kongctl metadata on child resources and tracks field presence.
func (t *PortalEmailTemplateResource) UnmarshalJSON(data []byte) error {
	var raw map[string]*json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var tmp struct {
		Ref     string                      `json:"ref"`
		Portal  string                      `json:"portal,omitempty"`
		Name    kkComps.EmailTemplateName   `json:"name,omitempty"`
		Content *PortalEmailTemplateContent `json:"content,omitempty"`
		Enabled *bool                       `json:"enabled,omitempty"`
		Kongctl any                         `json:"kongctl,omitempty"`
	}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	if tmp.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on portal email template")
	}

	t.Ref = tmp.Ref
	t.Portal = tmp.Portal
	t.Name = tmp.Name
	t.Content = tmp.Content
	t.Enabled = tmp.Enabled

	if _, ok := raw["name"]; ok {
		t.NameSet = true
	}
	if _, ok := raw["content"]; ok {
		t.ContentSet = true
	}
	if _, ok := raw["enabled"]; ok {
		t.EnabledSet = true
	}

	return nil
}

var validPortalEmailTemplateNames = map[kkComps.EmailTemplateName]struct{}{
	kkComps.EmailTemplateNameAppRegistrationApproved: {},
	kkComps.EmailTemplateNameAppRegistrationRejected: {},
	kkComps.EmailTemplateNameAppRegistrationRevoked:  {},
	kkComps.EmailTemplateNameConfirmEmailAddress:     {},
	kkComps.EmailTemplateNameResetPassword:           {},
	kkComps.EmailTemplateNameAccountAccessApproved:   {},
	kkComps.EmailTemplateNameAccountAccessRejected:   {},
	kkComps.EmailTemplateNameAccountAccessRevoked:    {},
}
