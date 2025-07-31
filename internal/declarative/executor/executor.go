package executor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/log"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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
	
	// Resource executors
	portalExecutor *BaseExecutor[kkComps.CreatePortal, kkComps.UpdatePortal]
	apiExecutor    *BaseExecutor[kkComps.CreateAPIRequest, kkComps.UpdateAPIRequest]
	authStrategyExecutor *BaseExecutor[kkComps.CreateAppAuthStrategyRequest, kkComps.UpdateAppAuthStrategyRequest]
	
	// Portal child resource executors
	portalCustomizationExecutor *BaseSingletonExecutor[kkComps.PortalCustomization]
	portalDomainExecutor        *BaseExecutor[kkComps.CreatePortalCustomDomainRequest,
		kkComps.UpdatePortalCustomDomainRequest]
	portalPageExecutor    *BaseExecutor[kkComps.CreatePortalPageRequest, kkComps.UpdatePortalPageRequest]
	portalSnippetExecutor *BaseExecutor[kkComps.CreatePortalSnippetRequest, kkComps.UpdatePortalSnippetRequest]
	
	// API child resource executors
	apiVersionExecutor      *BaseCreateDeleteExecutor[kkComps.CreateAPIVersionRequest]
	apiPublicationExecutor  *BaseCreateDeleteExecutor[kkComps.APIPublication]
	apiDocumentExecutor     *BaseExecutor[kkComps.CreateAPIDocumentRequest, kkComps.APIDocument]
	// API implementation is not yet supported by SDK but we include adapter for completeness
	apiImplementationExecutor *BaseExecutor[kkComps.CreateAPIVersionRequest, kkComps.APIVersion]
}

