package planner

import (
	"context"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

type EGWControlPlanePlannerImpl struct {
	*BasePlanner
	resources *resources.ResourceSet
}

func NewEGWControlPlanePlanner(planner *BasePlanner, resources *resources.ResourceSet) *EGWControlPlanePlannerImpl {
	return &EGWControlPlanePlannerImpl{
		BasePlanner: planner,
		resources:   resources,
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

func (p *Planner) shouldUpdateEGWControlPlaneResource(current state.EventGatewayControlPlane, desired resources.EventGatewayControlPlaneResource) (bool, map[string]any) {
	updates := make(map[string]any)

	if desired.Name != current.Name {
		currentName := current.Name
		if currentName != desired.Name {
			updates["name"] = desired.Name
		}
	}

	if desired.Description != current.Description {
		currentDesc := getString(current.Description)
		if currentDesc != *desired.Description {
			updates["description"] = *desired.Description
		}
	}

	if desired.Labels != nil {
		if labels.CompareUserLabels(current.NormalizedLabels, desired.GetLabels()) {
			updates["labels"] = desired.GetLabels()
		}
	}

	// Add other field comparisons

	return len(updates) > 0, updates
}

func (p *Planner) planEGWControlPlaneCreate(egwControlPlane resources.EventGatewayControlPlaneResource, plan *Plan) string {
	var protection any
	if egwControlPlane.Kongctl != nil && egwControlPlane.Kongctl.Protected != nil {
		protection = *egwControlPlane.Kongctl.Protected
	}

	// Extract namespace
	namespace := DefaultNamespace
	if egwControlPlane.Kongctl != nil && egwControlPlane.Kongctl.Namespace != nil {
		namespace = *egwControlPlane.Kongctl.Namespace
	}

	config := CreateConfig{
		ResourceType:   string(egwControlPlane.GetType()),
		ResourceName:   egwControlPlane.Name,
		ResourceRef:    egwControlPlane.Ref,
		RequiredFields: []string{"name"},
		FieldExtractor: func(_ any) map[string]any {
			return extractEGWControlPlaneFields(egwControlPlane)
		},
		Namespace: namespace,
		DependsOn: []string{},
	}

	change, err := p.genericPlanner.PlanCreate(context.Background(), config)
	if err != nil {
		return ""
	}
	change.Protection = protection
	plan.AddChange(change)
	return change.ID
}

func extractEGWControlPlaneFields(resource any) map[string]any {
	fields := make(map[string]any)
	egwControlPlane, ok := resource.(resources.EventGatewayControlPlaneResource)
	if !ok {
		return fields
	}

	fields["name"] = egwControlPlane.Name

	if egwControlPlane.Description != nil {
		fields["description"] = *egwControlPlane.Description
	}

	if len(egwControlPlane.GetLabels()) > 0 {
		fields["labels"] = egwControlPlane.GetLabels()
	}
	return fields
}

func (p *Planner) planEGWControlPlaneUpdateWithFields(
	current state.EventGatewayControlPlane,
	desired resources.EventGatewayControlPlaneResource,
	updateFields map[string]any,
	plan *Plan) {
	var protection any
	if desired.Kongctl != nil && desired.Kongctl.Protected != nil {
		protection = *desired.Kongctl.Protected
	}

	// Extract namespace
	namespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		namespace = *desired.Kongctl.Namespace
	}

	updateFields[FieldCurrentLabels] = current.NormalizedLabels
	config := UpdateConfig{
		ResourceType:   string(desired.GetType()),
		ResourceName:   desired.Name,
		ResourceRef:    desired.Ref,
		RequiredFields: []string{"name"},
		FieldComparator: func(current, desired map[string]any) bool {
			// todo
			return false
		},
		Namespace: namespace,
	}

	change, err := p.genericPlanner.PlanUpdate(context.Background(), config)
	if err != nil {
		// Handle error appropriately - this is example code
		// In real implementation, return the error
		return
	}
	change.Protection = protection

	plan.AddChange(change)
}

func (p *Planner) planEGWControlPlaneDelete(egwControlPlane state.EventGatewayControlPlane, plan *Plan) {
	namespace := DefaultNamespace
	if ns, ok := egwControlPlane.NormalizedLabels[labels.NamespaceKey]; ok {
		namespace = ns
	}

	config := DeleteConfig{
		ResourceType: string(resources.ResourceTypeEventGatewayControlPlane),
		ResourceName: egwControlPlane.Name,
		ResourceID:   egwControlPlane.ID,
		Namespace:    namespace,
	}

	change := p.genericPlanner.PlanDelete(context.Background(), config)
	plan.AddChange(change)
}
