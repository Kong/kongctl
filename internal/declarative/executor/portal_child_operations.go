package executor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/log"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// Portal Customization operations (singleton resource)

// updatePortalCustomization handles both CREATE and UPDATE operations for portal customization
// Since customization is a singleton resource that always exists, we always use update
func (e *Executor) updatePortalCustomization(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Get logger from context
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	
	// Get portal ID from references
	portalID := ""
	if portalRef, ok := change.References["portal_id"]; ok {
		if portalRef.ID != "" {
			portalID = portalRef.ID
		} else {
			// Need to resolve portal reference
			resolvedID, err := e.resolvePortalRef(ctx, portalRef.Ref)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalID = resolvedID
		}
	}
	
	if portalID == "" {
		return "", fmt.Errorf("portal ID is required for customization")
	}
	
	logger.Debug("Updating portal customization",
		slog.String("portal_id", portalID),
		slog.Any("fields", change.Fields))
	
	// Build customization object
	var customization kkComps.PortalCustomization
	
	// Handle theme
	if themeData, ok := change.Fields["theme"].(map[string]interface{}); ok {
		theme := &kkComps.Theme{}
		
		if name, ok := themeData["name"].(string); ok {
			theme.Name = &name
		}
		if mode, ok := themeData["mode"].(string); ok {
			modeValue := kkComps.Mode(mode)
			theme.Mode = &modeValue
		}
		
		// Handle colors
		if colorsData, ok := themeData["colors"].(map[string]interface{}); ok {
			colors := &kkComps.Colors{}
			if primary, ok := colorsData["primary"].(string); ok {
				colors.Primary = &primary
			}
			theme.Colors = colors
		}
		
		customization.Theme = theme
	}
	
	// Handle layout
	if layout, ok := change.Fields["layout"].(string); ok {
		customization.Layout = &layout
	}
	
	// Handle CSS
	if css, ok := change.Fields["css"].(string); ok {
		customization.CSS = &css
	}
	
	// Handle menu
	if menuData, ok := change.Fields["menu"].(map[string]interface{}); ok {
		menu := &kkComps.Menu{}
		
		if mainItems, ok := menuData["main"].([]interface{}); ok {
			var mainMenu []kkComps.PortalMenuItem
			for _, item := range mainItems {
				if itemMap, ok := item.(map[string]interface{}); ok {
					menuItem := kkComps.PortalMenuItem{
						Path:  itemMap["path"].(string),
						Title: itemMap["title"].(string),
					}
					
					if visibility, ok := itemMap["visibility"].(string); ok {
						visValue := kkComps.Visibility(visibility)
						menuItem.Visibility = visValue
					}
					if external, ok := itemMap["external"].(bool); ok {
						menuItem.External = external
					}
					
					mainMenu = append(mainMenu, menuItem)
				}
			}
			menu.Main = mainMenu
		}
		
		customization.Menu = menu
	}
	
	// Update the customization
	err := e.client.UpdatePortalCustomization(ctx, portalID, customization)
	if err != nil {
		return "", fmt.Errorf("failed to update portal customization: %w", err)
	}
	
	// Portal customization doesn't return an ID, use portal ID instead
	return portalID, nil
}

// Portal Custom Domain operations

// createPortalCustomDomain handles CREATE operations for portal custom domains
func (e *Executor) createPortalCustomDomain(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Get logger from context
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	
	// Get portal ID from references
	portalID := ""
	if portalRef, ok := change.References["portal_id"]; ok {
		if portalRef.ID != "" {
			portalID = portalRef.ID
		} else {
			// Need to resolve portal reference
			resolvedID, err := e.resolvePortalRef(ctx, portalRef.Ref)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalID = resolvedID
		}
	}
	
	if portalID == "" {
		return "", fmt.Errorf("portal ID is required for custom domain")
	}
	
	logger.Debug("Creating portal custom domain",
		slog.String("portal_id", portalID),
		slog.Any("fields", change.Fields))
	
	// Build request
	req := kkComps.CreatePortalCustomDomainRequest{
		Hostname: change.Fields["hostname"].(string),
		Enabled:  change.Fields["enabled"].(bool),
	}
	
	// Handle SSL settings
	if sslData, ok := change.Fields["ssl"].(map[string]interface{}); ok {
		ssl := kkComps.CreatePortalCustomDomainSSL{}
		if method, ok := sslData["domain_verification_method"].(string); ok {
			ssl.DomainVerificationMethod = kkComps.PortalCustomDomainVerificationMethod(method)
		}
		req.Ssl = ssl
	}
	
	// Create the custom domain
	err := e.client.CreatePortalCustomDomain(ctx, portalID, req)
	if err != nil {
		return "", fmt.Errorf("failed to create portal custom domain: %w", err)
	}
	
	// Custom domain doesn't return an ID, use portal ID instead
	return portalID, nil
}