// New creates a new Executor instance
func New(client *state.Client, reporter ProgressReporter, dryRun bool) *Executor {
	e := &Executor{
		client:   client,
		reporter: reporter,
		dryRun:   dryRun,
		createdResources: make(map[string]string),
		refToID: make(map[string]map[string]string),
		stateCache: state.NewCache(),
	}
	
	// Initialize resource executors
	e.portalExecutor = NewBaseExecutor[kkComps.CreatePortal, kkComps.UpdatePortal](
		NewPortalAdapter(client),
		client,
		dryRun,
	)
	e.apiExecutor = NewBaseExecutor[kkComps.CreateAPIRequest, kkComps.UpdateAPIRequest](
		NewAPIAdapter(client),
		client,
		dryRun,
	)
	e.authStrategyExecutor = NewBaseExecutor[kkComps.CreateAppAuthStrategyRequest, kkComps.UpdateAppAuthStrategyRequest](
		NewAuthStrategyAdapter(client),
		client,
		dryRun,
	)
	
	// Initialize portal child resource executors
	e.portalCustomizationExecutor = NewBaseSingletonExecutor[kkComps.PortalCustomization](
		NewPortalCustomizationAdapter(client),
		dryRun,
	)
	e.portalDomainExecutor = NewBaseExecutor[kkComps.CreatePortalCustomDomainRequest,
		kkComps.UpdatePortalCustomDomainRequest](
		NewPortalDomainAdapter(client),
		client,
		dryRun,
	)
	e.portalPageExecutor = NewBaseExecutor[kkComps.CreatePortalPageRequest, kkComps.UpdatePortalPageRequest](
		NewPortalPageAdapter(client),
		client,
		dryRun,
	)
	e.portalSnippetExecutor = NewBaseExecutor[kkComps.CreatePortalSnippetRequest, kkComps.UpdatePortalSnippetRequest](
		NewPortalSnippetAdapter(client),
		client,
		dryRun,
	)
	
	// Initialize API child resource executors
	e.apiVersionExecutor = NewBaseCreateDeleteExecutor[kkComps.CreateAPIVersionRequest](
		NewAPIVersionAdapter(client),
		dryRun,
	)
	e.apiPublicationExecutor = NewBaseCreateDeleteExecutor[kkComps.APIPublication](
		NewAPIPublicationAdapter(client),
		dryRun,
	)
	e.apiDocumentExecutor = NewBaseExecutor[kkComps.CreateAPIDocumentRequest, kkComps.APIDocument](
		NewAPIDocumentAdapter(client),
		client,
		dryRun,
	)
	// API implementation placeholder - not yet supported by SDK
	e.apiImplementationExecutor = NewBaseExecutor[kkComps.CreateAPIVersionRequest, kkComps.APIVersion](
		NewAPIImplementationAdapter(client),
		client,
		dryRun,
	)
	
	return e
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
	for i, changeID := range plan.ExecutionOrder {
		// Find the change by ID
		var change *planner.PlannedChange
		for j := range plan.Changes {
			if plan.Changes[j].ID == changeID {
				change = &plan.Changes[j]
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
		if err := e.executeChange(ctx, result, change, plan, i); err != nil {
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
func (e *Executor) executeChange(ctx context.Context, result *ExecutionResult, change *planner.PlannedChange,
	plan *planner.Plan, changeIndex int) error {
	// Notify reporter of change start
	if e.reporter != nil {
		e.reporter.StartChange(*change)
	}
	
	// Extract resource name from fields
	resourceName := getResourceName(change.Fields)
	
	// Pre-execution validation (always performed, even in dry-run)
	if err := e.validateChangePreExecution(ctx, *change); err != nil {
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
			e.reporter.CompleteChange(*change, err)
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
			e.reporter.SkipChange(*change, "dry-run mode")
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
			
			// Propagate the created resource ID to any pending changes that reference it
			if changeIndex+1 < len(plan.ExecutionOrder) {
				// Update remaining changes directly in plan.Changes
				for i := changeIndex + 1; i < len(plan.ExecutionOrder); i++ {
					changeID := plan.ExecutionOrder[i]
					for j := range plan.Changes {
						if plan.Changes[j].ID == changeID {
							// Check all references in this change
							for refKey, refInfo := range plan.Changes[j].References {
								// Match based on resource type from the reference key
								refResourceType := strings.TrimSuffix(refKey, "_id")
								if refResourceType == change.ResourceType && refInfo.Ref == change.ResourceRef {
									// Update the reference with the created resource ID
									refInfo.ID = resourceID
									plan.Changes[j].References[refKey] = refInfo
									slog.Debug("Propagated created resource ID to dependent change",
										"change_id", plan.Changes[j].ID,
										"ref_key", refKey,
										"resource_type", change.ResourceType,
										"resource_ref", change.ResourceRef,
										"resource_id", resourceID,
									)
								}
							}
							break
						}
					}
				}
			}
		}
	}
	
	// Notify reporter
	if e.reporter != nil {
		e.reporter.CompleteChange(*change, err)
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
			
			// Check protection status using common utility
			isProtected := common.GetProtectionStatus(portal.NormalizedLabels)
			isProtectionChange := common.IsProtectionChange(change.Protection)
			
				// Validate protection using common utility
				resourceName := common.ExtractResourceName(change.Fields)
				if err := common.ValidateResourceProtection(
					"portal", resourceName, isProtected, change, isProtectionChange,
				); err != nil {
					return err
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
				resourceName := common.ExtractResourceName(change.Fields)
				api, err := e.client.GetAPIByName(ctx, resourceName)
				if err != nil {
					// API error is acceptable here - might mean not found
					// Only fail if it's a real API error (not 404)
					if !strings.Contains(err.Error(), "not found") {
						return common.FormatAPIError("api", resourceName, "check existence", err)
					}
				}
				if api != nil {
					// API already exists - this is an error for CREATE
					return common.FormatResourceExistsError("api", resourceName)
				}
			}
		}
	}
	
	return nil
}


// resolveAuthStrategyRef resolves an auth strategy reference to its ID
//
//nolint:unused // kept for backward compatibility, will be removed in Phase 2 cleanup
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

// resolveAPIRef resolves an API reference to its ID
func (e *Executor) resolveAPIRef(ctx context.Context, refInfo planner.ReferenceInfo) (string, error) {
	// First check if it was created in this execution
	if apis, ok := e.refToID["api"]; ok {
		if id, found := apis[refInfo.Ref]; found {
			slog.Debug("Resolved API reference from created resources",
				"api_ref", refInfo.Ref,
				"api_id", id,
			)
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
	
	// Try to find the API in Konnect with retry for eventual consistency
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		
		api, err := e.client.GetAPIByName(ctx, lookupValue)
		if err == nil && api != nil {
			apiID := api.ID
			slog.Debug("Resolved API reference from Konnect",
				"api_ref", refInfo.Ref,
				"lookup_value", lookupValue,
				"api_id", apiID,
				"attempt", attempt+1,
			)
			
			// Cache this resolution
			if apis, ok := e.refToID["api"]; ok {
				apis[refInfo.Ref] = apiID
			}
			return apiID, nil
		}
		lastErr = err
	}
	
	return "", fmt.Errorf("failed to resolve API reference %s (lookup: %s) after 3 attempts: %w", 
		refInfo.Ref, lookupValue, lastErr)
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

func (e *Executor) createResource(ctx context.Context, change *planner.PlannedChange) (string, error) {
	// Add namespace and protection to context for adapters
	ctx = context.WithValue(ctx, contextKeyNamespace, change.Namespace)
	ctx = context.WithValue(ctx, contextKeyProtection, change.Protection)
	// NOTE: PlannedChange is added to context AFTER reference resolution for each resource type
	
	switch change.ResourceType {
	case "portal":
		// No references to resolve for portal
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.portalExecutor.Create(ctx, *change)
	case "api":
		// No references to resolve for api
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.apiExecutor.Create(ctx, *change)
	case "api_version":
		// First resolve API reference if needed
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References["api_id"] = apiRef
		}
		// Add context with resolved references
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.apiVersionExecutor.Create(ctx, *change)
	case "api_publication":
		// First resolve API reference if needed
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References["api_id"] = apiRef
		}
		// Also resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		// Add context with resolved references
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.apiPublicationExecutor.Create(ctx, *change)
	case "api_implementation":
		// First resolve API reference if needed
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References["api_id"] = apiRef
		}
		// Add context with resolved references
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.apiImplementationExecutor.Create(ctx, *change)
	case "api_document":
		// First resolve API reference if needed
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References["api_id"] = apiRef
		}
		// Add context with resolved references
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.apiDocumentExecutor.Create(ctx, *change)
	case "application_auth_strategy":
		// No references to resolve for application_auth_strategy
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.authStrategyExecutor.Create(ctx, *change)
	case "portal_customization":
		// Portal customization is a singleton resource - always exists, so we update instead
		portalID, err := e.resolvePortalRef(ctx, change.References["portal_id"])
		if err != nil {
			return "", err
		}
		// Add context after resolution
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.portalCustomizationExecutor.Update(ctx, *change, portalID)
	case "portal_custom_domain":
		// No references to resolve for portal_custom_domain
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.portalDomainExecutor.Create(ctx, *change)
	case "portal_page":
		// First resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		// Handle parent page reference resolution if needed
		if parentPageRef, ok := change.References["parent_page_id"]; ok && parentPageRef.ID == "" {
			portalID := change.References["portal_id"].ID
			parentPageID, err := e.resolvePortalPageRef(ctx, portalID, parentPageRef.Ref, parentPageRef.LookupFields)
			if err != nil {
				return "", fmt.Errorf("failed to resolve parent page reference: %w", err)
			}
			// Create a new reference with the resolved ID
			parentPageRef.ID = parentPageID
			change.References["parent_page_id"] = parentPageRef
		}
		// Add context with resolved references
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.portalPageExecutor.Create(ctx, *change)
	case "portal_snippet":
		// First resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		// Add context with resolved references
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.portalSnippetExecutor.Create(ctx, *change)
	default:
		return "", fmt.Errorf("create operation not yet implemented for %s", change.ResourceType)
	}
}

func (e *Executor) updateResource(ctx context.Context, change *planner.PlannedChange) (string, error) {
	// Add namespace, protection, and planned change to context for adapters
	ctx = context.WithValue(ctx, contextKeyNamespace, change.Namespace)
	ctx = context.WithValue(ctx, contextKeyProtection, change.Protection)
	ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
	
	switch change.ResourceType {
	case "portal":
		return e.portalExecutor.Update(ctx, *change)
	case "api":
		return e.apiExecutor.Update(ctx, *change)
	case "api_document":
		// First resolve API reference if needed
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References["api_id"] = apiRef
		}
		return e.apiDocumentExecutor.Update(ctx, *change)
	case "application_auth_strategy":
		return e.authStrategyExecutor.Update(ctx, *change)
	case "portal_customization":
		portalID, err := e.resolvePortalRef(ctx, change.References["portal_id"])
		if err != nil {
			return "", err
		}
		return e.portalCustomizationExecutor.Update(ctx, *change, portalID)
	case "portal_custom_domain":
		return e.portalDomainExecutor.Update(ctx, *change)
	case "portal_page":
		// First resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		// Handle parent page reference resolution if needed
		if parentPageRef, ok := change.References["parent_page_id"]; ok && parentPageRef.ID == "" {
			portalID := change.References["portal_id"].ID
			parentPageID, err := e.resolvePortalPageRef(ctx, portalID, parentPageRef.Ref, parentPageRef.LookupFields)
			if err != nil {
				return "", fmt.Errorf("failed to resolve parent page reference: %w", err)
			}
			// Create a new reference with the resolved ID
			parentPageRef.ID = parentPageID
			change.References["parent_page_id"] = parentPageRef
		}
		return e.portalPageExecutor.Update(ctx, *change)
	case "portal_snippet":
		// First resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		return e.portalSnippetExecutor.Update(ctx, *change)
	// Note: api_version, api_publication, and api_implementation don't support update
	default:
		return "", fmt.Errorf("update operation not yet implemented for %s", change.ResourceType)
	}
}

