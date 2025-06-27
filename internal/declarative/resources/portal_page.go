package resources

import (
	"fmt"
	"strings"
)

// PortalPageResource represents a portal page
type PortalPageResource struct {
	// Note: Using a simplified structure since the SDK type isn't clear
	// In practice, this would embed the appropriate SDK type
	Ref        string `yaml:"ref" json:"ref"`
	Title      string `yaml:"title" json:"title"`
	Path       string `yaml:"path" json:"path"`
	Content    string `yaml:"content" json:"content"`
	Visibility string `yaml:"visibility,omitempty" json:"visibility,omitempty"` // public or private
	ParentID   string `yaml:"parent_id,omitempty" json:"parent_id,omitempty"`   // For nested pages
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
	
	if p.Title == "" {
		return fmt.Errorf("page title is required")
	}
	
	if p.Path == "" {
		return fmt.Errorf("page path is required")
	}
	
	// Path must start with /
	if !strings.HasPrefix(p.Path, "/") {
		return fmt.Errorf("page path must start with /")
	}
	
	// Validate visibility
	if p.Visibility != "" && p.Visibility != "public" && p.Visibility != "private" {
		return fmt.Errorf("page visibility must be 'public' or 'private'")
	}
	
	return nil
}

// SetDefaults applies default values
func (p *PortalPageResource) SetDefaults() {
	if p.Visibility == "" {
		p.Visibility = "public"
	}
}

// PortalSnippetResource represents a portal snippet
type PortalSnippetResource struct {
	Ref     string `yaml:"ref" json:"ref"`
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