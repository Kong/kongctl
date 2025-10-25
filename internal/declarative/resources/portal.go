package resources

import (
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util"
)

// PortalResource represents a portal in declarative configuration
type PortalResource struct {
	kkComps.CreatePortal `             yaml:",inline"           json:",inline"`
	Ref                  string       `yaml:"ref"               json:"ref"`
	Kongctl              *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`

	// Child resources that match API endpoints
	Customization *PortalCustomizationResource `yaml:"customization,omitempty" json:"customization,omitempty"`
	CustomDomain  *PortalCustomDomainResource  `yaml:"custom_domain,omitempty" json:"custom_domain,omitempty"`
	Pages         []PortalPageResource         `yaml:"pages,omitempty"         json:"pages,omitempty"`
	Snippets      []PortalSnippetResource      `yaml:"snippets,omitempty"      json:"snippets,omitempty"`

	// External resource marker
	External *ExternalBlock `yaml:"_external,omitempty" json:"_external,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetType returns the resource type
func (p PortalResource) GetType() ResourceType {
	return ResourceTypePortal
}

// GetRef returns the reference identifier used for cross-resource references
func (p PortalResource) GetRef() string {
	return p.Ref
}

// GetMoniker returns the resource moniker (for portals, this is the name)
func (p PortalResource) GetMoniker() string {
	return util.StringValue(p.Name)
}

// GetDependencies returns references to other resources this portal depends on
func (p PortalResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}

	// Portal may depend on an auth strategy
	if authStrategyID := util.StringValue(p.DefaultApplicationAuthStrategyID); authStrategyID != "" {
		deps = append(deps, ResourceRef{
			Kind: "application_auth_strategy",
			Ref:  authStrategyID,
		})
	}

	return deps
}

// GetLabels returns the labels for this resource
func (p PortalResource) GetLabels() map[string]string {
	if p.Labels == nil {
		return nil
	}

	// Convert from SDK's map[string]*string to map[string]string
	result := make(map[string]string)
	for k, v := range p.Labels {
		if v != nil {
			result[k] = *v
		}
	}
	return result
}

// SetLabels sets the labels for this resource
func (p *PortalResource) SetLabels(labels map[string]string) {
	if labels == nil {
		p.Labels = nil
		return
	}

	// Convert from map[string]string to SDK's map[string]*string
	result := make(map[string]*string)
	for k, v := range labels {
		value := v
		result[k] = &value
	}
	p.Labels = result
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (p PortalResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{
		"default_application_auth_strategy_id": "application_auth_strategy",
	}
}

// Validate ensures the portal resource is valid
func (p PortalResource) Validate() error {
	if err := ValidateRef(p.Ref); err != nil {
		return fmt.Errorf("invalid portal ref: %w", err)
	}

	// Validate child resources
	if p.Customization != nil {
		if err := p.Customization.Validate(); err != nil {
			return fmt.Errorf("invalid portal customization: %w", err)
		}
	}

	if p.CustomDomain != nil {
		if err := p.CustomDomain.Validate(); err != nil {
			return fmt.Errorf("invalid custom domain: %w", err)
		}
	}

	// Validate pages
	pageRefs := make(map[string]bool)
	for i, page := range p.Pages {
		if err := page.Validate(); err != nil {
			return fmt.Errorf("invalid page %d: %w", i, err)
		}
		if pageRefs[page.GetRef()] {
			return fmt.Errorf("duplicate page ref: %s", page.GetRef())
		}
		pageRefs[page.GetRef()] = true
	}

	// Validate snippets
	snippetRefs := make(map[string]bool)
	for i, snippet := range p.Snippets {
		if err := snippet.Validate(); err != nil {
			return fmt.Errorf("invalid snippet %d: %w", i, err)
		}
		if snippetRefs[snippet.GetRef()] {
			return fmt.Errorf("duplicate snippet ref: %s", snippet.GetRef())
		}
		snippetRefs[snippet.GetRef()] = true
	}

	// Validate external block if present
	if p.External != nil {
		if err := p.External.Validate(); err != nil {
			return fmt.Errorf("invalid _external block: %w", err)
		}
	}

	return nil
}

// SetDefaults applies default values to portal resource
func (p *PortalResource) SetDefaults() {
	// If Name is not set, use ref as default
	if util.StringValue(p.Name) == "" {
		name := p.Ref
		p.Name = &name
	}

	// Apply defaults to child resources
	if p.Customization != nil {
		p.Customization.SetDefaults()
	}

	if p.CustomDomain != nil {
		p.CustomDomain.SetDefaults()
	}

	// Apply defaults to pages
	for i := range p.Pages {
		p.Pages[i].SetDefaults()
	}

	// Apply defaults to snippets
	for i := range p.Snippets {
		p.Snippets[i].SetDefaults()
	}
}

// GetKonnectID returns the resolved Konnect ID if available
func (p PortalResource) GetKonnectID() string {
	return p.konnectID
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (p PortalResource) GetKonnectMonikerFilter() string {
	name := util.StringValue(p.Name)
	if name == "" {
		return ""
	}
	return fmt.Sprintf("name[eq]=%s", name)
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (p *PortalResource) TryMatchKonnectResource(konnectResource any) bool {
	// Use reflection to access fields from state.Portal
	v := reflect.ValueOf(konnectResource)

	// Handle pointer types
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Ensure we have a struct
	if v.Kind() != reflect.Struct {
		return false
	}

	// Get ID field (we'll need this regardless of match type)
	idField := v.FieldByName("ID")
	if !idField.IsValid() {
		// Try accessing embedded Portal
		portalField := v.FieldByName("Portal")
		if portalField.IsValid() && portalField.Kind() == reflect.Struct {
			idField = portalField.FieldByName("ID")
		}
	}

	if !idField.IsValid() || idField.Kind() != reflect.String {
		return false
	}

	// Check match based on configuration
	matched := false

	if p.IsExternal() && p.External != nil {
		if p.External.ID != "" {
			// Direct ID match
			matched = (idField.String() == p.External.ID)
		} else if p.External.Selector != nil {
			// Selector-based match
			matched = p.External.Selector.Match(konnectResource)
		}
	} else {
		// Non-external: match by name (existing logic)
		nameField := v.FieldByName("Name")
		if !nameField.IsValid() {
			// Try accessing embedded Portal
			portalField := v.FieldByName("Portal")
			if portalField.IsValid() && portalField.Kind() == reflect.Struct {
				nameField = portalField.FieldByName("Name")
			}
		}

		if nameField.IsValid() && nameField.Kind() == reflect.String {
			matched = (nameField.String() == util.StringValue(p.Name))
		}
	}

	if matched {
		p.konnectID = idField.String()
		return true
	}

	return false
}

// IsExternal returns true if this portal is externally managed
func (p *PortalResource) IsExternal() bool {
	return p.External != nil && p.External.IsExternal()
}
