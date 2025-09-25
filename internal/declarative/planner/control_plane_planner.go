package planner

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// controlPlanePlannerImpl implements planning logic for control plane resources
type controlPlanePlannerImpl struct {
	*BasePlanner
}

// NewControlPlanePlanner creates a new control plane planner
func NewControlPlanePlanner(base *BasePlanner) ControlPlanePlanner {
	return &controlPlanePlannerImpl{BasePlanner: base}
}

// PlanChanges generates changes for control plane resources
func (p *controlPlanePlannerImpl) PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error {
	namespace := plannerCtx.Namespace
	desired := p.GetDesiredControlPlanes(namespace)

	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	currentControlPlanes, err := p.GetClient().ListManagedControlPlanes(ctx, []string{namespace})
	if err != nil {
		if state.IsAPIClientError(err) {
			return nil
		}
		return fmt.Errorf("failed to list current control planes in namespace %s: %w", namespace, err)
	}

	currentByName := make(map[string]state.ControlPlane)
	for _, cp := range currentControlPlanes {
		currentByName[cp.Name] = cp
	}

	protectionErrors := &ProtectionErrorCollector{}

	for _, desiredCP := range desired {
		current, exists := currentByName[desiredCP.Name]
		desiredProtected := isProtected(desiredCP)

		if !exists {
			p.planControlPlaneCreate(desiredCP, desiredProtected, plan)
			continue
		}

		currentProtected := labels.IsProtectedResource(current.NormalizedLabels)
		needsUpdate, updateFields := p.shouldUpdateControlPlane(current, desiredCP)

		if currentProtected != desiredProtected {
			protectionChange := &ProtectionChange{Old: currentProtected, New: desiredProtected}
			err := p.ValidateProtectionWithChange(
				"control_plane", desiredCP.Name, currentProtected, ActionUpdate, protectionChange, needsUpdate,
			)
			protectionErrors.Add(err)
			if err == nil {
				p.planControlPlaneProtectionChangeWithFields(current, desiredCP, protectionChange, updateFields, plan)
			}
			continue
		}

		if needsUpdate {
			err := p.ValidateProtection("control_plane", desiredCP.Name, currentProtected, ActionUpdate)
			protectionErrors.Add(err)
			if err == nil {
				p.planControlPlaneUpdate(current, desiredCP, updateFields, plan)
			}
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		desiredNames := make(map[string]struct{})
		for _, cp := range desired {
			desiredNames[cp.Name] = struct{}{}
		}

		for name, current := range currentByName {
			if _, ok := desiredNames[name]; ok {
				continue
			}

			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			err := p.ValidateProtection("control_plane", name, isProtected, ActionDelete)
			protectionErrors.Add(err)
			if err == nil {
				p.planControlPlaneDelete(current, plan)
			}
		}
	}

	if protectionErrors.HasErrors() {
		return protectionErrors.Error()
	}

	return nil
}

func (p *controlPlanePlannerImpl) planControlPlaneCreate(
	desired resources.ControlPlaneResource,
	protected bool,
	plan *Plan,
) {
	fields := extractControlPlaneFields(desired)

	namespace := resources.GetNamespace(desired.Kongctl)
	config := CreateConfig{
		ResourceType:   "control_plane",
		ResourceName:   desired.Name,
		ResourceRef:    desired.GetRef(),
		RequiredFields: []string{"name"},
		FieldExtractor: func(_ any) map[string]any { return fields },
		Namespace:      namespace,
	}

	generic := p.GetGenericPlanner()
	if generic != nil {
		change, err := generic.PlanCreate(context.Background(), config)
		if err == nil {
			change.Protection = protected
			plan.AddChange(change)
			return
		}

		p.planner.logger.Error("Failed to plan control plane create", "error", err.Error())
	}

	changeID := p.NextChangeID(ActionCreate, "control_plane", desired.GetRef())
	plan.AddChange(PlannedChange{
		ID:           changeID,
		ResourceType: "control_plane",
		ResourceRef:  desired.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		Namespace:    namespace,
		Protection:   protected,
	})
}

func (p *controlPlanePlannerImpl) planControlPlaneUpdate(
	current state.ControlPlane,
	desired resources.ControlPlaneResource,
	updateFields map[string]any,
	plan *Plan,
) {
	// Always include name for identification
	updateFields["name"] = current.Name

	if _, hasLabels := updateFields["labels"]; hasLabels {
		updateFields[FieldCurrentLabels] = current.NormalizedLabels
	}

	namespace := resources.GetNamespace(desired.Kongctl)
	config := UpdateConfig{
		ResourceType:   "control_plane",
		ResourceName:   desired.Name,
		ResourceRef:    desired.GetRef(),
		ResourceID:     current.ID,
		DesiredFields:  updateFields,
		RequiredFields: []string{"name"},
		Namespace:      namespace,
	}

	generic := p.GetGenericPlanner()
	if generic != nil {
		change, err := generic.PlanUpdate(context.Background(), config)
		if err == nil {
			if labels.IsProtectedResource(current.NormalizedLabels) {
				change.Protection = true
			}
			plan.AddChange(change)
			return
		}
		p.planner.logger.Error("Failed to plan control plane update", "error", err.Error())
	}

	fields := make(map[string]any)
	for key, value := range updateFields {
		fields[key] = value
	}

	changeID := p.NextChangeID(ActionUpdate, "control_plane", desired.GetRef())
	change := PlannedChange{
		ID:           changeID,
		ResourceType: "control_plane",
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		Namespace:    namespace,
	}

	if labels.IsProtectedResource(current.NormalizedLabels) {
		change.Protection = true
	}

	plan.AddChange(change)
}

func (p *controlPlanePlannerImpl) planControlPlaneProtectionChangeWithFields(
	current state.ControlPlane,
	desired resources.ControlPlaneResource,
	protectionChange *ProtectionChange,
	updateFields map[string]any,
	plan *Plan,
) {
	namespace := resources.GetNamespace(desired.Kongctl)
	config := ProtectionChangeConfig{
		ResourceType: "control_plane",
		ResourceName: desired.Name,
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		OldProtected: protectionChange.Old,
		NewProtected: protectionChange.New,
		Namespace:    namespace,
	}

	generic := p.GetGenericPlanner()
	change := PlannedChange{}
	if generic != nil {
		change = generic.PlanProtectionChange(context.Background(), config)
	} else {
		changeID := p.NextChangeID(ActionUpdate, "control_plane", desired.GetRef())
		change = PlannedChange{
			ID:           changeID,
			ResourceType: "control_plane",
			ResourceRef:  desired.GetRef(),
			ResourceID:   current.ID,
			Action:       ActionUpdate,
			Protection:   *protectionChange,
			Namespace:    namespace,
		}
	}

	fields := map[string]any{"name": current.Name}

	if protectionChange.Old && !protectionChange.New && len(updateFields) > 0 {
		for key, value := range updateFields {
			fields[key] = value
		}
		if _, hasLabels := updateFields["labels"]; hasLabels {
			fields[FieldCurrentLabels] = current.NormalizedLabels
		}
	}

	change.Fields = fields
	plan.AddChange(change)
}

func (p *controlPlanePlannerImpl) planControlPlaneDelete(current state.ControlPlane, plan *Plan) {
	namespace := DefaultNamespace
	if ns, ok := current.NormalizedLabels[labels.NamespaceKey]; ok {
		namespace = ns
	}

	generic := p.GetGenericPlanner()
	if generic != nil {
		config := DeleteConfig{
			ResourceType: "control_plane",
			ResourceName: current.Name,
			ResourceRef:  current.Name,
			ResourceID:   current.ID,
			Namespace:    namespace,
		}
		change := generic.PlanDelete(context.Background(), config)
		change.Fields = map[string]any{"name": current.Name}
		plan.AddChange(change)
		return
	}

	changeID := p.NextChangeID(ActionDelete, "control_plane", current.Name)
	plan.AddChange(PlannedChange{
		ID:           changeID,
		ResourceType: "control_plane",
		ResourceRef:  current.Name,
		ResourceID:   current.ID,
		Action:       ActionDelete,
		Fields:       map[string]any{"name": current.Name},
		Namespace:    namespace,
	})
}

func (p *controlPlanePlannerImpl) shouldUpdateControlPlane(
	current state.ControlPlane,
	desired resources.ControlPlaneResource,
) (bool, map[string]any) {
	updates := make(map[string]any)

	if desired.Description != nil {
		currentDesc := ""
		if current.Description != nil {
			currentDesc = *current.Description
		}
		if currentDesc != *desired.Description {
			updates["description"] = *desired.Description
		}
	}

	if desired.AuthType != nil {
		desiredAuth := string(*desired.AuthType)
		if desiredAuth != "" && desiredAuth != string(current.Config.AuthType) {
			updates["auth_type"] = desiredAuth
		}
	}

	if desired.ProxyUrls != nil {
		if !proxyURLsEqual(current.Config.ProxyUrls, desired.ProxyUrls) {
			updates["proxy_urls"] = desired.ProxyUrls
		}
	}

	if desired.Labels != nil {
		if labels.CompareUserLabels(current.NormalizedLabels, desired.Labels) {
			updates["labels"] = desired.Labels
		}
	}

	return len(updates) > 0, updates
}

func extractControlPlaneFields(cp resources.ControlPlaneResource) map[string]any {
	fields := make(map[string]any)
	fields["name"] = cp.Name

	if cp.Description != nil {
		fields["description"] = *cp.Description
	}

	if cp.ClusterType != nil {
		fields["cluster_type"] = string(*cp.ClusterType)
	}

	if cp.AuthType != nil {
		fields["auth_type"] = string(*cp.AuthType)
	}

	if cp.CloudGateway != nil {
		fields["cloud_gateway"] = *cp.CloudGateway
	}

	if cp.ProxyUrls != nil {
		fields["proxy_urls"] = cp.ProxyUrls
	}

	if cp.Labels != nil {
		fields["labels"] = cp.Labels
	}

	return fields
}

func proxyURLsEqual(current, desired []kkComps.ProxyURL) bool {
	if len(current) != len(desired) {
		return false
	}

	for i := range current {
		if current[i].Host != desired[i].Host ||
			current[i].Port != desired[i].Port ||
			current[i].Protocol != desired[i].Protocol {
			return false
		}
	}

	return true
}

func isProtected(cp resources.ControlPlaneResource) bool {
	if cp.Kongctl != nil && cp.Kongctl.Protected != nil {
		return *cp.Kongctl.Protected
	}
	return false
}
