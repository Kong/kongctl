package planner

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

type catalogServicePlannerImpl struct {
	*BasePlanner
}

// NewCatalogServicePlanner creates a new catalog service planner.
func NewCatalogServicePlanner(base *BasePlanner) CatalogServicePlanner {
	return &catalogServicePlannerImpl{
		BasePlanner: base,
	}
}

func (p *catalogServicePlannerImpl) PlannerComponent() string {
	return string(resources.ResourceTypeCatalogService)
}

// PlanChanges generates changes for catalog service resources.
func (p *catalogServicePlannerImpl) PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error {
	namespace := plannerCtx.Namespace
	desired := p.GetDesiredCatalogServices(namespace)

	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	return p.planner.planCatalogServiceChanges(ctx, plannerCtx, desired, plan)
}

// planCatalogServiceChanges handles diffing and planning for catalog services.
func (p *Planner) planCatalogServiceChanges(
	ctx context.Context,
	plannerCtx *Config,
	desired []resources.CatalogServiceResource,
	plan *Plan,
) error {
	p.logger.Debug("planCatalogServiceChanges called",
		slog.Int("desiredCount", len(desired)),
		slog.String("namespace", plannerCtx.Namespace))

	currentServices, err := p.client.ListManagedCatalogServices(ctx, []string{plannerCtx.Namespace})
	if err != nil {
		if state.IsAPIClientError(err) {
			return nil
		}
		return fmt.Errorf("failed to list catalog services: %w", err)
	}

	currentByName := make(map[string]state.CatalogService)
	for _, svc := range currentServices {
		currentByName[svc.Name] = svc
	}

	var protectionErrors []error
	desiredNames := make(map[string]bool)

	for _, desiredSvc := range desired {
		desiredNames[desiredSvc.Name] = true

		current, exists := currentByName[desiredSvc.Name]
		if !exists {
			p.planCatalogServiceCreate(desiredSvc, plan)
			continue
		}

		isProtected := labels.IsProtectedResource(current.NormalizedLabels)
		shouldProtect := false
		if desiredSvc.Kongctl != nil && desiredSvc.Kongctl.Protected != nil && *desiredSvc.Kongctl.Protected {
			shouldProtect = true
		}

		if isProtected != shouldProtect {
			needsUpdate, updateFields := p.shouldUpdateCatalogService(current, desiredSvc)
			protectionChange := &ProtectionChange{
				Old: isProtected,
				New: shouldProtect,
			}

			if err := p.validateProtectionWithChange(
				"catalog_service", desiredSvc.Name, isProtected, ActionUpdate, protectionChange, needsUpdate,
			); err != nil {
				protectionErrors = append(protectionErrors, err)
			} else {
				p.planCatalogServiceProtectionChangeWithFields(current, desiredSvc, updateFields, plan)
			}
		} else {
			needsUpdate, updateFields := p.shouldUpdateCatalogService(current, desiredSvc)
			if needsUpdate {
				if err := p.validateProtection("catalog_service", desiredSvc.Name, isProtected, ActionUpdate); err != nil {
					protectionErrors = append(protectionErrors, err)
				} else {
					p.planCatalogServiceUpdateWithFields(current, desiredSvc, updateFields, plan)
				}
			}
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		for name, current := range currentByName {
			if desiredNames[name] {
				continue
			}

			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			if err := p.validateProtection("catalog_service", name, isProtected, ActionDelete); err != nil {
				protectionErrors = append(protectionErrors, err)
			} else {
				p.planCatalogServiceDelete(current, plan)
			}
		}
	}

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

func (p *Planner) shouldUpdateCatalogService(
	current state.CatalogService,
	desired resources.CatalogServiceResource,
) (bool, map[string]any) {
	updates := make(map[string]any)

	if desired.Name != "" && current.Name != desired.Name {
		updates["name"] = desired.Name
	}

	if desired.DisplayName != "" && current.DisplayName != desired.DisplayName {
		updates["display_name"] = desired.DisplayName
	}

	if desired.Description != nil {
		currentDesc := getString(current.Description)
		if currentDesc != *desired.Description {
			updates["description"] = *desired.Description
		}
	}

	if desired.CustomFields != nil {
		if !reflect.DeepEqual(current.CustomFields, desired.CustomFields) {
			updates["custom_fields"] = desired.CustomFields
		}
	}

	if desired.Labels != nil {
		if labels.CompareUserLabels(current.NormalizedLabels, desired.GetLabels()) {
			updates["labels"] = desired.GetLabels()
		}
	}

	return len(updates) > 0, updates
}

func (p *Planner) planCatalogServiceCreate(resource resources.CatalogServiceResource, plan *Plan) string {
	namespace := DefaultNamespace
	if resource.Kongctl != nil && resource.Kongctl.Namespace != nil {
		namespace = *resource.Kongctl.Namespace
	}

	var protection any
	if resource.Kongctl != nil && resource.Kongctl.Protected != nil {
		protection = *resource.Kongctl.Protected
	}

	config := CreateConfig{
		ResourceType:   "catalog_service",
		ResourceName:   resource.Name,
		ResourceRef:    resource.GetRef(),
		RequiredFields: []string{"name", "display_name"},
		FieldExtractor: func(_ any) map[string]any {
			return extractCatalogServiceFields(resource)
		},
		Namespace: namespace,
		DependsOn: []string{},
	}

	change, err := p.genericPlanner.PlanCreate(context.Background(), config)
	if err != nil {
		p.logger.Error("Failed to plan catalog service create", slog.String("error", err.Error()))
		return ""
	}

	change.Protection = protection
	plan.AddChange(change)

	return change.ID
}

func extractCatalogServiceFields(resource resources.CatalogServiceResource) map[string]any {
	fields := make(map[string]any)

	fields["name"] = resource.Name
	fields["display_name"] = resource.DisplayName
	if resource.Description != nil {
		fields["description"] = *resource.Description
	}
	if len(resource.Labels) > 0 {
		fields["labels"] = resource.GetLabels()
	}
	if resource.CustomFields != nil {
		fields["custom_fields"] = resource.CustomFields
	}

	return fields
}

func (p *Planner) planCatalogServiceUpdateWithFields(
	current state.CatalogService,
	desired resources.CatalogServiceResource,
	updateFields map[string]any,
	plan *Plan,
) {
	namespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		namespace = *desired.Kongctl.Namespace
	}

	var protection any
	if desired.Kongctl != nil && desired.Kongctl.Protected != nil {
		protection = ProtectionChange{
			Old: labels.IsProtectedResource(current.NormalizedLabels),
			New: *desired.Kongctl.Protected,
		}
	}

	if _, ok := updateFields["name"]; !ok {
		updateFields["name"] = current.Name
	}
	if _, ok := updateFields["display_name"]; !ok {
		updateFields["display_name"] = current.DisplayName
	}
	if _, hasLabels := updateFields["labels"]; hasLabels {
		updateFields[FieldCurrentLabels] = current.NormalizedLabels
	}

	config := UpdateConfig{
		ResourceType:  "catalog_service",
		ResourceName:  desired.Name,
		ResourceRef:   desired.GetRef(),
		ResourceID:    current.ID,
		DesiredFields: updateFields,
		CurrentLabels: current.NormalizedLabels,
		DesiredLabels: desired.GetLabels(),
		RequiredFields: []string{
			"name",
			"display_name",
		},
		Namespace: namespace,
	}

	change, err := p.genericPlanner.PlanUpdate(context.Background(), config)
	if err != nil {
		p.logger.Error("Failed to plan catalog service update", slog.String("error", err.Error()))
		fields := make(map[string]any, len(updateFields))
		for k, v := range updateFields {
			fields[k] = v
		}
		changeID := p.nextChangeID(ActionUpdate, "catalog_service", desired.GetRef())
		fallback := PlannedChange{
			ID:           changeID,
			ResourceType: "catalog_service",
			ResourceRef:  desired.GetRef(),
			ResourceID:   current.ID,
			Action:       ActionUpdate,
			Fields:       fields,
			DependsOn:    []string{},
			Namespace:    namespace,
		}
		if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
			fallback.Namespace = *desired.Kongctl.Namespace
		}
		if labels.IsProtectedResource(current.NormalizedLabels) {
			fallback.Protection = true
		}
		plan.AddChange(fallback)
		return
	}

	change.Protection = protection
	plan.AddChange(change)
}