// updatePortalCustomDomain handles UPDATE operations for portal custom domains
func (e *Executor) updatePortalCustomDomain(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Get logger from context
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	
	// Get portal ID from references or resource ID
	portalID := change.ResourceID
	if portalID == "" && change.References != nil {
		if portalRef, ok := change.References["portal_id"]; ok {
			if portalRef.ID != "" {
				portalID = portalRef.ID
			} else {
				// Need to resolve portal reference
				resolvedID, err := e.resolvePortalRef(ctx, portalRef.Ref)
				if err != nil {
					return "", fmt.Errorf("failed to resolve portal reference: %w", err)
				}
				portalID = resolvedID
			}
		}
	}
	
	if portalID == "" {
		return "", fmt.Errorf("portal ID is required for custom domain update")
	}
	
	logger.Debug("Updating portal custom domain",
		slog.String("portal_id", portalID),
		slog.Any("fields", change.Fields))
	
	// Build update request
	var req kkComps.UpdatePortalCustomDomainRequest
	
	// Only update enabled field if present
	if enabled, ok := change.Fields["enabled"].(bool); ok {
		req.Enabled = &enabled
	}
	
	// Update the custom domain
	err := e.client.UpdatePortalCustomDomain(ctx, portalID, req)
	if err != nil {
		return "", fmt.Errorf("failed to update portal custom domain: %w", err)
	}
	
	return portalID, nil
}

// deletePortalCustomDomain handles DELETE operations for portal custom domains
func (e *Executor) deletePortalCustomDomain(ctx context.Context, change planner.PlannedChange) error {
	// Get portal ID
	portalID := change.ResourceID
	if portalID == "" {
		return fmt.Errorf("portal ID is required for custom domain deletion")
	}
	
	// Delete the custom domain
	err := e.client.DeletePortalCustomDomain(ctx, portalID)
	if err != nil {
		return fmt.Errorf("failed to delete portal custom domain: %w", err)
	}
	
	return nil
}

// Portal Page operations

// createPortalPage handles CREATE operations for portal pages
func (e *Executor) createPortalPage(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Get logger from context
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	
	// Get portal ID from references
	portalID := ""
	if portalRef, ok := change.References["portal_id"]; ok {
		if portalRef.ID != "" {
			portalID = portalRef.ID
		} else {
			// Need to resolve portal reference
			resolvedID, err := e.resolvePortalRef(ctx, portalRef.Ref)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalID = resolvedID
		}
	}
	
	if portalID == "" {
		return "", fmt.Errorf("portal ID is required for page creation")
	}
	
	logger.Debug("Creating portal page",
		slog.String("portal_id", portalID),
		slog.Any("fields", change.Fields))
	
	// Build request
	req := kkComps.CreatePortalPageRequest{
		Slug:    change.Fields["slug"].(string),
		Content: change.Fields["content"].(string),
	}
	
	// Handle optional fields
	if title, ok := change.Fields["title"].(string); ok {
		req.Title = &title
	}
	
	if visibilityStr, ok := change.Fields["visibility"].(string); ok {
		visibility := kkComps.PageVisibilityStatus(visibilityStr)
		req.Visibility = &visibility
	}
	
	if statusStr, ok := change.Fields["status"].(string); ok {
		status := kkComps.PublishedStatus(statusStr)
		req.Status = &status
	}
	
	if description, ok := change.Fields["description"].(string); ok {
		req.Description = &description
	}
	
	if parentPageID, ok := change.Fields["parent_page_id"].(string); ok {
		req.ParentPageID = &parentPageID
	}
	
	// Create the page
	pageID, err := e.client.CreatePortalPage(ctx, portalID, req)
	if err != nil {
		return "", fmt.Errorf("failed to create portal page: %w", err)
	}
	
	return pageID, nil
}

