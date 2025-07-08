package planner

import (
	"context"
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// Portal Customization planning

func (p *Planner) planPortalCustomizationsChanges(
	_ context.Context, desired []resources.PortalCustomizationResource, plan *Plan,
) error { //nolint:unparam // Will return errors when state fetching is added
	// Skip if no customizations to plan
	if len(desired) == 0 {
		return nil
	}

	// For each desired customization
	for _, desiredCustomization := range desired {
		// Portal customizations are singleton resources per portal
		// Always update since it exists by default
		p.planPortalCustomizationUpdate(desiredCustomization, plan)
	}

	return nil
}

func (p *Planner) planPortalCustomizationUpdate(
	customization resources.PortalCustomizationResource, plan *Plan,
) {
	fields := make(map[string]interface{})
	
	// Add theme settings if present
	if customization.Theme != nil {
		themeFields := make(map[string]interface{})
		if customization.Theme.Colors != nil {
			colorsFields := make(map[string]interface{})
			if customization.Theme.Colors.Primary != nil {
				colorsFields["primary"] = *customization.Theme.Colors.Primary
			}
			themeFields["colors"] = colorsFields
		}
		fields["theme"] = themeFields
	}

	// Add menu settings if present
	if customization.Menu != nil {
		menuFields := make(map[string]interface{})
		if customization.Menu.Main != nil {
			var mainMenuItems []map[string]interface{}
			for _, item := range customization.Menu.Main {
				menuItem := map[string]interface{}{
					"path":  item.Path,
					"title": item.Title,
				}
				mainMenuItems = append(mainMenuItems, menuItem)
			}
			menuFields["main"] = mainMenuItems
		}
		fields["menu"] = menuFields
	}

	// Determine dependencies - depends on parent portal
	var dependencies []string
	if customization.Portal != "" {
		// Find the change ID for the parent portal
		for _, change := range plan.Changes {
			if change.ResourceType == "portal" && change.ResourceRef == customization.Portal {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	// Portal customization is a singleton resource - always use UPDATE action
	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, customization.Ref),
		ResourceType: "portal_customization",
		ResourceRef:  customization.Ref,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    dependencies,
	}

	// Store parent portal reference
	if customization.Portal != "" {
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: customization.Portal,
			},
		}
	}

	plan.AddChange(change)
}

// Portal Custom Domain planning

func (p *Planner) planPortalCustomDomainsChanges(
	_ context.Context, desired []resources.PortalCustomDomainResource, plan *Plan,
) error { //nolint:unparam // Will return errors when state fetching is added
	// Skip if no custom domains to plan
	if len(desired) == 0 {
		return nil
	}

	// For each desired custom domain
	for _, desiredDomain := range desired {
		// Portal custom domains are singleton resources per portal
		// Always create/update, never fetch current state
		p.planPortalCustomDomainCreate(desiredDomain, plan)
	}

	return nil
}

func (p *Planner) planPortalCustomDomainCreate(
	domain resources.PortalCustomDomainResource, plan *Plan,
) {
	fields := make(map[string]interface{})
	fields["hostname"] = domain.Hostname
	fields["enabled"] = domain.Enabled

	// Add SSL settings if present
	// Check if DomainVerificationMethod is set (non-empty string)
	if domain.Ssl.DomainVerificationMethod != "" {
		sslFields := make(map[string]interface{})
		sslFields["domain_verification_method"] = string(domain.Ssl.DomainVerificationMethod)
		fields["ssl"] = sslFields
	}

	// Determine dependencies - depends on parent portal
	var dependencies []string
	if domain.Portal != "" {
		// Find the change ID for the parent portal
		for _, change := range plan.Changes {
			if change.ResourceType == "portal" && change.ResourceRef == domain.Portal {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, domain.Ref),
		ResourceType: "portal_custom_domain",
		ResourceRef:  domain.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependencies,
	}

	// Store parent portal reference
	if domain.Portal != "" {
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: domain.Portal,
			},
		}
	}

	plan.AddChange(change)
}

// Portal Page planning