func (p *Planner) planCatalogServiceProtectionChangeWithFields(
	current state.CatalogService,
	desired resources.CatalogServiceResource,
	updateFields map[string]any,
	plan *Plan,
) {
	namespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		namespace = *desired.Kongctl.Namespace
	}

	config := ProtectionChangeConfig{
		ResourceType: "catalog_service",
		ResourceName: desired.Name,
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		OldProtected: labels.IsProtectedResource(current.NormalizedLabels),
		NewProtected: desired.Kongctl != nil && desired.Kongctl.Protected != nil && *desired.Kongctl.Protected,
		Namespace:    namespace,
	}

	change := p.genericPlanner.PlanProtectionChange(context.Background(), config)

	fields := make(map[string]any)
	for k, v := range updateFields {
		fields[k] = v
	}
	fields["name"] = current.Name
	fields["display_name"] = current.DisplayName
	fields["id"] = current.ID

	if ns, ok := current.NormalizedLabels[labels.NamespaceKey]; ok {
		fields["namespace"] = ns
	}

	if current.NormalizedLabels != nil {
		preserved := make(map[string]string)
		for key, val := range current.NormalizedLabels {
			if strings.HasPrefix(key, "KONGCTL-") && key != labels.ProtectedKey {
				preserved[key] = val
			}
		}
		if len(preserved) > 0 {
			fields["preserved_labels"] = preserved
		}
	}

	change.Fields = fields
	plan.AddChange(change)
}

func (p *Planner) planCatalogServiceDelete(current state.CatalogService, plan *Plan) {
	namespace := DefaultNamespace
	if ns, ok := current.NormalizedLabels[labels.NamespaceKey]; ok && ns != "" {
		namespace = ns
	}

	config := DeleteConfig{
		ResourceType: "catalog_service",
		ResourceName: current.Name,
		ResourceRef:  current.Name,
		ResourceID:   current.ID,
		Namespace:    namespace,
	}

	change := p.genericPlanner.PlanDelete(context.Background(), config)
	plan.AddChange(change)
}