func (e *Executor) deleteResource(ctx context.Context, change *planner.PlannedChange) error {
	// Add namespace and protection to context for adapters
	ctx = context.WithValue(ctx, contextKeyNamespace, change.Namespace)
	ctx = context.WithValue(ctx, contextKeyProtection, change.Protection)
	// NOTE: PlannedChange is added to context AFTER reference resolution for each resource type
	
	switch change.ResourceType {
	case "portal":
		// No references to resolve for portal
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.portalExecutor.Delete(ctx, *change)
	case "api":
		// No references to resolve for api
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.apiExecutor.Delete(ctx, *change)
	case "api_version":
		// No references to resolve for api_version delete
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.apiVersionExecutor.Delete(ctx, *change)
	case "api_publication":
		// No references to resolve for api_publication delete
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.apiPublicationExecutor.Delete(ctx, *change)
	case "api_implementation":
		// No references to resolve for api_implementation delete
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.apiImplementationExecutor.Delete(ctx, *change)
	case "api_document":
		// First resolve API reference if needed
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References["api_id"] = apiRef
		}
		// Add context with resolved references
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.apiDocumentExecutor.Delete(ctx, *change)
	case "application_auth_strategy":
		// No references to resolve for application_auth_strategy
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.authStrategyExecutor.Delete(ctx, *change)
	case "portal_custom_domain":
		// No references to resolve for portal_custom_domain
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.portalDomainExecutor.Delete(ctx, *change)
	case "portal_page":
		// First resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		// Add context with resolved references
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.portalPageExecutor.Delete(ctx, *change)
	case "portal_snippet":
		// First resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		// Add context with resolved references
		ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
		return e.portalSnippetExecutor.Delete(ctx, *change)
	// Note: portal_customization is a singleton resource and cannot be deleted
	default:
		return fmt.Errorf("delete operation not yet implemented for %s", change.ResourceType)
	}
}

