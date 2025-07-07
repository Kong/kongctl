package resources

import (
	"encoding/json"
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// PortalResource represents a portal in declarative configuration
type PortalResource struct {
	kkComps.CreatePortal `yaml:",inline"`
	Ref                  string       `yaml:"ref" json:"ref"`
	Kongctl              *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`

	// Child resources that match API endpoints
	Customization *PortalCustomizationResource `yaml:"customization,omitempty" json:"customization,omitempty"`
	CustomDomain  *PortalCustomDomainResource  `yaml:"custom_domain,omitempty" json:"custom_domain,omitempty"`
	Pages         []PortalPageResource         `yaml:"pages,omitempty" json:"pages,omitempty"`
	Snippets      []PortalSnippetResource      `yaml:"snippets,omitempty" json:"snippets,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetKind returns the resource kind
func (p PortalResource) GetKind() string {
	return "portal"
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
	var deps []ResourceRef

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
	if p.Ref == "" {
		return fmt.Errorf("portal ref is required")
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

	return nil
}

// SetDefaults applies default values to portal resource
func (p *PortalResource) SetDefaults() {
	// If Name is not set, use ref as default
	if p.Name == "" {
		p.Name = p.Ref
	}

	// Apply defaults to pages
	for i := range p.Pages {
		p.Pages[i].SetDefaults()
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
func (p *PortalResource) TryMatchKonnectResource(konnectResource interface{}) bool {
	// For portals, we match by name
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

	// Look for Name and ID fields
	nameField := v.FieldByName("Name")
	idField := v.FieldByName("ID")

	if !nameField.IsValid() || !idField.IsValid() {
		// Try accessing embedded Portal
		portalField := v.FieldByName("Portal")
		if portalField.IsValid() && portalField.Kind() == reflect.Struct {
			nameField = portalField.FieldByName("Name")
			idField = portalField.FieldByName("ID")
		}
	}

	// Extract values if fields are valid
	if nameField.IsValid() && idField.IsValid() &&
		nameField.Kind() == reflect.String && idField.Kind() == reflect.String {
		if nameField.String() == p.Name {
			p.konnectID = idField.String()
			return true
		}
	}

	return false
}

// UnmarshalJSON implements custom JSON unmarshaling to preserve empty labels
func (p *PortalResource) UnmarshalJSON(data []byte) error {
	// Temporary struct to capture all fields including raw labels
	var temp struct {
		Ref                              string                         `json:"ref"`
		Name                             string                         `json:"name"`
		DisplayName                      *string                        `json:"display_name,omitempty"`
		Description                      *string                        `json:"description,omitempty"`
		AuthenticationEnabled            *bool                          `json:"authentication_enabled,omitempty"`
		RbacEnabled                      *bool                          `json:"rbac_enabled,omitempty"`
		DefaultAPIVisibility             *kkComps.DefaultAPIVisibility  `json:"default_api_visibility,omitempty"`
		DefaultPageVisibility            *kkComps.DefaultPageVisibility `json:"default_page_visibility,omitempty"`
		DefaultApplicationAuthStrategyID *string                        `json:"default_application_auth_strategy_id,omitempty"`
		AutoApproveDevelopers            *bool                          `json:"auto_approve_developers,omitempty"`
		AutoApproveApplications          *bool                          `json:"auto_approve_applications,omitempty"`
		Labels                           json.RawMessage                `json:"labels,omitempty"`
		Kongctl                          *KongctlMeta                   `json:"kongctl,omitempty"`
		Customization                    *PortalCustomizationResource   `json:"customization,omitempty"`
		CustomDomain                     *PortalCustomDomainResource    `json:"custom_domain,omitempty"`
		Pages                            []PortalPageResource           `json:"pages,omitempty"`
		Snippets                         []PortalSnippetResource        `json:"snippets,omitempty"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Set our fields
	p.Ref = temp.Ref
	p.Kongctl = temp.Kongctl
	p.Name = temp.Name
	p.DisplayName = temp.DisplayName
	p.Description = temp.Description
	p.AuthenticationEnabled = temp.AuthenticationEnabled
	p.RbacEnabled = temp.RbacEnabled
	p.DefaultAPIVisibility = temp.DefaultAPIVisibility
	p.DefaultPageVisibility = temp.DefaultPageVisibility
	p.DefaultApplicationAuthStrategyID = temp.DefaultApplicationAuthStrategyID
	p.AutoApproveDevelopers = temp.AutoApproveDevelopers
	p.AutoApproveApplications = temp.AutoApproveApplications

	// Handle nested resources
	p.Customization = temp.Customization
	p.CustomDomain = temp.CustomDomain
	p.Pages = temp.Pages
	p.Snippets = temp.Snippets

	// Handle labels specially to preserve empty map vs nil
	if len(temp.Labels) > 0 {
		// Check if labels is null (happens when all values are commented out)
		if string(temp.Labels) == "null" {
			// Treat null as empty map - user wants to remove all labels
			p.Labels = make(map[string]*string)
		} else {
			// Parse labels - if it's an empty object {}, we want to preserve that
			var labels map[string]*string
			if err := json.Unmarshal(temp.Labels, &labels); err != nil {
				return fmt.Errorf("failed to unmarshal labels: %w", err)
			}
			p.Labels = labels
		}
	}
	// If labels field was not present in JSON, p.Labels remains nil

	return nil
}

