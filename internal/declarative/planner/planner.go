package planner

import (
	"context"
	"fmt"

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
	resolver    *ReferenceResolver
	depResolver *DependencyResolver
	changeCount int
	
	// Resource-specific planners
	portalPlanner       PortalPlanner
	authStrategyPlanner AuthStrategyPlanner
	apiPlanner          APIPlanner
	
	// Desired resources (set during plan generation)
	desiredPortals             []resources.PortalResource
	desiredAuthStrategies      []resources.ApplicationAuthStrategyResource
	desiredAPIs                []resources.APIResource
	desiredAPIVersions         []resources.APIVersionResource
	desiredAPIPublications     []resources.APIPublicationResource
	desiredAPIImplementations  []resources.APIImplementationResource
	desiredAPIDocuments        []resources.APIDocumentResource
}

// NewPlanner creates a new planner
func NewPlanner(client *state.Client) *Planner {
	p := &Planner{
		client:      client,
		resolver:    NewReferenceResolver(client),
		depResolver: NewDependencyResolver(),
		changeCount: 0,
	}
	
	// Initialize resource-specific planners
	base := NewBasePlanner(p)
	p.portalPlanner = NewPortalPlanner(base)
	p.authStrategyPlanner = NewAuthStrategyPlanner(base)
	p.apiPlanner = NewAPIPlanner(base)
	
	return p
}

// GeneratePlan creates a plan from declarative configuration
func (p *Planner) GeneratePlan(ctx context.Context, rs *resources.ResourceSet, opts Options) (*Plan, error) {
	plan := NewPlan("1.0", "kongctl/dev", opts.Mode)

	// Store desired resources for access by planners
	p.desiredPortals = rs.Portals
	p.desiredAuthStrategies = rs.ApplicationAuthStrategies
	p.desiredAPIs = rs.APIs
	p.desiredAPIVersions = rs.APIVersions
	p.desiredAPIPublications = rs.APIPublications
	p.desiredAPIImplementations = rs.APIImplementations
	p.desiredAPIDocuments = rs.APIDocuments

	// Pre-resolution phase: Resolve resource identities before planning
	if err := p.resolveResourceIdentities(ctx, rs); err != nil {
		return nil, fmt.Errorf("failed to resolve resource identities: %w", err)
	}

	// Generate changes using interface-based planners
	if err := p.authStrategyPlanner.PlanChanges(ctx, plan); err != nil {
		return nil, fmt.Errorf("failed to plan auth strategy changes: %w", err)
	}

	if err := p.portalPlanner.PlanChanges(ctx, plan); err != nil {
		return nil, fmt.Errorf("failed to plan portal changes: %w", err)
	}

	// Plan API changes (includes child resources)
	if err := p.apiPlanner.PlanChanges(ctx, plan); err != nil {
		return nil, fmt.Errorf("failed to plan API changes: %w", err)
	}

	// Future: Add other resource types

	// Resolve references for all changes
	resolveResult, err := p.resolver.ResolveReferences(ctx, plan.Changes)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve references: %w", err)
	}

	// Apply resolved references to changes
	for changeID, refs := range resolveResult.ChangeReferences {
		for i := range plan.Changes {
			if plan.Changes[i].ID == changeID {
				plan.Changes[i].References = make(map[string]ReferenceInfo)
				for field, ref := range refs {
					plan.Changes[i].References[field] = ReferenceInfo(ref)
				}
				break
			}
		}
	}

	// Resolve dependencies and calculate execution order
	executionOrder, err := p.depResolver.ResolveDependencies(plan.Changes)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
	}
	plan.SetExecutionOrder(executionOrder)

	// Add warnings for unresolved references
	for _, change := range plan.Changes {
		for field, ref := range change.References {
			if ref.ID == "<unknown>" {
				plan.AddWarning(change.ID, fmt.Sprintf(
					"Reference %s=%s will be resolved during execution",
					field, ref.Ref))
			}
		}
	}

	return plan, nil
}

// nextChangeID generates semantic change IDs
func (p *Planner) nextChangeID(action ActionType, ref string) string {
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
	return fmt.Sprintf("%d-%s-%s", p.changeCount, actionChar, ref)
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

// getString dereferences string pointer or returns empty
func getString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// GetDesiredAPIs returns the desired API resources
func (p *Planner) GetDesiredAPIs() []resources.APIResource {
	return p.desiredAPIs
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
