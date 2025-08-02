package planner

import (
	"context"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// Portal Customization planning

func (p *Planner) planPortalCustomizationsChanges(
	ctx context.Context, parentNamespace string, desired []resources.PortalCustomizationResource, plan *Plan,
) error { //nolint:unparam // Will return errors in future enhancements
	// Get existing portals to check current customization
	// Use context to get namespace filter for API calls
	namespace, ok := ctx.Value(NamespaceContextKey).(string)
	if !ok {
		namespace = "*"
	}
	namespaceFilter := []string{namespace}
	existingPortals, _ := p.client.ListManagedPortals(ctx, namespaceFilter)
	portalNameToID := make(map[string]string)
	for _, portal := range existingPortals {
		portalNameToID[portal.Name] = portal.ID
	}

	// For each desired customization
	for _, desiredCustomization := range desired {
		// Find the portal ID
		var portalID string
		var portalName string
		for _, portal := range p.desiredPortals {
			if portal.Ref == desiredCustomization.Portal {
				portalName = portal.Name
				portalID = portalNameToID[portalName]
				break
			}
		}

		// If portal exists, fetch current customization and compare
		if portalID != "" {
			current, err := p.client.GetPortalCustomization(ctx, portalID)
			if err != nil {
				// If portal customization API is not configured, skip processing
				if strings.Contains(err.Error(), "portal customization API not configured") {
					continue
				}
				// If we can't fetch current state, plan the update anyway
				p.planPortalCustomizationUpdate(parentNamespace, desiredCustomization, portalName, portalID, plan)
				continue
			}

			// Compare and only update if needed
			needsUpdate, updateFields := p.shouldUpdatePortalCustomization(current, desiredCustomization)
			if needsUpdate {
				p.planPortalCustomizationUpdateWithFields(
					parentNamespace, desiredCustomization, portalName, portalID, updateFields, plan,
				)
			}
		} else {
			// Portal doesn't exist yet, plan the update for after portal creation
			p.planPortalCustomizationUpdate(parentNamespace, desiredCustomization, portalName, "", plan)
		}
	}

	return nil
}

func (p *Planner) planPortalCustomizationUpdate(
	parentNamespace string, customization resources.PortalCustomizationResource,
	portalName string, portalID string, plan *Plan,
) {
	// Build all fields from the resource
	fields := p.buildAllCustomizationFields(customization)
	p.planPortalCustomizationUpdateWithFields(parentNamespace, customization, portalName, portalID, fields, plan)
}

func (p *Planner) planPortalCustomizationUpdateWithFields(
	parentNamespace string, customization resources.PortalCustomizationResource, portalName string, portalID string,
	fields map[string]interface{}, plan *Plan,
) {
	// Only proceed if there are fields to update
	if len(fields) == 0 {
		return
	}

	// Determine dependencies - depends on parent portal
	var dependencies []string
	if customization.Portal != "" {
		// Find the change ID for the parent portal
		for _, change := range plan.Changes {
			if change.ResourceType == ResourceTypePortal && change.ResourceRef == customization.Portal {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	// Portal customization is a singleton resource - always use UPDATE action
	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypePortalCustomization, customization.Ref),
		ResourceType: ResourceTypePortalCustomization,
		ResourceRef:  customization.Ref,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    dependencies,
		Namespace:    parentNamespace,
	}

	// Store parent portal reference
	if customization.Portal != "" {
		// Set Parent field for proper display and serialization
		change.Parent = &ParentInfo{
			Ref: customization.Portal,
			ID:  portalID, // May be empty if portal doesn't exist yet
		}
		
		// Also store in References for executor to use
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: customization.Portal,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	plan.AddChange(change)
}

// shouldUpdatePortalCustomization compares current and desired customization
func (p *Planner) shouldUpdatePortalCustomization(
	current *kkComps.PortalCustomization,
	desired resources.PortalCustomizationResource,
) (bool, map[string]interface{}) {
	updates := make(map[string]interface{})

	// Compare theme
	if !p.compareTheme(current.Theme, desired.Theme) {
		if desired.Theme != nil {
			updates["theme"] = p.buildThemeFields(desired.Theme)
		}
	}

	// Compare layout
	if !p.compareStringPtr(current.Layout, desired.Layout) {
		if desired.Layout != nil {
			updates["layout"] = *desired.Layout
		}
	}

	// Compare CSS
	if !p.compareStringPtr(current.CSS, desired.CSS) {
		if desired.CSS != nil {
			updates["css"] = *desired.CSS
		}
	}

	// Compare menu
	if !p.compareMenu(current.Menu, desired.Menu) {
		if desired.Menu != nil {
			updates["menu"] = p.buildMenuFields(desired.Menu)
		}
	}

	return len(updates) > 0, updates
}

// buildAllCustomizationFields builds all fields from the customization resource
func (p *Planner) buildAllCustomizationFields(
	customization resources.PortalCustomizationResource,
) map[string]interface{} {
	fields := make(map[string]interface{})

	// Add theme settings if present
	if customization.Theme != nil {
		fields["theme"] = p.buildThemeFields(customization.Theme)
	}

	// Add layout if present
	if customization.Layout != nil {
		fields["layout"] = *customization.Layout
	}

	// Add CSS if present
	if customization.CSS != nil {
		fields["css"] = *customization.CSS
	}

	// Add menu settings if present
	if customization.Menu != nil {
		fields["menu"] = p.buildMenuFields(customization.Menu)
	}

	return fields
}

// buildThemeFields constructs theme fields map from theme object
func (p *Planner) buildThemeFields(theme *kkComps.Theme) map[string]interface{} {
	themeFields := make(map[string]interface{})
	
	// Add mode if present
	if theme.Mode != nil {
		themeFields["mode"] = string(*theme.Mode)
	}
	
	// Add name if present
	if theme.Name != nil {
		themeFields["name"] = *theme.Name
	}
	
	// Add colors if present
	if theme.Colors != nil {
		colorsFields := make(map[string]interface{})
		if theme.Colors.Primary != nil {
			colorsFields["primary"] = *theme.Colors.Primary
		}
		themeFields["colors"] = colorsFields
	}
	
	return themeFields
}

// buildMenuFields constructs menu fields map from menu object
func (p *Planner) buildMenuFields(menu *kkComps.Menu) map[string]interface{} {
	menuFields := make(map[string]interface{})
	
	// Add main menu items
	if menu.Main != nil {
		var mainMenuItems []map[string]interface{}
		for _, item := range menu.Main {
			menuItem := map[string]interface{}{
				"path":       item.Path,
				"title":      item.Title,
				"external":   item.External,
				"visibility": string(item.Visibility),
			}
			mainMenuItems = append(mainMenuItems, menuItem)
		}
		menuFields["main"] = mainMenuItems
	}
	
	// Add footer sections
	if menu.FooterSections != nil {
		var footerSections []map[string]interface{}
		for _, section := range menu.FooterSections {
			var items []map[string]interface{}
			for _, item := range section.Items {
				menuItem := map[string]interface{}{
					"path":       item.Path,
					"title":      item.Title,
					"external":   item.External,
					"visibility": string(item.Visibility),
				}
				items = append(items, menuItem)
			}
			sectionMap := map[string]interface{}{
				"title": section.Title,
				"items": items,
			}
			footerSections = append(footerSections, sectionMap)
		}
		menuFields["footer_sections"] = footerSections
	}
	
	return menuFields
}

// compareTheme does deep comparison of theme objects
func (p *Planner) compareTheme(current, desired *kkComps.Theme) bool {
	if current == nil && desired == nil {
		return true
	}
	if current == nil || desired == nil {
		return false
	}
	
	// Compare mode
	if !p.compareModePtr(current.Mode, desired.Mode) {
		return false
	}
	
	// Compare name
	if !p.compareStringPtr(current.Name, desired.Name) {
		return false
	}
	
	// Compare colors
	if current.Colors == nil && desired.Colors == nil {
		return true
	}
	if current.Colors == nil || desired.Colors == nil {
		return false
	}
	
	return p.compareStringPtr(current.Colors.Primary, desired.Colors.Primary)
}

// compareMenu does deep comparison of menu objects
func (p *Planner) compareMenu(current, desired *kkComps.Menu) bool {
	if current == nil && desired == nil {
		return true
	}
	if current == nil || desired == nil {
		return false
	}
	
	// Compare main menu items
	if len(current.Main) != len(desired.Main) {
		return false
	}
	for i, currentItem := range current.Main {
		desiredItem := desired.Main[i]
		if currentItem.Path != desiredItem.Path ||
			currentItem.Title != desiredItem.Title ||
			currentItem.External != desiredItem.External ||
			currentItem.Visibility != desiredItem.Visibility {
			return false
		}
	}
	
	// Compare footer sections
	if len(current.FooterSections) != len(desired.FooterSections) {
		return false
	}
	for i, currentSection := range current.FooterSections {
		desiredSection := desired.FooterSections[i]
		if currentSection.Title != desiredSection.Title ||
			len(currentSection.Items) != len(desiredSection.Items) {
			return false
		}
		
		// Compare items in section
		for j, currentItem := range currentSection.Items {
			desiredItem := desiredSection.Items[j]
			if currentItem.Path != desiredItem.Path ||
				currentItem.Title != desiredItem.Title ||
				currentItem.External != desiredItem.External ||
				currentItem.Visibility != desiredItem.Visibility {
				return false
			}
		}
	}
	
	return true
}

// compareStringPtr compares two string pointers
func (p *Planner) compareStringPtr(current, desired *string) bool {
	if current == nil && desired == nil {
		return true
	}
	if current == nil || desired == nil {
		return false
	}
	return *current == *desired
}

// compareModePtr compares two Mode pointers
func (p *Planner) compareModePtr(current, desired *kkComps.Mode) bool {
	if current == nil && desired == nil {
		return true
	}
	if current == nil || desired == nil {
		return false
	}
	return *current == *desired
}

// Portal Custom Domain planning

func (p *Planner) planPortalCustomDomainsChanges(
	_ context.Context, parentNamespace string, desired []resources.PortalCustomDomainResource, plan *Plan,
) error { //nolint:unparam // Will return errors when state fetching is added
	// For each desired custom domain
	for _, desiredDomain := range desired {
		// Portal custom domains are singleton resources per portal
		// Always create/update, never fetch current state
		p.planPortalCustomDomainCreate(parentNamespace, desiredDomain, plan)
	}

	return nil
}

func (p *Planner) planPortalCustomDomainCreate(
	parentNamespace string, domain resources.PortalCustomDomainResource, plan *Plan,
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
			if change.ResourceType == ResourceTypePortal && change.ResourceRef == domain.Portal {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypePortalCustomDomain, domain.Ref),
		ResourceType: ResourceTypePortalCustomDomain,
		ResourceRef:  domain.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependencies,
		Namespace:    parentNamespace,
	}

	// Store parent portal reference
	if domain.Portal != "" {
		// Find the portal in desiredPortals to get its name
		var portalName string
		for _, portal := range p.desiredPortals {
			if portal.Ref == domain.Portal {
				portalName = portal.Name
				break
			}
		}
		
		// Set Parent field for proper display and serialization
		change.Parent = &ParentInfo{
			Ref: domain.Portal,
			ID:  "", // Will be resolved during execution
		}
		
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: domain.Portal,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	plan.AddChange(change)
}

// Portal Page planning

func (p *Planner) planPortalPagesChanges(
	ctx context.Context, parentNamespace string, portalID string, portalRef string,
	desired []resources.PortalPageResource, plan *Plan,
) error {
	// Fetch existing pages for this portal
	existingPages := make([]state.PortalPage, 0)
	if portalID != "" {
		pages, err := p.client.ListManagedPortalPages(ctx, portalID)
		if err != nil {
			// If portal page API is not configured, skip processing
			// This happens in tests or when portal pages feature is not available
			if strings.Contains(err.Error(), "portal page API not configured") {
				// In sync mode with no desired pages, this is OK - nothing to delete
				if plan.Metadata.Mode == PlanModeSync && len(desired) == 0 {
					return nil
				}
				// But if there are desired pages, we need the API
				if len(desired) > 0 {
					return fmt.Errorf("failed to list portal pages: %w", err)
				}
				return nil
			}
			// If portal doesn't exist yet, that's ok - we'll create pages after portal is created
			if !strings.Contains(err.Error(), "not found") {
				return fmt.Errorf("failed to list portal pages: %w", err)
			}
		} else {
			existingPages = pages
		}
	}

	// Build maps for matching
	// Build map from full slug path to page
	existingByPath := make(map[string]state.PortalPage)
	existingByID := make(map[string]state.PortalPage)
	
	// First, index all pages by ID for easy lookup
	for _, page := range existingPages {
		existingByID[page.ID] = page
	}
	
	// Helper to build full path for a page
	var getPagePath func(pageID string) string
	pageIDToPath := make(map[string]string) // cache to avoid recalculation
	
	getPagePath = func(pageID string) string {
		// Check cache first
		if path, cached := pageIDToPath[pageID]; cached {
			return path
		}
		
		page, exists := existingByID[pageID]
		if !exists {
			return ""
		}
		
		// Special handling for root page with slug "/"
		normalizedSlug := page.Slug
		if page.Slug != "/" {
			normalizedSlug = strings.TrimPrefix(page.Slug, "/")
		}
		
		// Root page - path is just the slug
		if page.ParentPageID == "" {
			pageIDToPath[pageID] = normalizedSlug
			return normalizedSlug
		}
		
		// Child page - build full path recursively
		parentPath := getPagePath(page.ParentPageID)
		if parentPath == "" {
			// Parent not found, use slug only
			pageIDToPath[pageID] = normalizedSlug
			return normalizedSlug
		}
		
		fullPath := parentPath + "/" + normalizedSlug
		pageIDToPath[pageID] = fullPath
		return fullPath
	}
	
	// Build the path map for all existing pages
	for _, page := range existingPages {
		path := getPagePath(page.ID)
		if path != "" {
			existingByPath[path] = page
		}
	}

	// Note: We don't have refs for existing pages, so we match by full slug paths

	// Process desired pages
	for _, desiredPage := range desired {
		// Build the full path for this desired page to check if it exists
		var fullPath string
		// Special handling for root page with slug "/"
		normalizedDesiredSlug := desiredPage.Slug
		if desiredPage.Slug != "/" {
			normalizedDesiredSlug = strings.TrimPrefix(desiredPage.Slug, "/")
		}
		
		if desiredPage.ParentPageRef == "" {
			// Root page
			fullPath = normalizedDesiredSlug
		} else {
			// Child page - build parent path first
			parentPath := p.buildParentPath(desiredPage.ParentPageRef, desired)
			if parentPath != "" {
				fullPath = parentPath + "/" + normalizedDesiredSlug
			} else {
				// Parent path couldn't be built, use slug only
				fullPath = normalizedDesiredSlug
			}
		}
		
		// Check if page exists by full path
		existingPage, exists := existingByPath[fullPath]
		
		if !exists {
			// CREATE new page
			p.planPortalPageCreate(parentNamespace, desiredPage, portalRef, portalID, plan)
		} else {
			// Check if UPDATE is needed - must fetch full content first
			if portalID != "" && existingPage.ID != "" {
				fullPage, err := p.client.GetPortalPage(ctx, portalID, existingPage.ID)
				if err != nil {
					return fmt.Errorf("failed to fetch portal page %s for comparison: %w", existingPage.ID, err)
				}
				
				needsUpdate, updateFields := p.shouldUpdatePortalPage(fullPage, desiredPage)
				if needsUpdate {
					p.planPortalPageUpdate(parentNamespace, existingPage, desiredPage, portalRef, updateFields, plan)
				}
			}
		}
	}

	// In sync mode, delete pages that exist but are not in desired state
	if plan.Metadata.Mode == PlanModeSync {
		// Build set of desired page paths
		desiredPaths := make(map[string]bool)
		for _, desiredPage := range desired {
			// Build the full path for this desired page
			var fullPath string
			// Special handling for root page with slug "/"
			normalizedDesiredSlug := desiredPage.Slug
			if desiredPage.Slug != "/" {
				normalizedDesiredSlug = strings.TrimPrefix(desiredPage.Slug, "/")
			}
			
			if desiredPage.ParentPageRef == "" {
				// Root page
				fullPath = normalizedDesiredSlug
			} else {
				// Child page - build parent path first
				parentPath := p.buildParentPath(desiredPage.ParentPageRef, desired)
				if parentPath != "" {
					fullPath = parentPath + "/" + normalizedDesiredSlug
				} else {
					// Parent path couldn't be built, use slug only
					fullPath = normalizedDesiredSlug
				}
			}
			
			desiredPaths[fullPath] = true
		}

		// Find pages to delete
		for path, existingPage := range existingByPath {
			if !desiredPaths[path] {
				p.planPortalPageDelete(portalRef, portalID, existingPage.ID, existingPage.Slug, plan)
			}
		}
	}

	return nil
}

func (p *Planner) planPortalPageCreate(
	parentNamespace string, page resources.PortalPageResource, _ string, portalID string, plan *Plan,
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
			if change.ResourceType == ResourceTypePortal && change.ResourceRef == page.Portal {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypePortalPage, page.GetRef()),
		ResourceType: ResourceTypePortalPage,
		ResourceRef:  page.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependencies,
		Namespace:    parentNamespace,
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
		
		// Set Parent field for proper display and serialization
		change.Parent = &ParentInfo{
			Ref: page.Portal,
			ID:  portalID, // May be empty if portal doesn't exist yet
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
			if depChange.ResourceType == ResourceTypePortalPage && depChange.ResourceRef == page.ParentPageRef {
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
	parentNamespace string,
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
			if change.ResourceType == ResourceTypePortal && change.ResourceRef == portalRef {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypePortalPage, desired.GetRef()),
		ResourceType: ResourceTypePortalPage,
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    dependencies,
		Namespace:    parentNamespace,
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
		
		// Set Parent field for proper display and serialization
		change.Parent = &ParentInfo{
			Ref: portalRef,
			ID:  "", // Already known via ResourceID but not needed for display
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

// planPortalPageDelete creates a DELETE change for a portal page
func (p *Planner) planPortalPageDelete(
	portalRef string, portalID string, pageID string, slug string, plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypePortalPage, pageID),
		ResourceType: ResourceTypePortalPage,
		ResourceRef:  "[unknown]",
		ResourceID:   pageID,
		ResourceMonikers: map[string]string{
			"slug":         slug,
			"parent_portal": portalRef,
		},
		Parent:    &ParentInfo{Ref: portalRef, ID: portalID},
		Action:    ActionDelete,
		Fields:    map[string]interface{}{"slug": slug},
		DependsOn: []string{},
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
	ctx context.Context, parentNamespace string, portalID string, portalRef string,
	desired []resources.PortalSnippetResource, plan *Plan,
) error {
	// Fetch existing snippets for this portal
	existingSnippets := make(map[string]state.PortalSnippet)
	if portalID != "" {
		snippets, err := p.client.ListPortalSnippets(ctx, portalID)
		if err != nil {
			// If portal snippet API is not configured, skip processing
			if strings.Contains(err.Error(), "portal snippet API not configured") {
				// In sync mode with no desired snippets, this is OK - nothing to delete
				if plan.Metadata.Mode == PlanModeSync && len(desired) == 0 {
					return nil
				}
				// But if there are desired snippets, we need the API
				if len(desired) > 0 {
					return fmt.Errorf("failed to list portal snippets: %w", err)
				}
				return nil
			}
			// If portal doesn't exist yet, that's ok - we'll create snippets after portal is created
			if !strings.Contains(err.Error(), "not found") {
				return fmt.Errorf("failed to list portal snippets: %w", err)
			}
		} else {
			// Build map by name for matching
			for _, snippet := range snippets {
				existingSnippets[snippet.Name] = snippet
			}
		}
	}

	// Process desired snippets
	for _, desiredSnippet := range desired {
		// Check if snippet exists by name
		if existingSnippet, exists := existingSnippets[desiredSnippet.Name]; exists {
			// Check if UPDATE is needed - must fetch full content first
			if portalID != "" && existingSnippet.ID != "" {
				fullSnippet, err := p.client.GetPortalSnippet(ctx, portalID, existingSnippet.ID)
				if err != nil {
					return fmt.Errorf("failed to fetch portal snippet %s for comparison: %w", existingSnippet.ID, err)
				}
				
				needsUpdate, updateFields := p.shouldUpdatePortalSnippet(fullSnippet, desiredSnippet)
				if needsUpdate {
					p.planPortalSnippetUpdate(parentNamespace, existingSnippet, desiredSnippet, portalRef, updateFields, plan)
				}
			}
		} else {
			// CREATE new snippet
			p.planPortalSnippetCreate(parentNamespace, desiredSnippet, plan)
		}
	}

	return nil
}

func (p *Planner) planPortalSnippetCreate(
	parentNamespace string, snippet resources.PortalSnippetResource, plan *Plan,
) {
	fields := make(map[string]interface{})
	fields["name"] = snippet.Name
	fields["content"] = snippet.Content
	
	// Include optional fields if present
	if snippet.Title != nil {
		fields["title"] = *snippet.Title
	}
	if snippet.Visibility != nil {
		fields["visibility"] = string(*snippet.Visibility)
	}
	if snippet.Status != nil {
		fields["status"] = string(*snippet.Status)
	}
	if snippet.Description != nil {
		fields["description"] = *snippet.Description
	}

	// Determine dependencies - depends on parent portal
	var dependencies []string
	if snippet.Portal != "" {
		// Find the change ID for the parent portal
		for _, change := range plan.Changes {
			if change.ResourceType == ResourceTypePortal && change.ResourceRef == snippet.Portal {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypePortalSnippet, snippet.GetRef()),
		ResourceType: ResourceTypePortalSnippet,
		ResourceRef:  snippet.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependencies,
		Namespace:    parentNamespace,
	}

	// Store parent portal reference
	if snippet.Portal != "" {
		// Find the portal in desiredPortals to get its name
		var portalName string
		for _, portal := range p.desiredPortals {
			if portal.Ref == snippet.Portal {
				portalName = portal.Name
				break
			}
		}
		
		// Set Parent field for proper display and serialization
		change.Parent = &ParentInfo{
			Ref: snippet.Portal,
			ID:  "", // Will be resolved during execution
		}
		
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: snippet.Portal,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	plan.AddChange(change)
}// shouldUpdatePortalSnippet checks if a portal snippet needs updating
func (p *Planner) shouldUpdatePortalSnippet(
	current *state.PortalSnippet,
	desired resources.PortalSnippetResource,
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

	// Note: We don't update name as that's the identifier

	return len(updates) > 0, updates
}

// planPortalSnippetUpdate creates an UPDATE change for a portal snippet
func (p *Planner) planPortalSnippetUpdate(
	parentNamespace string,
	current state.PortalSnippet,
	desired resources.PortalSnippetResource,
	portalRef string,
	updateFields map[string]interface{},
	plan *Plan,
) {
	fields := make(map[string]interface{})

	// Always include name for identification
	fields["name"] = current.Name

	// Add fields that need updating
	for field, value := range updateFields {
		fields[field] = value
	}

	// Determine dependencies - depends on parent portal
	var dependencies []string
	if portalRef != "" {
		// Find the change ID for the parent portal
		for _, change := range plan.Changes {
			if change.ResourceType == ResourceTypePortal && change.ResourceRef == portalRef {
				dependencies = append(dependencies, change.ID)
				break
			}
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypePortalSnippet, desired.GetRef()),
		ResourceType: ResourceTypePortalSnippet,
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    dependencies,
		Namespace:    parentNamespace,
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