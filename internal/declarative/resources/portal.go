package resources

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// PortalResource represents a portal in declarative configuration
type PortalResource struct {
	kkComps.CreatePortal `yaml:",inline"`
	Ref     string       `yaml:"ref" json:"ref"`
	Kongctl *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
	
	// Child resources that match API endpoints
	Customization *PortalCustomizationResource `yaml:"customization,omitempty" json:"customization,omitempty"`
	CustomDomain  *PortalCustomDomainResource  `yaml:"custom_domain,omitempty" json:"custom_domain,omitempty"`
	Pages         []PortalPageResource         `yaml:"pages,omitempty" json:"pages,omitempty"`
	Snippets      []PortalSnippetResource      `yaml:"snippets,omitempty" json:"snippets,omitempty"`
}

// GetRef returns the reference identifier used for cross-resource references
func (p PortalResource) GetRef() string {
	return p.Ref
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