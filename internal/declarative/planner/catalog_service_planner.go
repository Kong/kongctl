package planner

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"reflect"

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

	currentServices, err := p.listManagedCatalogServices(ctx, []string{plannerCtx.Namespace})
	if err != nil {
		if state.IsAPIClientError(err) {
			return nil
		}
		return fmt.Errorf("failed to list catalog services: %w", err)
	}

	currentByName := make(map[string]managedRoot[state.CatalogService])
	for _, svc := range currentServices {
		currentByName[svc.Name] = managedRoot[state.CatalogService]{
			resource: svc, name: svc.Name,
			protected: labels.IsProtectedResource(svc.NormalizedLabels),
		}
	}
	reconciler := newManagedRootReconciler(NewBasePlanner(p), ResourceTypeCatalogService,
		managedRootOperations[resources.CatalogServiceResource, state.CatalogService]{
			diff:   p.shouldUpdateCatalogService,
			create: p.planCatalogServiceCreate,
			update: p.planCatalogServiceUpdateWithFields,
			changeProtection: func(current state.CatalogService, desired resources.CatalogServiceResource,
				_, _ bool, fields map[string]any, changed map[string]FieldChange, plan *Plan,
			) {
				p.planCatalogServiceProtectionChangeWithFields(current, desired, fields, changed, plan)
			},
			remove: p.planCatalogServiceDelete,
		}, plan)

	if plan.Metadata.Mode == PlanModeDelete {
		for _, desiredSvc := range desired {
			reconciler.deleteDesired(desiredSvc.Name, findManagedRoot(currentByName, desiredSvc.Name))
		}
		return reconciler.errors.Error()
	}

	desiredNames := make(map[string]bool)
	for _, desiredSvc := range desired {
		desiredNames[desiredSvc.Name] = true
		reconciler.reconcileLifecycle(managedRoot[resources.CatalogServiceResource]{
			resource: desiredSvc, name: desiredSvc.Name,
			protected: desiredSvc.Kongctl != nil && desiredSvc.Kongctl.Protected != nil && *desiredSvc.Kongctl.Protected,
		}, findManagedRoot(currentByName, desiredSvc.Name))
	}

	if plan.Metadata.Mode == PlanModeSync {
		for name, current := range currentByName {
			if !desiredNames[name] {
				reconciler.remove(current)
			}
		}
	}
	return reconciler.errors.Error()
}

func (p *Planner) shouldUpdateCatalogService(
	current state.CatalogService,
	desired resources.CatalogServiceResource,
) (bool, map[string]any, map[string]FieldChange) {
	updates := make(map[string]any)
	changedFields := make(map[string]FieldChange)

	if desired.Name != "" && current.Name != desired.Name {
		updates[FieldName] = desired.Name
		changedFields[FieldName] = FieldChange{
			Old: current.Name,
			New: desired.Name,
		}
	}

	if desired.DisplayName != "" && current.DisplayName != desired.DisplayName {
		updates[FieldDisplayName] = desired.DisplayName
		changedFields[FieldDisplayName] = FieldChange{
			Old: current.DisplayName,
			New: desired.DisplayName,
		}
	}

	if desired.Description != nil {
		currentDesc := getString(current.Description)
		if currentDesc != *desired.Description {
			updates[FieldDescription] = *desired.Description
			changedFields[FieldDescription] = FieldChange{
				Old: currentDesc,
				New: *desired.Description,
			}
		}
	}

	if desired.CustomFields != nil {
		if !reflect.DeepEqual(current.CustomFields, desired.CustomFields) {
			updates[FieldCustomFields] = desired.CustomFields
			changedFields[FieldCustomFields] = FieldChange{
				Old: current.CustomFields,
				New: desired.CustomFields,
			}
		}
	}

	if desired.Labels != nil {
		if labels.CompareUserLabels(current.NormalizedLabels, desired.GetLabels()) {
			updates[FieldLabels] = desired.GetLabels()
			changedFields[FieldLabels] = FieldChange{
				Old: labels.GetUserLabels(current.NormalizedLabels),
				New: labels.GetUserLabels(desired.GetLabels()),
			}
		}
	}

	return len(updates) > 0, updates, changedFields
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
		ResourceType:   ResourceTypeCatalogService,
		ResourceName:   resource.Name,
		ResourceRef:    resource.GetRef(),
		RequiredFields: []string{FieldName, FieldDisplayName},
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

	fields[FieldName] = resource.Name
	fields[FieldDisplayName] = resource.DisplayName
	if resource.Description != nil {
		fields[FieldDescription] = *resource.Description
	}
	if len(resource.Labels) > 0 {
		fields[FieldLabels] = resource.GetLabels()
	}
	if resource.CustomFields != nil {
		fields[FieldCustomFields] = resource.CustomFields
	}

	return fields
}

