package resources

import (
	"fmt"
	"regexp"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// PortalPageResource represents a portal page
type PortalPageResource struct {
	kkComps.CreatePortalPageRequest `yaml:",inline" json:",inline"`
	Ref    string `yaml:"ref" json:"ref"`
	Portal string `yaml:"portal,omitempty" json:"portal,omitempty"` // Parent portal reference
}

// GetRef returns the reference identifier
func (p PortalPageResource) GetRef() string {
	return p.Ref
}

// Validate ensures the portal page resource is valid
func (p PortalPageResource) Validate() error {
	if p.Ref == "" {
		return fmt.Errorf("page ref is required")
	}
	
	if p.Slug == "" {
		return fmt.Errorf("page slug is required")
	}
	
	// Validate slug format (no slashes, valid URL segment)
	slugRegex := regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	if !slugRegex.MatchString(p.Slug) {
		return fmt.Errorf("page slug must be lowercase alphanumeric with hyphens (e.g., 'getting-started')")
	}
	
	if p.Content == "" {
		return fmt.Errorf("page content is required")
	}
	
	// Validate visibility if set
	if p.Visibility != nil {
		validVisibility := false
		for _, v := range []kkComps.PageVisibilityStatus{
			kkComps.PageVisibilityStatusPublic,
			kkComps.PageVisibilityStatusPrivate,
		} {
			if *p.Visibility == v {
				validVisibility = true
				break
			}
		}
		if !validVisibility {
			return fmt.Errorf("page visibility must be 'public' or 'private'")
		}
	}
	
	// Validate status if set
	if p.Status != nil {
		validStatus := false
		for _, s := range []kkComps.PublishedStatus{
			kkComps.PublishedStatusPublished,
			kkComps.PublishedStatusUnpublished,
		} {
			if *p.Status == s {
				validStatus = true
				break
			}
		}
		if !validStatus {
			return fmt.Errorf("page status must be 'published' or 'unpublished'")
		}
	}
	
	return nil
}

// SetDefaults applies default values
func (p *PortalPageResource) SetDefaults() {
	// Set default visibility to public if not specified
	if p.Visibility == nil {
		visibility := kkComps.PageVisibilityStatusPublic
		p.Visibility = &visibility
	}
	
	// Set default status to published if not specified
	if p.Status == nil {
		status := kkComps.PublishedStatusPublished
		p.Status = &status
	}
	
	// Set title from slug if not provided
	if p.Title == nil && p.Slug != "" {
		title := p.Slug
		p.Title = &title
	}
}

// PortalSnippetResource represents a portal snippet
type PortalSnippetResource struct {
	Ref     string `yaml:"ref" json:"ref"`
	Portal  string `yaml:"portal,omitempty" json:"portal,omitempty"` // Parent portal reference
	Name    string `yaml:"name" json:"name"`
	Content string `yaml:"content" json:"content"`
}

// GetRef returns the reference identifier
func (s PortalSnippetResource) GetRef() string {
	return s.Ref
}

// Validate ensures the portal snippet resource is valid
func (s PortalSnippetResource) Validate() error {
	if s.Ref == "" {
		return fmt.Errorf("snippet ref is required")
	}
	
	if s.Name == "" {
		return fmt.Errorf("snippet name is required")
	}
	
	if s.Content == "" {
		return fmt.Errorf("snippet content is required")
	}
	
	return nil
}

// SetDefaults applies default values
func (s *PortalSnippetResource) SetDefaults() {
	// No defaults needed for snippets currently
}