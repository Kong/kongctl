package planner

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)


// Options configures plan generation behavior
type Options struct {
	Mode PlanMode
}

// Planner generates execution plans
type Planner struct {
	client      *state.Client
	logger      *slog.Logger
	resolver    *ReferenceResolver
	depResolver *DependencyResolver
	changeCount int
	
	// Generic planner for common operations
	genericPlanner *GenericPlanner
	
	// Resource-specific planners
	portalPlanner       PortalPlanner
	authStrategyPlanner AuthStrategyPlanner
	apiPlanner          APIPlanner
	
	// ResourceSet containing all desired resources
	resources *resources.ResourceSet
	
	// Legacy field access for backward compatibility (provides global access)
	desiredPortals             []resources.PortalResource
	desiredPortalPages         []resources.PortalPageResource
	desiredPortalSnippets      []resources.PortalSnippetResource
	desiredPortalCustomizations []resources.PortalCustomizationResource
	desiredPortalCustomDomains  []resources.PortalCustomDomainResource
}

// NewPlanner creates a new planner
func NewPlanner(client *state.Client, logger *slog.Logger) *Planner {
	p := &Planner{
		client:      client,
		logger:      logger,
		resolver:    NewReferenceResolver(client),
		depResolver: NewDependencyResolver(),
		changeCount: 0,
	}
	
	// Initialize generic planner
	p.genericPlanner = NewGenericPlanner(p)
	
	// Initialize resource-specific planners
	base := NewBasePlanner(p)
	p.portalPlanner = NewPortalPlanner(base)
	p.authStrategyPlanner = NewAuthStrategyPlanner(base)
	p.apiPlanner = NewAPIPlanner(base)
	
	return p
}

