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
