package planner

import (
	"context"

	"github.com/kong/kongctl/internal/declarative/resources"
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
	_ context.Context, desired []resources.PortalPageResource, plan *Plan,
) error { //nolint:unparam // Will return errors when state fetching is added
	// Skip if no pages to plan
	if len(desired) == 0 {
		return nil
	}

	// TODO: In the future, we should fetch current pages and compare
	// For now, just create all desired pages

	// For each desired page
	for _, desiredPage := range desired {
		p.planPortalPageCreate(desiredPage, plan)
	}

	return nil
}

func (p *Planner) planPortalPageCreate(
	page resources.PortalPageResource, plan *Plan,
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
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: page.Portal,
			},
		}
	}

	plan.AddChange(change)
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