func (p *Planner) planCatalogServiceUpdateWithFields(
	current state.CatalogService,
	desired resources.CatalogServiceResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
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

	if _, ok := updateFields[FieldName]; !ok {
		updateFields[FieldName] = current.Name
	}
	if _, ok := updateFields[FieldDisplayName]; !ok {
		updateFields[FieldDisplayName] = current.DisplayName
	}
	if _, hasLabels := updateFields[FieldLabels]; hasLabels {
		updateFields[FieldCurrentLabels] = current.NormalizedLabels
	}

	config := UpdateConfig{
		ResourceType:   ResourceTypeCatalogService,
		ResourceName:   desired.Name,
		ResourceRef:    desired.GetRef(),
		ResourceID:     current.ID,
		DesiredFields:  updateFields,
		ChangedFields:  changedFields,
		CurrentLabels:  current.NormalizedLabels,
		DesiredLabels:  desired.GetLabels(),
		RequiredFields: []string{FieldName, FieldDisplayName},
		Namespace:      namespace,
	}

	change, err := p.genericPlanner.PlanUpdate(context.Background(), config)
	if err != nil {
		p.logger.Error("Failed to plan catalog service update", slog.String("error", err.Error()))
		fields := make(map[string]any, len(updateFields))
		maps.Copy(fields, updateFields)
		changeID := p.nextChangeID(ActionUpdate, ResourceTypeCatalogService, desired.GetRef())
		fallback := PlannedChange{
			ID:            changeID,
			ResourceType:  ResourceTypeCatalogService,
			ResourceRef:   desired.GetRef(),
			ResourceID:    current.ID,
			Action:        ActionUpdate,
			Fields:        fields,
			ChangedFields: changedFields,
			DependsOn:     []string{},
			Namespace:     namespace,
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
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	namespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		namespace = *desired.Kongctl.Namespace
	}

	config := ProtectionChangeConfig{
		ResourceType: ResourceTypeCatalogService,
		ResourceName: desired.Name,
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		OldProtected: labels.IsProtectedResource(current.NormalizedLabels),
		NewProtected: desired.Kongctl != nil && desired.Kongctl.Protected != nil && *desired.Kongctl.Protected,
		Namespace:    namespace,
	}

	change := p.genericPlanner.PlanProtectionChange(context.Background(), config)

	fields := make(map[string]any)
	maps.Copy(fields, updateFields)
	fields[FieldName] = current.Name
	fields[FieldDisplayName] = current.DisplayName

	change.Fields = fields
	if len(changedFields) > 0 {
		change.ChangedFields = changedFields
	}
	plan.AddChange(change)
}

func (p *Planner) planCatalogServiceDelete(current state.CatalogService, plan *Plan) {
	namespace := DefaultNamespace
	if ns, ok := current.NormalizedLabels[labels.NamespaceKey]; ok && ns != "" {
		namespace = ns
	}

	config := DeleteConfig{
		ResourceType: ResourceTypeCatalogService,
		ResourceName: current.Name,
		ResourceRef:  current.Name,
		ResourceID:   current.ID,
		Namespace:    namespace,
	}

	change := p.genericPlanner.PlanDelete(context.Background(), config)
	plan.AddChange(change)
}
