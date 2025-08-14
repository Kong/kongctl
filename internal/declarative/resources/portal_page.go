package resources

import (
	"fmt"
	"reflect"
	"regexp"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// PortalPageResource represents a portal page
type PortalPageResource struct {
	kkComps.CreatePortalPageRequest `yaml:",inline" json:",inline"`
	Ref           string                `yaml:"ref" json:"ref"`
	// Parent portal reference
	Portal        string                `yaml:"portal,omitempty" json:"portal,omitempty"`
	// Reference to parent page
	ParentPageRef string                `yaml:"parent_page_ref,omitempty" json:"parent_page_ref,omitempty"`
	// Nested child pages
	Children      []PortalPageResource  `yaml:"children,omitempty" json:"children,omitempty"`
	
	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetRef returns the reference identifier
func (p PortalPageResource) GetRef() string {
	return p.Ref
}

// Validate ensures the portal page resource is valid
func (p PortalPageResource) Validate() error {
	if err := ValidateRef(p.Ref); err != nil {
		return fmt.Errorf("invalid page ref: %w", err)
	}
	
	// Validate slug
	switch p.Slug {
	case "/":
		// Special case: allow "/" for root pages only
		if p.ParentPageRef != "" {
			return fmt.Errorf("slug '/' is only valid for root pages (pages without a parent)")
		}
		// "/" is valid for root pages, skip further validation
	case "":
		return fmt.Errorf("page slug is required")
	default:
		// Validate slug format using Konnect's regex pattern for non-root pages
		slugRegex := regexp.MustCompile(`^[\w-]+$`)
		if !slugRegex.MatchString(p.Slug) {
			return fmt.Errorf("invalid slug %q: slugs must contain only letters, numbers, underscores, and hyphens", p.Slug)
		}
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
	
	// Validate children recursively
	for i, child := range p.Children {
		// Children should not redefine portal
		if child.Portal != "" {
			return fmt.Errorf("child page[%d] ref=%q should not define portal (inherited from parent)", i, child.Ref)
		}
		if err := child.Validate(); err != nil {
			return fmt.Errorf("child page[%d] ref=%q validation failed: %w", i, child.Ref, err)
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

// GetType returns the resource type
func (p PortalPageResource) GetType() ResourceType {
	return ResourceTypePortalPage
}

// GetMoniker returns the resource moniker (for pages, this is the slug)
func (p PortalPageResource) GetMoniker() string {
	return p.Slug
}

// GetDependencies returns references to other resources this page depends on
func (p PortalPageResource) GetDependencies() []ResourceRef {
	// Portal pages don't have dependencies (parent page is handled separately)
	return []ResourceRef{}
}

// GetKonnectID returns the resolved Konnect ID if available
func (p PortalPageResource) GetKonnectID() string {
	return p.konnectID
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (p PortalPageResource) GetKonnectMonikerFilter() string {
	if p.Slug == "" {
		return ""
	}
	return fmt.Sprintf("slug[eq]=%s", p.Slug)
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (p *PortalPageResource) TryMatchKonnectResource(konnectResource any) bool {
	// For portal pages, we match by slug
	// Use reflection to access fields from state.PortalPage
	v := reflect.ValueOf(konnectResource)
	
	// Handle pointer types
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	
	// Ensure we have a struct
	if v.Kind() != reflect.Struct {
		return false
	}
	
	// Look for Slug and ID fields
	slugField := v.FieldByName("Slug")
	idField := v.FieldByName("ID")
	
	// Extract values if fields are valid
	if slugField.IsValid() && idField.IsValid() && 
	   slugField.Kind() == reflect.String && idField.Kind() == reflect.String {
		if slugField.String() == p.Slug {
			p.konnectID = idField.String()
			return true
		}
	}
	return false
}

// PortalSnippetResource represents a portal snippet
type PortalSnippetResource struct {
	Ref         string                            `yaml:"ref" json:"ref"`
	// Parent portal reference
	Portal      string                            `yaml:"portal,omitempty" json:"portal,omitempty"`
	Name        string                            `yaml:"name" json:"name"`
	Content     string                            `yaml:"content" json:"content"`
	Title       *string                           `yaml:"title,omitempty" json:"title,omitempty"`
	Visibility  *kkComps.SnippetVisibilityStatus  `yaml:"visibility,omitempty" json:"visibility,omitempty"`
	Status      *kkComps.PublishedStatus          `yaml:"status,omitempty" json:"status,omitempty"`
	Description *string                           `yaml:"description,omitempty" json:"description,omitempty"`
	
	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetRef returns the reference identifier
func (s PortalSnippetResource) GetRef() string {
	return s.Ref
}

// Validate ensures the portal snippet resource is valid
func (s PortalSnippetResource) Validate() error {
	if err := ValidateRef(s.Ref); err != nil {
		return fmt.Errorf("invalid snippet ref: %w", err)
	}
	
	if s.Name == "" {
		return fmt.Errorf("snippet name is required")
	}
	
	if s.Content == "" {
		return fmt.Errorf("snippet content is required")
	}
	
	// Validate visibility if set
	if s.Visibility != nil {
		validVisibility := false
		for _, v := range []kkComps.SnippetVisibilityStatus{
			kkComps.SnippetVisibilityStatusPublic,
			kkComps.SnippetVisibilityStatusPrivate,
		} {
			if *s.Visibility == v {
				validVisibility = true
				break
			}
		}
		if !validVisibility {
			return fmt.Errorf("snippet visibility must be 'public' or 'private'")
		}
	}
	
	// Validate status if set
	if s.Status != nil {
		validStatus := false
		for _, st := range []kkComps.PublishedStatus{
			kkComps.PublishedStatusPublished,
			kkComps.PublishedStatusUnpublished,
		} {
			if *s.Status == st {
				validStatus = true
				break
			}
		}
		if !validStatus {
			return fmt.Errorf("snippet status must be 'published' or 'unpublished'")
		}
	}
	
	return nil
}

// SetDefaults applies default values
func (s *PortalSnippetResource) SetDefaults() {
	// Set default visibility to public if not specified
	if s.Visibility == nil {
		visibility := kkComps.SnippetVisibilityStatusPublic
		s.Visibility = &visibility
	}
	
	// Set default status to published if not specified
	if s.Status == nil {
		status := kkComps.PublishedStatusPublished
		s.Status = &status
	}
	
	// Set title from name if not provided
	if s.Title == nil && s.Name != "" {
		title := s.Name
		s.Title = &title
	}
}

// GetType returns the resource type
func (s PortalSnippetResource) GetType() ResourceType {
	return ResourceTypePortalSnippet
}

// GetMoniker returns the resource moniker (for snippets, this is the name)
func (s PortalSnippetResource) GetMoniker() string {
	return s.Name
}

// GetDependencies returns references to other resources this snippet depends on
func (s PortalSnippetResource) GetDependencies() []ResourceRef {
	// Portal snippets don't have dependencies
	return []ResourceRef{}
}

// GetKonnectID returns the resolved Konnect ID if available
func (s PortalSnippetResource) GetKonnectID() string {
	return s.konnectID
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (s PortalSnippetResource) GetKonnectMonikerFilter() string {
	if s.Name == "" {
		return ""
	}
	return fmt.Sprintf("name[eq]=%s", s.Name)
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (s *PortalSnippetResource) TryMatchKonnectResource(konnectResource any) bool {
	// For portal snippets, we match by name
	// Use reflection to access fields from state.PortalSnippet
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
	
	// Extract values if fields are valid
	if nameField.IsValid() && idField.IsValid() && 
	   nameField.Kind() == reflect.String && idField.Kind() == reflect.String {
		if nameField.String() == s.Name {
			s.konnectID = idField.String()
			return true
		}
	}
	return false
}