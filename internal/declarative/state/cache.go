package state

import (
	"strings"
	"time"
)

// Cache represents cached Konnect state with hierarchical structure
type Cache struct {
	// Top-level resources
	Portals                   map[string]*CachedPortal            // portalID -> portal with children
	APIs                      map[string]*CachedAPI               // apiID -> api with children
	ApplicationAuthStrategies map[string]*ApplicationAuthStrategy // strategyID -> strategy
}

// CachedPortal represents a portal with all its child resources
type CachedPortal struct {
	Portal

	// Child resources
	Pages         map[string]*CachedPortalPage // pageID -> page (with nested children)
	Customization *PortalCustomization         // singleton
	CustomDomain  *PortalCustomDomain          // singleton
	Snippets      map[string]*PortalSnippet    // snippetID -> snippet
}

// CachedPortalPage represents a page with its children
type CachedPortalPage struct {
	PortalPage

	// Child pages indexed by ID
	Children map[string]*CachedPortalPage // pageID -> child page
}

// CachedAPI represents an API with all its child resources
type CachedAPI struct {
	API

	// Child resources
	Versions        map[string]*APIVersion        // versionID -> version
	Publications    map[string]*APIPublication    // portalID -> publication (one per portal)
	Implementations map[string]*APIImplementation // implementationID -> implementation
	Documents       map[string]*CachedAPIDocument // documentID -> document (with nested children)
}

// CachedAPIDocument represents a document with its children
type CachedAPIDocument struct {
	APIDocument

	// Child documents indexed by ID
	Children map[string]*CachedAPIDocument // documentID -> child document
}

// NewCache creates an initialized cache
func NewCache() *Cache {
	return &Cache{
		Portals:                   make(map[string]*CachedPortal),
		APIs:                      make(map[string]*CachedAPI),
		ApplicationAuthStrategies: make(map[string]*ApplicationAuthStrategy),
	}
}

// GetPortalPage finds a page anywhere in the portal hierarchy
func (p *CachedPortal) GetPortalPage(pageID string) *CachedPortalPage {
	// Check direct children
	if page, ok := p.Pages[pageID]; ok {
		return page
	}

	// Recursively check nested pages
	for _, page := range p.Pages {
		if found := page.GetDescendant(pageID); found != nil {
			return found
		}
	}

	return nil
}

// GetDescendant finds a descendant page by ID
func (p *CachedPortalPage) GetDescendant(pageID string) *CachedPortalPage {
	if child, ok := p.Children[pageID]; ok {
		return child
	}

	for _, child := range p.Children {
		if found := child.GetDescendant(pageID); found != nil {
			return found
		}
	}

	return nil
}

// FindPageBySlugPath finds a page by its full slug path
func (p *CachedPortal) FindPageBySlugPath(slugPath string) *CachedPortalPage {
	segments := strings.Split(strings.Trim(slugPath, "/"), "/")
	if len(segments) == 0 {
		return nil
	}

	// Start from root pages
	for _, page := range p.Pages {
		if found := page.findByPathSegments(segments); found != nil {
			return found
		}
	}

	return nil
}

func (p *CachedPortalPage) findByPathSegments(segments []string) *CachedPortalPage {
	if len(segments) == 0 {
		return nil
	}

	normalizedSlug := strings.TrimPrefix(p.Slug, "/")

	// Check if this page matches the first segment
	if normalizedSlug == segments[0] {
		if len(segments) == 1 {
			return p
		}

		// Continue searching in children
		for _, child := range p.Children {
			if found := child.findByPathSegments(segments[1:]); found != nil {
				return found
			}
		}
	}

	return nil
}

// PortalCustomization represents portal customization (placeholder for missing type)
type PortalCustomization struct {
	// TODO: Add fields when implementing portal customization support
}

// PortalCustomDomain represents portal custom domain (placeholder for missing type)
type PortalCustomDomain struct {
	ID                       string
	PortalID                 string
	Hostname                 string
	Enabled                  bool
	DomainVerificationMethod string
	VerificationStatus       string
	ValidationErrors         []string
	SkipCACheck              *bool
	UploadedAt               *time.Time
	ExpiresAt                *time.Time
	CnameStatus              string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// PortalSnippet represents portal snippet
type PortalSnippet struct {
	ID               string
	Name             string
	Title            string
	Content          string // Will be empty from list, populated from fetch
	Description      string
	Visibility       string
	Status           string
	NormalizedLabels map[string]string
}
