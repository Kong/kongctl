package executor

import (
	"context"
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// Executor handles the execution of declarative configuration plans
type Executor struct {
	client   *state.Client
	reporter ProgressReporter
	dryRun   bool
	// Track created resources during execution
	createdResources map[string]string // changeID -> resourceID
	// Track resource refs to IDs for reference resolution
	refToID map[string]map[string]string // resourceType -> ref -> resourceID
	// Unified state cache
	stateCache *state.Cache
}

// New creates a new Executor instance
func New(client *state.Client, reporter ProgressReporter, dryRun bool) *Executor {
	return &Executor{
		client:   client,
		reporter: reporter,
		dryRun:   dryRun,
		createdResources: make(map[string]string),
		refToID: make(map[string]map[string]string),
		stateCache: state.NewCache(),
	}
}

// Execute runs the plan and returns the execution result
func (e *Executor) Execute(ctx context.Context, plan *planner.Plan) (*ExecutionResult, error) {
	result := &ExecutionResult{
		DryRun: e.dryRun,
	}
	
	// Notify reporter of execution start
	if e.reporter != nil {
		e.reporter.StartExecution(plan)
	}
	
	// Execute changes in order
	for _, changeID := range plan.ExecutionOrder {
		// Find the change by ID
		var change *planner.PlannedChange
		for i := range plan.Changes {
			if plan.Changes[i].ID == changeID {
				change = &plan.Changes[i]
				break
			}
		}
		
		if change == nil {
			// This shouldn't happen, but handle gracefully
			err := fmt.Errorf("change with ID %s not found in plan", changeID)
			result.Errors = append(result.Errors, ExecutionError{
				ChangeID: changeID,
				Error:    err.Error(),
			})
			result.FailureCount++
			continue
		}
		
		// Execute the change
		if err := e.executeChange(ctx, result, *change); err != nil {
			// Error already recorded in executeChange
			continue
		}
	}
	
	// Notify reporter of execution completion
	if e.reporter != nil {
		e.reporter.FinishExecution(result)
	}
	
	return result, nil
}

// executeChange executes a single change from the plan
func (e *Executor) executeChange(ctx context.Context, result *ExecutionResult, change planner.PlannedChange) error {
	// Notify reporter of change start
	if e.reporter != nil {
		e.reporter.StartChange(change)
	}
	
	// Extract resource name from fields
	resourceName := getResourceName(change.Fields)
	
	// Pre-execution validation (always performed, even in dry-run)
	if err := e.validateChangePreExecution(ctx, change); err != nil {
		// Record error
		execError := ExecutionError{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			Error:        err.Error(),
		}
		result.Errors = append(result.Errors, execError)
		result.FailureCount++
		
		// In dry-run, also record validation result
		if e.dryRun {
			result.ValidationResults = append(result.ValidationResults, ValidationResult{
				ChangeID:     change.ID,
				ResourceType: change.ResourceType,
				ResourceName: resourceName,
				ResourceRef:  change.ResourceRef,
				Action:       string(change.Action),
				Status:       "would_fail",
				Validation:   "failed",
				Message:      err.Error(),
			})
		}
		
		// Notify reporter
		if e.reporter != nil {
			e.reporter.CompleteChange(change, err)
		}
		
		return err
	}
	
	// If dry-run, skip actual execution
	if e.dryRun {
		result.SkippedCount++
		result.ValidationResults = append(result.ValidationResults, ValidationResult{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			Status:       "would_succeed",
			Validation:   "passed",
		})
		
		if e.reporter != nil {
			e.reporter.SkipChange(change, "dry-run mode")
		}
		
		return nil
	}
	
	// Execute the actual change
	var err error
	var resourceID string
	
	switch change.Action {
	case planner.ActionCreate:
		resourceID, err = e.createResource(ctx, change)
	case planner.ActionUpdate:
		resourceID, err = e.updateResource(ctx, change)
	case planner.ActionDelete:
		err = e.deleteResource(ctx, change)
		resourceID = change.ResourceID
	default:
		err = fmt.Errorf("unknown action: %s", change.Action)
	}
	
	// Record result
	if err != nil {
		execError := ExecutionError{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			Error:        err.Error(),
		}
		result.Errors = append(result.Errors, execError)
		result.FailureCount++
	} else {
		result.SuccessCount++
		result.ChangesApplied = append(result.ChangesApplied, AppliedChange{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			ResourceID:   resourceID,
		})
		
		// Track created resources for dependencies
		if change.Action == planner.ActionCreate && resourceID != "" {
			e.createdResources[change.ID] = resourceID
			
			// Also track by resource type and ref for reference resolution
			if e.refToID[change.ResourceType] == nil {
				e.refToID[change.ResourceType] = make(map[string]string)
			}
			e.refToID[change.ResourceType][change.ResourceRef] = resourceID
		}
	}
	
	// Notify reporter
	if e.reporter != nil {
		e.reporter.CompleteChange(change, err)
	}
	
	return err
}

// validateChangePreExecution performs validation before executing a change
func (e *Executor) validateChangePreExecution(ctx context.Context, change planner.PlannedChange) error {
	switch change.Action {
	case planner.ActionUpdate, planner.ActionDelete:
		// For update/delete, verify resource still exists and check protection
		// Special case: portal_customization is a singleton resource without its own ID
		if change.ResourceID == "" && change.ResourceType != "portal_customization" {
			return fmt.Errorf("resource ID required for %s operation", change.Action)
		}
		
		// Perform resource-specific validation
		switch change.ResourceType {
		case "portal":
			if e.client != nil {
				portal, err := e.client.GetPortalByName(ctx, getResourceName(change.Fields))
				if err != nil {
					return fmt.Errorf("failed to fetch portal: %w", err)
				}
				if portal == nil {
					return fmt.Errorf("portal no longer exists")
				}
			
			// Check protection status
			isProtected := portal.NormalizedLabels[labels.ProtectedKey] == "true"
			
			// For updates, check if this is a protection change (which is allowed)
			isProtectionChange := false
			if change.Action == planner.ActionUpdate {
				// Check if this is a protection change
				switch p := change.Protection.(type) {
				case planner.ProtectionChange:
					isProtectionChange = true
				case map[string]interface{}:
					// From JSON deserialization
					if _, hasOld := p["old"].(bool); hasOld {
						if _, hasNew := p["new"].(bool); hasNew {
							isProtectionChange = true
						}
					}
				}
			}
			
				// Block protected resources unless it's a protection change
				if isProtected && !isProtectionChange && 
					(change.Action == planner.ActionUpdate || change.Action == planner.ActionDelete) {
					return fmt.Errorf("resource is protected and cannot be %s", 
						actionToVerb(change.Action))
				}
			}
		case "api":
			if e.client != nil {
				api, err := e.client.GetAPIByName(ctx, getResourceName(change.Fields))
				if err != nil {
					return fmt.Errorf("failed to fetch API: %w", err)
				}
				if api == nil {
					return fmt.Errorf("API no longer exists")
				}
			
				// Check protection status
				isProtected := api.NormalizedLabels[labels.ProtectedKey] == "true"
				
				// For updates, check if this is a protection change (which is allowed)
				isProtectionChange := false
				if change.Action == planner.ActionUpdate {
					// Check if this is a protection change
					switch p := change.Protection.(type) {
					case planner.ProtectionChange:
						isProtectionChange = true
					case map[string]interface{}:
						// From JSON deserialization
						if _, hasOld := p["old"].(bool); hasOld {
							if _, hasNew := p["new"].(bool); hasNew {
								isProtectionChange = true
							}
						}
					}
				}
				
				// Block protected resources unless it's a protection change
				if isProtected && !isProtectionChange && 
					(change.Action == planner.ActionUpdate || change.Action == planner.ActionDelete) {
					return fmt.Errorf("resource is protected and cannot be %s", 
						actionToVerb(change.Action))
				}
			}
		}
		
	case planner.ActionCreate:
		// For create, verify resource doesn't already exist
		switch change.ResourceType {
		case "portal":
			if e.client != nil {
				resourceName := getResourceName(change.Fields)
				portal, err := e.client.GetPortalByName(ctx, resourceName)
				if err != nil {
					// API error is acceptable here - might mean not found
					// Only fail if it's a real API error (not 404)
					if !strings.Contains(err.Error(), "not found") {
						return fmt.Errorf("failed to check existing portal: %w", err)
					}
				}
				if portal != nil {
					// Portal already exists - this is an error for CREATE
					return fmt.Errorf("portal '%s' already exists", resourceName)
				}
			}
		case "api":
			if e.client != nil {
				resourceName := getResourceName(change.Fields)
				api, err := e.client.GetAPIByName(ctx, resourceName)
				if err != nil {
					// API error is acceptable here - might mean not found
					// Only fail if it's a real API error (not 404)
					if !strings.Contains(err.Error(), "not found") {
						return fmt.Errorf("failed to check existing API: %w", err)
					}
				}
				if api != nil {
					// API already exists - this is an error for CREATE
					return fmt.Errorf("API '%s' already exists", resourceName)
				}
			}
		}
	}
	
	return nil
}


// resolveAuthStrategyRef resolves an auth strategy reference to its ID
func (e *Executor) resolveAuthStrategyRef(ctx context.Context, ref string) (string, error) {
	// First check if it was created in this execution
	if authStrategies, ok := e.refToID["application_auth_strategy"]; ok {
		if id, found := authStrategies[ref]; found {
			return id, nil
		}
	}
	
	// Otherwise, look it up from the API
	strategy, err := e.client.GetAuthStrategyByName(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("failed to get auth strategy by name: %w", err)
	}
	if strategy == nil {
		return "", fmt.Errorf("auth strategy not found: %s", ref)
	}
	
	return strategy.ID, nil
}

// resolvePortalRef resolves a portal reference to its ID
func (e *Executor) resolvePortalRef(ctx context.Context, refInfo planner.ReferenceInfo) (string, error) {
	// First check if it was created in this execution
	if portals, ok := e.refToID["portal"]; ok {
		if id, found := portals[refInfo.Ref]; found {
			return id, nil
		}
	}
	
	// Determine the lookup value - use name from lookup fields if available
	lookupValue := refInfo.Ref
	if refInfo.LookupFields != nil {
		if name, hasName := refInfo.LookupFields["name"]; hasName && name != "" {
			lookupValue = name
		}
	}
	
	// Otherwise, look it up from the API
	portal, err := e.client.GetPortalByName(ctx, lookupValue)
	if err != nil {
		return "", fmt.Errorf("failed to get portal by name: %w", err)
	}
	if portal == nil {
		return "", fmt.Errorf("portal not found: ref=%s, looked up by name=%s", refInfo.Ref, lookupValue)
	}
	
	return portal.ID, nil
}

// populatePortalPages fetches and caches all pages for a portal
func (e *Executor) populatePortalPages(ctx context.Context, portalID string) error {
	portal, exists := e.stateCache.Portals[portalID]
	if !exists {
		portal = &state.CachedPortal{
			Pages: make(map[string]*state.CachedPortalPage),
		}
		e.stateCache.Portals[portalID] = portal
	}
	
	// Fetch all pages
	pages, err := e.client.ListManagedPortalPages(ctx, portalID)
	if err != nil {
		return fmt.Errorf("failed to list portal pages: %w", err)
	}
	
	// First pass: create all pages
	pageMap := make(map[string]*state.CachedPortalPage)
	for _, page := range pages {
		cachedPage := &state.CachedPortalPage{
			PortalPage: page,
			Children:   make(map[string]*state.CachedPortalPage),
		}
		pageMap[page.ID] = cachedPage
	}
	
	// Second pass: establish parent-child relationships
	for _, page := range pages {
		cachedPage := pageMap[page.ID]
		
		if page.ParentPageID == "" {
			// Root page
			portal.Pages[page.ID] = cachedPage
		} else if parent, ok := pageMap[page.ParentPageID]; ok {
			// Child page
			parent.Children[page.ID] = cachedPage
		}
	}
	
	return nil
}

// resolvePortalPageRef resolves a portal page reference to its ID
func (e *Executor) resolvePortalPageRef(
	ctx context.Context, portalID string, pageRef string, lookupFields map[string]string,
) (string, error) {
	// First check if it was created in this execution
	if pages, ok := e.refToID["portal_page"]; ok {
		if id, found := pages[pageRef]; found {
			return id, nil
		}
	}
	
	// Ensure portal pages are cached
	if _, exists := e.stateCache.Portals[portalID]; !exists || 
		e.stateCache.Portals[portalID].Pages == nil {
		if err := e.populatePortalPages(ctx, portalID); err != nil {
			return "", err
		}
	}
	
	portal := e.stateCache.Portals[portalID]
	
	// If we have a parent path, use it for more accurate matching
	if lookupFields != nil && lookupFields["parent_path"] != "" {
		targetPath := lookupFields["parent_path"]
		
		if page := portal.FindPageBySlugPath(targetPath); page != nil {
			return page.ID, nil
		}
	}
	
	// Fallback: search all pages for matching slug
	var searchPages func(pages map[string]*state.CachedPortalPage) string
	searchPages = func(pages map[string]*state.CachedPortalPage) string {
		for _, page := range pages {
			normalizedSlug := strings.TrimPrefix(page.Slug, "/")
			if normalizedSlug == pageRef {
				return page.ID
			}
			// Search children
			if childID := searchPages(page.Children); childID != "" {
				return childID
			}
		}
		return ""
	}
	
	if pageID := searchPages(portal.Pages); pageID != "" {
		return pageID, nil
	}
	
	return "", fmt.Errorf("portal page not found: ref=%s in portal=%s", pageRef, portalID)
}

// Resource operations

func (e *Executor) createResource(ctx context.Context, change planner.PlannedChange) (string, error) {
	switch change.ResourceType {
	case "portal":
		return e.createPortal(ctx, change)
	case "api":
		return e.createAPI(ctx, change)
	case "api_version":
		return e.createAPIVersion(ctx, change)
	case "api_publication":
		return e.createAPIPublication(ctx, change)
	case "api_implementation":
		return e.createAPIImplementation(ctx, change)
	case "api_document":
		return e.createAPIDocument(ctx, change)
	case "application_auth_strategy":
		return e.createApplicationAuthStrategy(ctx, change)
	case "portal_customization":
		// Portal customization is a singleton resource - always exists, so we update instead
		return e.updatePortalCustomization(ctx, change)
	case "portal_custom_domain":
		return e.createPortalCustomDomain(ctx, change)
	case "portal_page":
		return e.createPortalPage(ctx, change)
	case "portal_snippet":
		return e.createPortalSnippet(ctx, change)
	default:
		return "", fmt.Errorf("create operation not yet implemented for %s", change.ResourceType)
	}
}

func (e *Executor) updateResource(ctx context.Context, change planner.PlannedChange) (string, error) {
	switch change.ResourceType {
	case "portal":
		return e.updatePortal(ctx, change)
	case "api":
		return e.updateAPI(ctx, change)
	case "api_document":
		return e.updateAPIDocument(ctx, change)
	case "application_auth_strategy":
		return e.updateApplicationAuthStrategy(ctx, change)
	case "portal_customization":
		return e.updatePortalCustomization(ctx, change)
	case "portal_custom_domain":
		return e.updatePortalCustomDomain(ctx, change)
	case "portal_page":
		return e.updatePortalPage(ctx, change)
	case "portal_snippet":
		return e.updatePortalSnippet(ctx, change)
	// Note: api_version, api_publication, and api_implementation don't support update
	default:
		return "", fmt.Errorf("update operation not yet implemented for %s", change.ResourceType)
	}
}

func (e *Executor) deleteResource(ctx context.Context, change planner.PlannedChange) error {
	switch change.ResourceType {
	case "portal":
		return e.deletePortal(ctx, change)
	case "api":
		return e.deleteAPI(ctx, change)
	case "api_publication":
		return e.deleteAPIPublication(ctx, change)
	case "api_implementation":
		return e.deleteAPIImplementation(ctx, change)
	case "api_document":
		return e.deleteAPIDocument(ctx, change)
	case "application_auth_strategy":
		return e.deleteApplicationAuthStrategy(ctx, change)
	case "portal_custom_domain":
		return e.deletePortalCustomDomain(ctx, change)
	case "portal_page":
		return e.deletePortalPage(ctx, change)
	case "portal_snippet":
		return e.deletePortalSnippet(ctx, change)
	// Note: api_version doesn't support delete
	// Note: portal_customization is a singleton resource and cannot be deleted
	default:
		return fmt.Errorf("delete operation not yet implemented for %s", change.ResourceType)
	}
}

// Helper functions

func getResourceName(fields map[string]interface{}) string {
	if name, ok := fields["name"].(string); ok {
		return name
	}
	return "<unknown>"
}

func actionToVerb(action planner.ActionType) string {
	switch action {
	case planner.ActionCreate:
		return "created"
	case planner.ActionUpdate:
		return "updated"
	case planner.ActionDelete:
		return "deleted"
	default:
		return string(action)
	}
}

// getParentAPIID resolves the parent API ID for child resources
func (e *Executor) getParentAPIID(ctx context.Context, change planner.PlannedChange) (string, error) {
	if change.Parent == nil {
		return "", fmt.Errorf("parent API reference required")
	}
	
	// Use the parent ID if it was already resolved
	if change.Parent.ID != "" {
		return change.Parent.ID, nil
	}
	
	// Check if parent was created in this execution
	for _, dep := range change.DependsOn {
		if resourceID, ok := e.createdResources[dep]; ok {
			return resourceID, nil
		}
	}
	
	// Otherwise look up by name
	parentAPI, err := e.client.GetAPIByName(ctx, change.Parent.Ref)
	if err != nil {
		return "", fmt.Errorf("failed to get parent API: %w", err)
	}
	if parentAPI == nil {
		return "", fmt.Errorf("parent API not found: %s", change.Parent.Ref)
	}
	
	return parentAPI.ID, nil
}