// updatePortalPage handles UPDATE operations for portal pages
func (e *Executor) updatePortalPage(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Get logger from context
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	
	// Get portal ID and page ID
	portalID := ""
	pageID := change.ResourceID
	
	if pageID == "" {
		return "", fmt.Errorf("page ID is required for update operation")
	}
	
	// Get portal ID from references or resource ID
	if change.References != nil {
		if portalRef, ok := change.References["portal_id"]; ok {
			if portalRef.ID != "" {
				portalID = portalRef.ID
			} else {
				// Need to resolve portal reference
				resolvedID, err := e.resolvePortalRef(ctx, portalRef.Ref)
				if err != nil {
					return "", fmt.Errorf("failed to resolve portal reference: %w", err)
				}
				portalID = resolvedID
			}
		}
	}
	
	if portalID == "" {
		return "", fmt.Errorf("portal ID is required for page update")
	}
	
	logger.Debug("Updating portal page",
		slog.String("portal_id", portalID),
		slog.String("page_id", pageID),
		slog.Any("fields", change.Fields))
	
	// Build update request
	var req kkComps.UpdatePortalPageRequest
	
	// Handle optional fields
	if slug, ok := change.Fields["slug"].(string); ok {
		req.Slug = &slug
	}
	
	if title, ok := change.Fields["title"].(string); ok {
		req.Title = &title
	}
	
	if content, ok := change.Fields["content"].(string); ok {
		req.Content = &content
	}
	
	if visibilityStr, ok := change.Fields["visibility"].(string); ok {
		visibility := kkComps.VisibilityStatus(visibilityStr)
		req.Visibility = &visibility
	}
	
	if statusStr, ok := change.Fields["status"].(string); ok {
		status := kkComps.PublishedStatus(statusStr)
		req.Status = &status
	}
	
	if description, ok := change.Fields["description"].(string); ok {
		req.Description = &description
	}
	
	if parentPageID, ok := change.Fields["parent_page_id"].(string); ok {
		req.ParentPageID = &parentPageID
	}
	
	// Update the page
	err := e.client.UpdatePortalPage(ctx, portalID, pageID, req)
	if err != nil {
		return "", fmt.Errorf("failed to update portal page: %w", err)
	}
	
	return pageID, nil
}

// deletePortalPage handles DELETE operations for portal pages
func (e *Executor) deletePortalPage(ctx context.Context, change planner.PlannedChange) error {
	// Get portal ID and page ID
	portalID := ""
	pageID := change.ResourceID
	
	if pageID == "" {
		return fmt.Errorf("page ID is required for deletion")
	}
	
	// Get portal ID from references
	if change.References != nil {
		if portalRef, ok := change.References["portal_id"]; ok {
			if portalRef.ID != "" {
				portalID = portalRef.ID
			} else {
				// Need to resolve portal reference
				resolvedID, err := e.resolvePortalRef(ctx, portalRef.Ref)
				if err != nil {
					return fmt.Errorf("failed to resolve portal reference: %w", err)
				}
				portalID = resolvedID
			}
		}
	}
	
	if portalID == "" {
		return fmt.Errorf("portal ID is required for page deletion")
	}
	
	// Delete the page
	err := e.client.DeletePortalPage(ctx, portalID, pageID)
	if err != nil {
		return fmt.Errorf("failed to delete portal page: %w", err)
	}
	
	return nil
}

// Portal Snippet operations

// createPortalSnippet handles CREATE operations for portal snippets
func (e *Executor) createPortalSnippet(_ context.Context, _ planner.PlannedChange) (string, error) {
	// TODO: Implement when SDK supports portal snippets
	return "", fmt.Errorf("portal snippet operations not yet implemented - SDK support pending")
}

// updatePortalSnippet handles UPDATE operations for portal snippets
func (e *Executor) updatePortalSnippet(_ context.Context, _ planner.PlannedChange) (string, error) {
	// TODO: Implement when SDK supports portal snippets
	return "", fmt.Errorf("portal snippet operations not yet implemented - SDK support pending")
}

// deletePortalSnippet handles DELETE operations for portal snippets
func (e *Executor) deletePortalSnippet(_ context.Context, _ planner.PlannedChange) error {
	// TODO: Implement when SDK supports portal snippets
	return fmt.Errorf("portal snippet operations not yet implemented - SDK support pending")
}