// GeneratePlan creates a plan from declarative configuration
func (p *Planner) GeneratePlan(ctx context.Context, rs *resources.ResourceSet, opts Options) (*Plan, error) {
	// Create base plan
	basePlan := NewPlan("1.0", "kongctl/dev", opts.Mode)
	
	// Pre-resolution phase: Resolve resource identities before planning
	if err := p.resolveResourceIdentities(ctx, rs); err != nil {
		return nil, fmt.Errorf("failed to resolve resource identities: %w", err)
	}
	
	// Extract all unique namespaces from desired resources
	namespaces := p.getResourceNamespaces(rs)
	
	// If no namespaces found and we're in sync mode, we need to check existing resources
	if len(namespaces) == 0 && opts.Mode == PlanModeSync {
		// Check if we have a namespace from _defaults
		if rs.DefaultNamespace != "" {
			// Use the namespace specified in _defaults
			namespaces = []string{rs.DefaultNamespace}
		} else {
			// For sync mode with empty config, only check the default namespace
			// to prevent accidental deletion of resources in other namespaces
			namespaces = []string{DefaultNamespace}
		}
	}
	
	// Log namespace processing
	p.logger.Debug("Processing namespaces", 
		slog.Int("count", len(namespaces)),
		slog.Any("namespaces", namespaces))
	
	// Process each namespace independently
	for _, namespace := range namespaces {
		// Create a namespace-specific planner context
		namespacePlanner := &Planner{
			client:      p.client,
			logger:      p.logger,
			resolver:    p.resolver,
			depResolver: p.depResolver,
			changeCount: p.changeCount,
		}
		
		// Initialize generic planner for namespace-specific planner
		namespacePlanner.genericPlanner = NewGenericPlanner(namespacePlanner)
		
		// Create new sub-planners for this namespace to ensure they reference
		// the namespace-specific resources, not the parent's empty lists
		base := NewBasePlanner(namespacePlanner)
		namespacePlanner.portalPlanner = NewPortalPlanner(base)
		namespacePlanner.authStrategyPlanner = NewAuthStrategyPlanner(base)
		namespacePlanner.apiPlanner = NewAPIPlanner(base)
		
		
		// Store full ResourceSet for access by planners (enables both filtered views and global lookups)
		namespacePlanner.resources = rs
		
		// Populate legacy field access for backward compatibility
		namespacePlanner.desiredPortals = rs.Portals
		namespacePlanner.desiredPortalPages = rs.PortalPages
		namespacePlanner.desiredPortalSnippets = rs.PortalSnippets
		namespacePlanner.desiredPortalCustomizations = rs.PortalCustomizations
		namespacePlanner.desiredPortalCustomDomains = rs.PortalCustomDomains
		
		// Create a plan for this namespace
		namespacePlan := NewPlan("1.0", "kongctl/dev", opts.Mode)
		
		// Generate changes using interface-based planners
		// Pass the specific namespace to planners instead of wildcard
		actualNamespace := namespace
		if namespace == "*" {
			// For sync mode with empty config, we still need to query all namespaces
			actualNamespace = "*"
		}
		
		// Create planner context with namespace
		plannerCtx := NewConfig(actualNamespace)
		
		if err := namespacePlanner.authStrategyPlanner.PlanChanges(ctx, plannerCtx, namespacePlan); err != nil {
			return nil, fmt.Errorf("failed to plan auth strategy changes for namespace %s: %w", namespace, err)
		}

		if err := namespacePlanner.portalPlanner.PlanChanges(ctx, plannerCtx, namespacePlan); err != nil {
			return nil, fmt.Errorf("failed to plan portal changes for namespace %s: %w", namespace, err)
		}

		// Plan API changes (includes child resources)
		if err := namespacePlanner.apiPlanner.PlanChanges(ctx, plannerCtx, namespacePlan); err != nil {
			return nil, fmt.Errorf("failed to plan API changes for namespace %s: %w", namespace, err)
		}
		
		// Merge namespace plan into base plan
		basePlan.Changes = append(basePlan.Changes, namespacePlan.Changes...)
		basePlan.Warnings = append(basePlan.Warnings, namespacePlan.Warnings...)
		
		// Update change count
		p.changeCount = namespacePlanner.changeCount
	}
	
	// Update the base plan summary after merging all namespace changes
	basePlan.UpdateSummary()

	// Note: Orphan portal child resources (those referencing non-existent portals)
	// are now handled within each namespace's processing using the namespace-filtered
	// resource access methods.

	// Resolve references for all changes
	resolveResult, err := p.resolver.ResolveReferences(ctx, basePlan.Changes)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve references: %w", err)
	}

	// Apply resolved references to changes
	for changeID, refs := range resolveResult.ChangeReferences {
		for i := range basePlan.Changes {
			if basePlan.Changes[i].ID == changeID {
				// Preserve existing references and merge with resolver results
				if basePlan.Changes[i].References == nil {
					basePlan.Changes[i].References = make(map[string]ReferenceInfo)
				}
				for field, ref := range refs {
					basePlan.Changes[i].References[field] = ReferenceInfo{
						Ref: ref.Ref,
						ID:  ref.ID,
					}
				}
				break
			}
		}
	}

	// Resolve dependencies and calculate execution order
	executionOrder, err := p.depResolver.ResolveDependencies(basePlan.Changes)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
	}
	basePlan.SetExecutionOrder(executionOrder)
	
	// Reassign change IDs to match execution order
	p.reassignChangeIDs(basePlan, executionOrder)

	// Add warnings for unresolved references
	for _, change := range basePlan.Changes {
		for field, ref := range change.References {
			if ref.ID == "[unknown]" {
				basePlan.AddWarning(change.ID, fmt.Sprintf(
					"Reference %s=%s will be resolved during execution",
					field, ref.Ref))
			}
		}
	}

	return basePlan, nil
}

// nextChangeID generates temporary change IDs during planning phase
func (p *Planner) nextChangeID(action ActionType, resourceType string, ref string) string {
	p.changeCount++
	actionChar := "?"
	switch action {
	case ActionCreate:
		actionChar = "c"
	case ActionUpdate:
		actionChar = "u"
	case ActionDelete:
		actionChar = "d"
	}
	// Use temporary IDs that will be reassigned based on execution order
	return fmt.Sprintf("temp-%d:%s:%s:%s", p.changeCount, actionChar, resourceType, ref)
}

