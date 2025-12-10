package planner

import (
	"context"

	"github.com/Kong/kongctl/internal/declarative/resources"
)

type EGWControlPlanePlannerImpl struct {
	planner   *Planner
	resources *resources.ResourceSet
}

func newEGWControlPlanePlanner(planner *Planner, resources *resources.ResourceSet) *EGWControlPlanePlannerImpl {
	return &EGWControlPlanePlannerImpl{
		planner:   planner,
		resources: resources,
	}
}

func (p *EGWControlPlanePlannerImpl) GetDesiredEGWControlPlanes(namespace string) []resources.EventGatewayControlPlaneResource {
	var result []resources.EventGatewayControlPlaneResource

	return result
}

func (p *EGWControlPlanePlannerImpl) PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error {
	namespace := plannerCtx.Namespace
	err := p.planner.planEGWControlPlaneChanges(ctx, plannerCtx, p.GetDesiredEGWControlPlanes(namespace), plan)
	if err != nil {
		return err
	}

	return nil
}

func (p *Planner) planEGWControlPlaneChanges(ctx context.Context, plannerCtx *Config, desired []resources.EventGatewayControlPlaneResource, plan *Plan) error {
	return nil
}
