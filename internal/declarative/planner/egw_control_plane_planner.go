package planner

import (
	"context"
	"fmt"

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
	return p.BasePlanner.GetDesiredEventGatewayControlPlanes(namespace)
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
	// Skip if no API resources to plan and not in sync mode
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		p.logger.Debug("Skipping API planning - no desired APIs")
		return nil
	}

	// Get namespace from planner context
	namespace := plannerCtx.Namespace

	// Fetch current managed APIs from the specific namespace
	namespaceFilter := []string{namespace}
	currentEGWControlPlanes, err := p.client.ListManagedEventGatewayControlPlanes(ctx, namespaceFilter)
	if err != nil {
		// If API client is not configured, skip API planning
		if err.Error() == "API client not configured" {
			return nil
		}
		return fmt.Errorf("failed to list current Event Gateway Control Planes: %w", err)
	}

	// Index current APIs by name
	currentByName := make(map[string]state.EventGatewayControlPlane)
	for _, cp := range currentEGWControlPlanes {
		currentByName[cp.Name] = cp
	}

	// Collect protection validation errors
	var protectionErrors []error

	// Compare each desired API
	for _, desiredEGWCP := range desired {
		current, exists := currentByName[desiredEGWCP.Name]

		if !exists {
			// CREATE action
			_ = p.planEGWControlPlaneCreate(desiredEGWCP, plan)
		} else {
			// Check if update needed
			isProtected := labels.IsProtectedResource(current.NormalizedLabels)

			// Get protection status from desired configuration
			shouldProtect := false
			if desiredEGWCP.Kongctl != nil && desiredEGWCP.Kongctl.Protected != nil && *desiredEGWCP.Kongctl.Protected {
				shouldProtect = true
			}

			// Handle protection changes
			if isProtected != shouldProtect {
				// When changing protection status, include any other field updates too
				needsUpdate, _ := p.shouldUpdateEGWControlPlaneResource(current, desiredEGWCP)

				// Create protection change object
				protectionChange := &ProtectionChange{
					Old: isProtected,
					New: shouldProtect,
				}

				// Validate protection change
				err := p.validateProtectionWithChange("api", desiredEGWCP.Name, isProtected, ActionUpdate,
					protectionChange, needsUpdate)
				if err != nil {
					protectionErrors = append(protectionErrors, err)
				} else {
					//p.planEGWControlPlaneProtectionChangeWithFields(current, desiredEGWCP, isProtected, shouldProtect, updateFields, plan)
				}
			} else {
				// Check if update needed based on configuration
				needsUpdate, updateFields := p.shouldUpdateEGWControlPlaneResource(current, desiredEGWCP)
				if needsUpdate {
					// Regular update - check protection
					if err := p.validateProtection("event-gateway-control-plane", desiredEGWCP.Name, isProtected, ActionUpdate); err != nil {
						protectionErrors = append(protectionErrors, err)
					} else {
						p.planEGWControlPlaneUpdateWithFields(current, desiredEGWCP, updateFields, plan)
					}
				}
			}
		}
	}

	// Check for managed resources to delete (sync mode only)
	if plan.Metadata.Mode == PlanModeSync {
		// Build set of desired Event gateway names
		desiredNames := make(map[string]bool)
		for _, eventGateway := range desired {
			desiredNames[eventGateway.Name] = true
		}

		// Find managed Event Gateway Control Planes not in desired state
		for name, current := range currentByName {
			if !desiredNames[name] {
				// Validate protection before adding DELETE
				isProtected := labels.IsProtectedResource(current.NormalizedLabels)
				if err := p.validateProtection("event-gateway-control-plane", name, isProtected, ActionDelete); err != nil {
					protectionErrors = append(protectionErrors, err)
				} else {
					p.planEGWControlPlaneDelete(current, plan)
				}
			}
		}
	}

	// Fail fast if any protected resources would be modified
	if len(protectionErrors) > 0 {
		errMsg := "Cannot generate plan due to protected resources:\n"
		for _, err := range protectionErrors {
			errMsg += fmt.Sprintf("- %s\n", err.Error())
		}
		errMsg += "\nTo proceed, first update these resources to set protected: false"
		return fmt.Errorf("%s", errMsg)
	}

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
		if getString(current.Description) != getString(desired.Description) {
			updates["description"] = getString(desired.Description)
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

	// Always include name for identification
	updateFields["name"] = current.Name

	updateFields[FieldCurrentLabels] = current.NormalizedLabels
	config := UpdateConfig{
		ResourceType:   string(desired.GetType()),
		ResourceName:   desired.Name,
		ResourceRef:    desired.Ref,
		ResourceID:     current.ID,
		CurrentFields:  nil, // Not needed for direct update
		DesiredFields:  updateFields,
		RequiredFields: []string{"name"},
		Namespace:      namespace,
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