// reassignChangeIDs updates change IDs to match execution order
func (p *Planner) reassignChangeIDs(plan *Plan, executionOrder []string) {
	// Create mapping from old IDs to new IDs based on execution order
	idMapping := make(map[string]string)
	for newPos, oldID := range executionOrder {
		// Extract components from old ID (format: "temp-N:action:type:ref")
		// We need to parse out the action, type, and ref parts
		parts := strings.SplitN(oldID, ":", 4)
		if len(parts) == 4 && strings.HasPrefix(parts[0], "temp-") {
			// Reconstruct with new position
			newID := fmt.Sprintf("%d:%s:%s:%s", newPos+1, parts[1], parts[2], parts[3])
			idMapping[oldID] = newID
		}
	}
	
	// Update change IDs
	for i := range plan.Changes {
		if newID, ok := idMapping[plan.Changes[i].ID]; ok {
			plan.Changes[i].ID = newID
		}
		
		// Update DependsOn references
		for j := range plan.Changes[i].DependsOn {
			if newID, ok := idMapping[plan.Changes[i].DependsOn[j]]; ok {
				plan.Changes[i].DependsOn[j] = newID
			}
		}
	}
	
	// Update execution order with new IDs
	for i := range plan.ExecutionOrder {
		if newID, ok := idMapping[plan.ExecutionOrder[i]]; ok {
			plan.ExecutionOrder[i] = newID
		}
	}
	
	// Update warnings
	for i := range plan.Warnings {
		if newID, ok := idMapping[plan.Warnings[i].ChangeID]; ok {
			plan.Warnings[i].ChangeID = newID
		}
	}
}

// validateProtection checks if a protected resource would be modified or deleted
func (p *Planner) validateProtection(
	resourceType, resourceName string, 
	currentProtected bool, 
	action ActionType,
) error {
	if action == ActionUpdate || action == ActionDelete {
		if currentProtected {
			var actionVerb string
			switch action { //nolint:exhaustive // ActionCreate is not possible here due to outer if condition
			case ActionDelete:
				actionVerb = "deleted"
			case ActionUpdate:
				actionVerb = "updated"
			default:
				actionVerb = "modified"
			}
			return fmt.Errorf("%s %q is protected and cannot be %s", 
				resourceType, resourceName, actionVerb)
		}
	}
	return nil
}

// validateProtectionWithChange checks if a protected resource would be modified or deleted,
// but allows protection-only removal
func (p *Planner) validateProtectionWithChange(
	resourceType, resourceName string, 
	currentProtected bool,
	action ActionType,
	protectionChange *ProtectionChange,
	hasOtherFieldChanges bool,
) error {
	if action == ActionUpdate && currentProtected {
		// Allow if only removing protection (no other field changes)
		if protectionChange != nil && !protectionChange.New && !hasOtherFieldChanges {
			return nil
		}
		// Block all other updates to protected resources
		return fmt.Errorf("%s %q is protected and cannot be updated", 
			resourceType, resourceName)
	}
	if action == ActionDelete && currentProtected {
		return fmt.Errorf("%s %q is protected and cannot be deleted", 
			resourceType, resourceName)
	}
	return nil
}

// getString dereferences string pointer or returns empty
func getString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Legacy methods for backward compatibility - delegate to ResourceSet methods
// These search across all namespaces since the callers expect global access

// GetDesiredAPIs returns all desired API resources (across all namespaces)
func (p *Planner) GetDesiredAPIs() []resources.APIResource {
	if p.resources == nil {
		return nil
	}
	return p.resources.APIs
}

// GetDesiredPortalCustomizations returns all desired portal customization resources (across all namespaces)
func (p *Planner) GetDesiredPortalCustomizations() []resources.PortalCustomizationResource {
	if p.resources == nil {
		return nil
	}
	return p.resources.PortalCustomizations
}

// GetDesiredPortalCustomDomains returns all desired portal custom domain resources (across all namespaces)
func (p *Planner) GetDesiredPortalCustomDomains() []resources.PortalCustomDomainResource {
	if p.resources == nil {
		return nil
	}
	return p.resources.PortalCustomDomains
}

// GetDesiredPortalPages returns all desired portal page resources (across all namespaces)
func (p *Planner) GetDesiredPortalPages() []resources.PortalPageResource {
	if p.resources == nil {
		return nil
	}
	return p.resources.PortalPages
}

// GetDesiredPortalSnippets returns all desired portal snippet resources (across all namespaces)  
func (p *Planner) GetDesiredPortalSnippets() []resources.PortalSnippetResource {
	if p.resources == nil {
		return nil
	}
	return p.resources.PortalSnippets
}

