package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
)

// rootPlanner binds a root entry point to its scope and error context. Child
// planning remains owned by that entry point.
type rootPlanner struct {
	resourceType resources.ResourceType
	displayName  string
	planner      ResourcePlanner
	inScope      func(*Plan) bool
}

// rootPlanners is the ordered inventory for root planner construction and
// dispatch. Keep this order stable: planning assigns change IDs and can resolve
// references against changes produced by earlier entries.
func (p *Planner) rootPlanners() []rootPlanner {
	base := NewBasePlanner(p)
	return []rootPlanner{
		{
			resourceType: resources.ResourceTypeDCRProvider,
			displayName:  "DCR provider",
			planner:      NewDCRProviderPlanner(base),
		},
		{
			resourceType: resources.ResourceTypeApplicationAuthStrategy,
			displayName:  "auth strategy",
			planner:      NewAuthStrategyPlanner(base),
		},
		{
			resourceType: resources.ResourceTypeControlPlane,
			displayName:  "control plane",
			planner:      NewControlPlanePlanner(base),
		},
		{
			resourceType: resources.ResourceTypePortal,
			displayName:  ResourceTypePortal,
			planner:      NewPortalPlanner(base),
		},
		{
			resourceType: resources.ResourceTypeCatalogService,
			displayName:  "catalog service",
			planner:      NewCatalogServicePlanner(base),
		},
		{
			resourceType: resources.ResourceTypeAIGateway,
			displayName:  "AI Gateway",
			planner:      NewAIGatewayPlanner(base),
		},
		{
			resourceType: resources.ResourceTypeDashboard,
			displayName:  "dashboard",
			planner:      NewDashboardPlanner(base),
		},
		{
			resourceType: resources.ResourceTypeAPI,
			displayName:  "API",
			planner:      NewAPIPlanner(base),
		},
		{
			resourceType: resources.ResourceTypeEventGatewayControlPlane,
			displayName:  "Event Gateway Control Plane",
			planner:      NewEGWControlPlanePlanner(base, p.resources),
		},
		{
			resourceType: resources.ResourceTypeOrganizationTeam,
			displayName:  "Team",
			planner:      NewOrganizationTeamPlanner(base),
			// Organization assignments can be in scope without a team root.
			inScope: p.shouldPlanOrganization,
		},
	}
}

func (p *Planner) planRootChanges(ctx context.Context, opts Options, plannerCtx *Config, plan *Plan) error {
	for _, root := range p.rootPlanners() {
		var inScope bool
		if root.inScope != nil {
			inScope = root.inScope(plan)
		} else {
			inScope = p.shouldPlanRoot(plan, root.resourceType)
		}
		if !inScope {
			continue
		}

		if err := root.planner.PlanChanges(
			withPlannerHTTPLogContext(ctx, opts, plannerComponent(root.planner), ""),
			plannerCtx,
			plan,
		); err != nil {
			return fmt.Errorf(
				"failed to plan %s changes for namespace %s: %w",
				root.displayName,
				plannerCtx.Namespace,
				err,
			)
		}
	}
	return nil
}