// Helper functions

// getResourceName is deprecated, use common.ExtractResourceName instead
// Kept for backward compatibility with existing code
func getResourceName(fields map[string]interface{}) string {
	return common.ExtractResourceName(fields)
}

// actionToVerb is deprecated, use common utilities instead
// Kept for backward compatibility with existing code
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
//
//nolint:unused // kept for backward compatibility, will be removed in Phase 2 cleanup
func (e *Executor) getParentAPIID(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Add debug logging
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	logger.Debug("getParentAPIID called",
		slog.String("change_id", change.ID),
		slog.String("resource_type", change.ResourceType),
		slog.String("resource_ref", change.ResourceRef),
		slog.Any("parent", change.Parent),
	)
	
	if change.Parent == nil {
		return "", fmt.Errorf("parent API reference required")
	}
	
	// Log parent details
	logger.Debug("Parent details",
		slog.String("parent_ref", change.Parent.Ref),
		slog.String("parent_id", change.Parent.ID),
		slog.Bool("parent_id_empty", change.Parent.ID == ""),
		slog.Int("parent_id_length", len(change.Parent.ID)),
	)
	
	// Use the parent ID if it was already resolved
	if change.Parent.ID != "" {
		logger.Debug("Using resolved parent ID", slog.String("parent_id", change.Parent.ID))
		return change.Parent.ID, nil
	}
	
	// Check if parent was created in this execution
	logger.Debug("Checking dependencies", slog.Int("dep_count", len(change.DependsOn)))
	for _, dep := range change.DependsOn {
		if resourceID, ok := e.createdResources[dep]; ok {
			logger.Debug("Found parent in created resources",
				slog.String("dependency", dep),
				slog.String("resource_id", resourceID),
			)
			return resourceID, nil
		}
	}
	
	// Otherwise look up by name
	logger.Debug("Falling back to API lookup by name", slog.String("api_ref", change.Parent.Ref))
	parentAPI, err := e.client.GetAPIByName(ctx, change.Parent.Ref)
	if err != nil {
		return "", fmt.Errorf("failed to get parent API: %w", err)
	}
	if parentAPI == nil {
		return "", fmt.Errorf("parent API not found: %s", change.Parent.Ref)
	}
	
	logger.Debug("Found parent API by name", 
		slog.String("api_name", parentAPI.Name),
		slog.String("api_id", parentAPI.ID),
	)
	
	return parentAPI.ID, nil
}