// resolveResourceIdentities pre-resolves Konnect IDs for all resources
func (p *Planner) resolveResourceIdentities(ctx context.Context, rs *resources.ResourceSet) error {
	// Resolve API identities
	if err := p.resolveAPIIdentities(ctx, rs.APIs); err != nil {
		return fmt.Errorf("failed to resolve API identities: %w", err)
	}
	
	// Resolve Portal identities
	if err := p.resolvePortalIdentities(ctx, rs.Portals); err != nil {
		return fmt.Errorf("failed to resolve portal identities: %w", err)
	}
	
	// Resolve Auth Strategy identities
	if err := p.resolveAuthStrategyIdentities(ctx, rs.ApplicationAuthStrategies); err != nil {
		return fmt.Errorf("failed to resolve auth strategy identities: %w", err)
	}
	
	// API child resources are resolved through their parent APIs
	// so we don't need to resolve them separately here
	
	return nil
}

// resolveAPIIdentities resolves Konnect IDs for API resources
func (p *Planner) resolveAPIIdentities(ctx context.Context, apis []resources.APIResource) error {
	for i := range apis {
		api := &apis[i]
		
		// Skip if already resolved
		if api.GetKonnectID() != "" {
			continue
		}
		
		// Try to find the API using filter
		filter := api.GetKonnectMonikerFilter()
		if filter == "" {
			continue
		}
		
		konnectAPI, err := p.client.GetAPIByFilter(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to lookup API %s: %w", api.GetRef(), err)
		}
		
		if konnectAPI != nil {
			// Match found, update the resource
			api.TryMatchKonnectResource(konnectAPI)
		}
	}
	
	return nil
}

// resolvePortalIdentities resolves Konnect IDs for Portal resources
func (p *Planner) resolvePortalIdentities(ctx context.Context, portals []resources.PortalResource) error {
	for i := range portals {
		portal := &portals[i]
		
		// Skip if already resolved
		if portal.GetKonnectID() != "" {
			continue
		}
		
		// Try to find the portal using filter
		filter := portal.GetKonnectMonikerFilter()
		if filter == "" {
			continue
		}
		
		konnectPortal, err := p.client.GetPortalByFilter(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to lookup portal %s: %w", portal.GetRef(), err)
		}
		
		if konnectPortal != nil {
			// Match found, update the resource
			portal.TryMatchKonnectResource(konnectPortal)
		}
	}
	
	return nil
}

// resolveAuthStrategyIdentities resolves Konnect IDs for Auth Strategy resources
func (p *Planner) resolveAuthStrategyIdentities(
	ctx context.Context, strategies []resources.ApplicationAuthStrategyResource,
) error {
	for i := range strategies {
		strategy := &strategies[i]
		
		// Skip if already resolved
		if strategy.GetKonnectID() != "" {
			continue
		}
		
		// Try to find the strategy using filter
		filter := strategy.GetKonnectMonikerFilter()
		if filter == "" {
			continue
		}
		
		konnectStrategy, err := p.client.GetAuthStrategyByFilter(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to lookup auth strategy %s: %w", strategy.GetRef(), err)
		}
		
		if konnectStrategy != nil {
			// Match found, update the resource
			strategy.TryMatchKonnectResource(konnectStrategy)
		}
	}
	
	return nil
}

// getResourceNamespaces extracts all unique namespaces from the desired resources
func (p *Planner) getResourceNamespaces(rs *resources.ResourceSet) []string {
	namespaceSet := make(map[string]bool)
	
	// Extract namespaces from parent resources
	for _, portal := range rs.Portals {
		ns := resources.GetNamespace(portal.Kongctl)
		namespaceSet[ns] = true
	}
	
	for _, api := range rs.APIs {
		ns := resources.GetNamespace(api.Kongctl)
		namespaceSet[ns] = true
	}
	
	for _, strategy := range rs.ApplicationAuthStrategies {
		ns := resources.GetNamespace(strategy.Kongctl)
		namespaceSet[ns] = true
	}
	
	// Convert set to sorted slice for consistent ordering
	namespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}
	
	// Sort for consistent processing order
	sort.Strings(namespaces)
	
	return namespaces
}


