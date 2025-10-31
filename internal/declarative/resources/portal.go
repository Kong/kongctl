package resources

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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
	return p.Name
}

// GetDependencies returns references to other resources this portal depends on
func (p PortalResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}

	// Portal may depend on an auth strategy
	if p.DefaultApplicationAuthStrategyID != nil && *p.DefaultApplicationAuthStrategyID != "" {
		deps = append(deps, ResourceRef{
			Kind: "application_auth_strategy",
			Ref:  *p.DefaultApplicationAuthStrategyID,
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
		result[k] = &v
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
	if p.Name == "" {
		p.Name = p.Ref
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
	if p.Name == "" {
		return ""
	}
	return fmt.Sprintf("name[eq]=%s", p.Name)
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
			matched = (nameField.String() == p.Name)
		}
	}

	if matched {
		p.konnectID = idField.String()
		return true
	}

	return false
}

// UnmarshalJSON ensures both the embedded SDK model and portal-specific fields
// (like ref and kongctl metadata) are populated when decoding from JSON/YAML
// while still surfacing unknown field errors (used for typo detection).
func (p *PortalResource) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	allowedKeys := make(map[string]struct{})
	sdkKeys := make(map[string]struct{})

	// Gather JSON field names from the embedded SDK struct
	sdkType := reflect.TypeOf(kkComps.CreatePortal{})
	for i := 0; i < sdkType.NumField(); i++ {
		field := sdkType.Field(i)
		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "" {
			continue
		}
		allowedKeys[name] = struct{}{}
		sdkKeys[name] = struct{}{}
	}

	// Add kongctl-specific fields
	extraKeys := []string{
		"ref",
		"kongctl",
		"customization",
		"custom_domain",
		"pages",
		"snippets",
		"_external",
	}
	for _, k := range extraKeys {
		allowedKeys[k] = struct{}{}
	}

	// Detect unknown fields early so parseYAML can provide suggestions.
	for key := range raw {
		if _, ok := allowedKeys[key]; !ok {
			return fmt.Errorf("json: unknown field %q", key)
		}
	}

	// Extract kongctl-specific fields
	if v, ok := raw["ref"]; ok {
		if err := json.Unmarshal(v, &p.Ref); err != nil {
			return err
		}
		delete(raw, "ref")
	}

	if v, ok := raw["kongctl"]; ok {
		if err := json.Unmarshal(v, &p.Kongctl); err != nil {
			return err
		}
		delete(raw, "kongctl")
	}

	if v, ok := raw["customization"]; ok {
		if err := json.Unmarshal(v, &p.Customization); err != nil {
			return err
		}
		delete(raw, "customization")
	}

	if v, ok := raw["custom_domain"]; ok {
		if err := json.Unmarshal(v, &p.CustomDomain); err != nil {
			return err
		}
		delete(raw, "custom_domain")
	}

	if v, ok := raw["pages"]; ok {
		if err := json.Unmarshal(v, &p.Pages); err != nil {
			return err
		}
		delete(raw, "pages")
	}

	if v, ok := raw["snippets"]; ok {
		if err := json.Unmarshal(v, &p.Snippets); err != nil {
			return err
		}
		delete(raw, "snippets")
	}

	if v, ok := raw["_external"]; ok {
		if err := json.Unmarshal(v, &p.External); err != nil {
			return err
		}
		delete(raw, "_external")
	}

	// Remaining fields belong to the embedded SDK struct.
	sdkPayload := make(map[string]json.RawMessage)
	for key, value := range raw {
		if _, ok := sdkKeys[key]; ok {
			sdkPayload[key] = value
		}
	}

	// Marshal back and unmarshal into the embedded CreatePortal struct.
	if len(sdkPayload) > 0 {
		encoded, err := json.Marshal(sdkPayload)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(encoded, &p.CreatePortal); err != nil {
			return err
		}
	} else {
		// Ensure embedded struct is zeroed if no fields were provided.
		p.CreatePortal = kkComps.CreatePortal{}
	}

	return nil
}

// MarshalJSON ensures the embedded SDK model and Kongctl fields are preserved when serializing.
func (p PortalResource) MarshalJSON() ([]byte, error) {
	alias := p.portalAlias()
	return json.Marshal(alias)
}

// MarshalYAML ensures YAML output mirrors the custom JSON encoding.
func (p PortalResource) MarshalYAML() (any, error) {
	return p.portalAlias(), nil
}

type portalAlias struct {
	portalCreateAlias `json:",inline" yaml:",inline"`
	Ref               string                       `json:"ref" yaml:"ref"`
	Kongctl           *KongctlMeta                 `json:"kongctl,omitempty" yaml:"kongctl,omitempty"`
	Customization     *PortalCustomizationResource `json:"customization,omitempty" yaml:"customization,omitempty"`
	CustomDomain      *PortalCustomDomainResource  `json:"custom_domain,omitempty" yaml:"custom_domain,omitempty"`
	Pages             []PortalPageResource         `json:"pages,omitempty" yaml:"pages,omitempty"`
	Snippets          []PortalSnippetResource      `json:"snippets,omitempty" yaml:"snippets,omitempty"`
	External          *ExternalBlock               `json:"_external,omitempty" yaml:"_external,omitempty"`
}

type portalCreateAlias kkComps.CreatePortal

func (p PortalResource) portalAlias() portalAlias {
	return portalAlias{
		portalCreateAlias: portalCreateAlias(p.CreatePortal),
		Ref:               p.Ref,
		Kongctl:           p.Kongctl,
		Customization:     p.Customization,
		CustomDomain:      p.CustomDomain,
		Pages:             p.Pages,
		Snippets:          p.Snippets,
		External:          p.External,
	}
}

// IsExternal returns true if this portal is externally managed
func (p *PortalResource) IsExternal() bool {
	return p.External != nil && p.External.IsExternal()
}