func (p *Planner) planPortalPagesChanges(
	ctx context.Context, portalID string, portalRef string, desired []resources.PortalPageResource, plan *Plan,
) error {
	// Skip if no pages to plan
	if len(desired) == 0 {
		return nil
	}

	// Fetch existing pages for this portal
	existingPages := make([]state.PortalPage, 0)
	if portalID != "" {
		pages, err := p.client.ListManagedPortalPages(ctx, portalID)
		if err != nil {
			// If portal doesn't exist yet, that's ok - we'll create pages after portal is created
			if !strings.Contains(err.Error(), "not found") {
				return fmt.Errorf("failed to list portal pages: %w", err)
			}
		} else {
			existingPages = pages
		}
	}

	// Build maps for matching
	// Map parent page ref to list of existing child pages by slug
	existingByParentAndSlug := make(map[string]map[string]state.PortalPage)
	// Also build a map from slug to page for easy lookup
	existingBySlug := make(map[string]state.PortalPage)
	
	for _, page := range existingPages {
		parentKey := page.ParentPageID
		if parentKey == "" {
			parentKey = "root"
		}
		
		if existingByParentAndSlug[parentKey] == nil {
			existingByParentAndSlug[parentKey] = make(map[string]state.PortalPage)
		}
		
		// Normalize slug by stripping leading slash for matching
		normalizedSlug := strings.TrimPrefix(page.Slug, "/")
		existingByParentAndSlug[parentKey][normalizedSlug] = page
		existingBySlug[normalizedSlug] = page
	}

	// Note: We don't have refs for existing pages, so we match by structure (slug + parent)

	// Process desired pages
	for _, desiredPage := range desired {
		// Determine parent key for matching
		parentKey := "root"
		if desiredPage.ParentPageRef != "" {
			// Try to resolve parent ref to ID using existing pages
			// The parent page should have a matching slug
			if parentPage, found := existingBySlug[desiredPage.ParentPageRef]; found {
				parentKey = parentPage.ID
			} else {
				// Parent doesn't exist yet - it might be created in this execution
				// Use the ref as the key for now
				parentKey = desiredPage.ParentPageRef
			}
		}

		// Check if page exists at this level
		// Normalize desired slug for matching (strip leading slash if present)
		normalizedDesiredSlug := strings.TrimPrefix(desiredPage.Slug, "/")
		var existingPage *state.PortalPage
		if siblings, hasParent := existingByParentAndSlug[parentKey]; hasParent {
			if existing, found := siblings[normalizedDesiredSlug]; found {
				existingPage = &existing
			}
		}

		if existingPage == nil {
			// CREATE new page
			p.planPortalPageCreate(desiredPage, portalRef, plan)
		} else {
			// Check if UPDATE is needed - must fetch full content first
			if portalID != "" && existingPage.ID != "" {
				fullPage, err := p.client.GetPortalPage(ctx, portalID, existingPage.ID)
				if err != nil {
					return fmt.Errorf("failed to fetch portal page %s for comparison: %w", existingPage.ID, err)
				}
				
				needsUpdate, updateFields := p.shouldUpdatePortalPage(fullPage, desiredPage)
				if needsUpdate {
					p.planPortalPageUpdate(*existingPage, desiredPage, portalRef, updateFields, plan)
				}
			}
		}
	}

	return nil
}

func (p *Planner) planPortalPageCreate(
	page resources.PortalPageResource, _ string, plan *Plan,
) {
	fields := make(map[string]interface{})
	fields["slug"] = page.Slug
	fields["content"] = page.Content
	
	if page.Title != nil {
		fields["title"] = *page.Title
	}
	
	if page.Visibility != nil {
		fields["visibility"] = string(*page.Visibility)
	}
	
	if page.Status != nil {
		fields["status"] = string(*page.Status)
	}
	
	if page.Description != nil {
		fields["description"] = *page.Description
	}
	
	if page.ParentPageID != nil {
		fields["parent_page_id"] = *page.ParentPageID
	}

	// Determine dependencies - depends on parent portal
	var dependencies []string
	if page.Portal != "" {
		// Find the change ID for the parent portal
		for _, change := range plan.Changes {
			if change.ResourceType == "portal" && change.ResourceRef == page.Portal {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, page.GetRef()),
		ResourceType: "portal_page",
		ResourceRef:  page.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependencies,
	}

	// Store parent portal reference
	if page.Portal != "" {
		// Find the portal in desiredPortals to get its name
		var portalName string
		for _, portal := range p.desiredPortals {
			if portal.Ref == page.Portal {
				portalName = portal.Name
				break
			}
		}
		
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: page.Portal,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	// Handle parent page reference
	if page.ParentPageRef != "" {
		// Add dependency on parent page
		for _, depChange := range plan.Changes {
			if depChange.ResourceType == "portal_page" && depChange.ResourceRef == page.ParentPageRef {
				change.DependsOn = append(change.DependsOn, depChange.ID)
				break
			}
		}
		
		// Build parent path to help with resolution
		// Get all desired pages from the planner
		allPages := make([]resources.PortalPageResource, 0)
		for _, portal := range p.desiredPortals {
			if portal.Ref == page.Portal {
				allPages = append(allPages, portal.Pages...)
				break
			}
		}
		// Also include pages at root level
		allPages = append(allPages, p.desiredPortalPages...)
		
		parentPath := p.buildParentPath(page.ParentPageRef, allPages)
		
		// Store parent page reference for resolution
		if change.References == nil {
			change.References = make(map[string]ReferenceInfo)
		}
		change.References["parent_page_id"] = ReferenceInfo{
			Ref: page.ParentPageRef,
			LookupFields: map[string]string{
				"parent_path": parentPath,
			},
		}
	}

	plan.AddChange(change)
}

// shouldUpdatePortalPage checks if a portal page needs updating
func (p *Planner) shouldUpdatePortalPage(
	current *state.PortalPage,
	desired resources.PortalPageResource,
) (bool, map[string]interface{}) {
	updates := make(map[string]interface{})

	// Compare content (always present)
	if current.Content != desired.Content {
		updates["content"] = desired.Content
	}

	// Compare title if set
	if desired.Title != nil && current.Title != *desired.Title {
		updates["title"] = *desired.Title
	}

	// Compare description if set
	if desired.Description != nil && current.Description != *desired.Description {
		updates["description"] = *desired.Description
	}

	// Compare visibility if set
	if desired.Visibility != nil {
		desiredVis := string(*desired.Visibility)
		if current.Visibility != desiredVis {
			updates["visibility"] = desiredVis
		}
	}

	// Compare status if set
	if desired.Status != nil {
		desiredStatus := string(*desired.Status)
		if current.Status != desiredStatus {
			updates["status"] = desiredStatus
		}
	}

	// Note: We don't update slug or parent_page_id as these would effectively be a different page

	return len(updates) > 0, updates
}

// planPortalPageUpdate creates an UPDATE change for a portal page
func (p *Planner) planPortalPageUpdate(
	current state.PortalPage,
	desired resources.PortalPageResource,
	portalRef string,
	updateFields map[string]interface{},
	plan *Plan,
) {
	fields := make(map[string]interface{})

	// Always include slug for identification
	fields["slug"] = current.Slug

	// Add fields that need updating
	for field, value := range updateFields {
		fields[field] = value
	}

	// Determine dependencies - depends on parent portal
	var dependencies []string
	if portalRef != "" {
		// Find the change ID for the parent portal
		for _, change := range plan.Changes {
			if change.ResourceType == "portal" && change.ResourceRef == portalRef {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, desired.GetRef()),
		ResourceType: "portal_page",
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    dependencies,
	}

	// Store parent portal reference
	if portalRef != "" {
		// Find the portal in desiredPortals to get its name
		var portalName string
		for _, portal := range p.desiredPortals {
			if portal.Ref == portalRef {
				portalName = portal.Name
				break
			}
		}
		
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: portalRef,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	plan.AddChange(change)
}

// buildParentPath constructs the full slug path for a page ref
func (p *Planner) buildParentPath(pageRef string, allPages []resources.PortalPageResource) string {
	pathSegments := []string{}
	current := pageRef
	
	// Build path from bottom up
	for current != "" {
		found := false
		for _, page := range allPages {
			if page.GetRef() == current {
				pathSegments = append([]string{page.Slug}, pathSegments...)
				current = page.ParentPageRef
				found = true
				break
			}
		}
		if !found {
			break // Avoid infinite loop
		}
	}
	
	return strings.Join(pathSegments, "/")
}

// Portal Snippet planning

func (p *Planner) planPortalSnippetsChanges(
	_ context.Context, desired []resources.PortalSnippetResource, plan *Plan,
) error { //nolint:unparam // Will return errors when state fetching is added
	// Skip if no snippets to plan
	if len(desired) == 0 {
		return nil
	}

	// TODO: In the future, we should fetch current snippets and compare
	// For now, just create all desired snippets

	// For each desired snippet
	for _, desiredSnippet := range desired {
		p.planPortalSnippetCreate(desiredSnippet, plan)
	}

	return nil
}

func (p *Planner) planPortalSnippetCreate(
	snippet resources.PortalSnippetResource, plan *Plan,
) {
	fields := make(map[string]interface{})
	fields["name"] = snippet.Name
	fields["content"] = snippet.Content

	// Determine dependencies - depends on parent portal
	var dependencies []string
	if snippet.Portal != "" {
		// Find the change ID for the parent portal
		for _, change := range plan.Changes {
			if change.ResourceType == "portal" && change.ResourceRef == snippet.Portal {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, snippet.GetRef()),
		ResourceType: "portal_snippet",
		ResourceRef:  snippet.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependencies,
	}

	// Store parent portal reference
	if snippet.Portal != "" {
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: snippet.Portal,
			},
		}
	}

	plan.AddChange(change)
}