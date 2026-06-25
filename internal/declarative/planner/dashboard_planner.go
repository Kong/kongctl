package planner

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/util"
)

type dashboardPlannerImpl struct {
	*BasePlanner
}

// NewDashboardPlanner creates a new dashboard planner.
func NewDashboardPlanner(base *BasePlanner) DashboardPlanner {
	return &dashboardPlannerImpl{BasePlanner: base}
}

func (p *dashboardPlannerImpl) PlannerComponent() string {
	return string(resources.ResourceTypeDashboard)
}

// PlanChanges generates changes for dashboard resources.
func (p *dashboardPlannerImpl) PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error {
	namespace := plannerCtx.Namespace
	desired := p.GetDesiredDashboards(namespace)

	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	return p.planner.planDashboardChanges(ctx, plannerCtx, desired, plan)
}

func (p *Planner) planDashboardChanges(
	ctx context.Context,
	plannerCtx *Config,
	desired []resources.DashboardResource,
	plan *Plan,
) error {
	p.logger.Debug("planDashboardChanges called",
		slog.Int("desiredCount", len(desired)),
		slog.String("namespace", plannerCtx.Namespace))

	currentDashboards, err := p.listManagedDashboards(ctx, []string{plannerCtx.Namespace})
	if err != nil {
		if state.IsAPIClientError(err) {
			return nil
		}
		return fmt.Errorf("failed to list dashboards: %w", err)
	}

	currentByID, currentByName := indexDashboards(currentDashboards)

	if plan.Metadata.Mode == PlanModeDelete {
		var protectionErrors []error
		for _, desiredDashboard := range desired {
			current, exists, err := matchCurrentDashboard(desiredDashboard, currentByID, currentByName)
			if err != nil {
				return err
			}
			if !exists {
				plan.AddWarning("", fmt.Sprintf(
					"dashboard %q not found in Konnect, skipping delete", desiredDashboard.Name,
				))
				continue
			}

			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			if err := p.validateProtection(
				ResourceTypeDashboard, desiredDashboard.Name, isProtected, ActionDelete,
			); err != nil {
				protectionErrors = append(protectionErrors, err)
			} else {
				p.planDashboardDelete(current, plan)
			}
		}

		if len(protectionErrors) > 0 {
			return dashboardProtectionError(protectionErrors)
		}
		return nil
	}

	var protectionErrors []error
	matchedCurrent := make(map[string]bool)

	for _, desiredDashboard := range desired {
		current, exists, err := matchCurrentDashboard(desiredDashboard, currentByID, currentByName)
		if err != nil {
			return err
		}
		if !exists {
			p.planDashboardCreate(desiredDashboard, plan)
			continue
		}
		matchedCurrent[dashboardIdentity(current)] = true

		isProtected := labels.IsProtectedResource(current.NormalizedLabels)
		shouldProtect := desiredDashboard.Kongctl != nil &&
			desiredDashboard.Kongctl.Protected != nil &&
			*desiredDashboard.Kongctl.Protected

		needsUpdate, updateFields, changedFields := p.shouldUpdateDashboard(current, desiredDashboard)
		if isProtected != shouldProtect {
			protectionChange := &ProtectionChange{Old: isProtected, New: shouldProtect}
			if err := p.validateProtectionWithChange(
				ResourceTypeDashboard, desiredDashboard.Name, isProtected, ActionUpdate, protectionChange, needsUpdate,
			); err != nil {
				protectionErrors = append(protectionErrors, err)
			} else {
				p.planDashboardUpdate(current, desiredDashboard, updateFields, changedFields, plan)
			}
			continue
		}

		if needsUpdate {
			if err := p.validateProtection(ResourceTypeDashboard, desiredDashboard.Name, isProtected, ActionUpdate); err != nil {
				protectionErrors = append(protectionErrors, err)
			} else {
				p.planDashboardUpdate(current, desiredDashboard, updateFields, changedFields, plan)
			}
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		for _, current := range currentDashboards {
			if matchedCurrent[dashboardIdentity(current)] {
				continue
			}

			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			if err := p.validateProtection(ResourceTypeDashboard, current.Name, isProtected, ActionDelete); err != nil {
				protectionErrors = append(protectionErrors, err)
			} else {
				p.planDashboardDelete(current, plan)
			}
		}
	}

	if len(protectionErrors) > 0 {
		return dashboardProtectionError(protectionErrors)
	}

	return nil
}

func (p *Planner) shouldUpdateDashboard(
	current state.Dashboard,
	desired resources.DashboardResource,
) (bool, map[string]any, map[string]FieldChange) {
	updates := make(map[string]any)
	changedFields := make(map[string]FieldChange)

	if current.Name != desired.Name {
		updates[FieldName] = desired.Name
		changedFields[FieldName] = FieldChange{Old: current.Name, New: desired.Name}
	}

	if !reflect.DeepEqual(current.Definition, desired.Definition) {
		updates[FieldDefinition] = desired.Definition
		changedFields[FieldDefinition] = FieldChange{Old: current.Definition, New: desired.Definition}
	}

	if desired.Labels != nil && labels.CompareUserLabels(current.NormalizedLabels, desired.GetLabels()) {
		updates[FieldLabels] = desired.GetLabels()
		changedFields[FieldLabels] = FieldChange{
			Old: labels.GetUserLabels(current.NormalizedLabels),
			New: labels.GetUserLabels(desired.GetLabels()),
		}
	}

	return len(updates) > 0, updates, changedFields
}

func indexDashboards(dashboards []state.Dashboard) (map[string]state.Dashboard, map[string][]state.Dashboard) {
	byID := make(map[string]state.Dashboard)
	byName := make(map[string][]state.Dashboard)
	for _, dashboard := range dashboards {
		if id := getString(dashboard.ID); id != "" {
			byID[id] = dashboard
		}
		byName[dashboard.Name] = append(byName[dashboard.Name], dashboard)
	}
	return byID, byName
}

func matchCurrentDashboard(
	desired resources.DashboardResource,
	currentByID map[string]state.Dashboard,
	currentByName map[string][]state.Dashboard,
) (state.Dashboard, bool, error) {
	if id := dashboardDesiredID(desired); id != "" {
		current, exists := currentByID[id]
		return current, exists, nil
	}

	matches := currentByName[desired.Name]
	switch len(matches) {
	case 0:
		return state.Dashboard{}, false, nil
	case 1:
		return matches[0], true, nil
	default:
		return state.Dashboard{}, false, fmt.Errorf(
			"multiple managed dashboards named %q found in namespace; use a UUID ref or remove duplicates",
			desired.Name,
		)
	}
}

func dashboardDesiredID(desired resources.DashboardResource) string {
	if id := desired.GetKonnectID(); id != "" {
		return id
	}
	if util.IsValidUUID(desired.Ref) {
		return desired.Ref
	}
	return ""
}

func dashboardIdentity(dashboard state.Dashboard) string {
	if id := getString(dashboard.ID); id != "" {
		return "id:" + id
	}
	return "name:" + dashboard.Name
}

func (p *Planner) planDashboardCreate(resource resources.DashboardResource, plan *Plan) {
	namespace, protection := dashboardNamespaceAndProtection(resource)

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeDashboard, resource.GetRef()),
		Action:       ActionCreate,
		ResourceType: ResourceTypeDashboard,
		ResourceRef:  resource.GetRef(),
		Fields:       extractDashboardFields(resource),
		Namespace:    namespace,
		Protection:   protection,
	}
	plan.AddChange(change)
}

func (p *Planner) planDashboardUpdate(
	current state.Dashboard,
	desired resources.DashboardResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	namespace, _ := dashboardNamespaceAndProtection(desired)
	fields := make(map[string]any)
	maps.Copy(fields, updateFields)
	fields[FieldName] = desired.Name
	fields[FieldDefinition] = desired.Definition
	if _, hasLabels := fields[FieldLabels]; hasLabels {
		fields[FieldCurrentLabels] = current.NormalizedLabels
	}

	protection := ProtectionChange{
		Old: labels.IsProtectedResource(current.NormalizedLabels),
		New: desired.Kongctl != nil && desired.Kongctl.Protected != nil && *desired.Kongctl.Protected,
	}

	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeDashboard, desired.GetRef()),
		Action:        ActionUpdate,
		ResourceType:  ResourceTypeDashboard,
		ResourceRef:   desired.GetRef(),
		ResourceID:    getString(current.ID),
		Fields:        fields,
		ChangedFields: changedFields,
		Namespace:     namespace,
		Protection:    protection,
	}
	plan.AddChange(change)
}

func (p *Planner) planDashboardDelete(current state.Dashboard, plan *Plan) {
	namespace := DefaultNamespace
	if ns, ok := current.NormalizedLabels[labels.NamespaceKey]; ok && ns != "" {
		namespace = ns
	}
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeDashboard, current.Name),
		Action:       ActionDelete,
		ResourceType: ResourceTypeDashboard,
		ResourceRef:  current.Name,
		ResourceID:   getString(current.ID),
		Fields: map[string]any{
			FieldName:       current.Name,
			FieldDefinition: current.Definition,
		},
		Namespace: namespace,
	}
	plan.AddChange(change)
}

func extractDashboardFields(resource resources.DashboardResource) map[string]any {
	fields := map[string]any{
		FieldName:       resource.Name,
		FieldDefinition: resource.Definition,
	}
	if resource.Labels != nil {
		fields[FieldLabels] = resource.GetLabels()
	}
	return fields
}

func dashboardNamespaceAndProtection(resource resources.DashboardResource) (string, any) {
	namespace := DefaultNamespace
	if resource.Kongctl != nil && resource.Kongctl.Namespace != nil {
		namespace = *resource.Kongctl.Namespace
	}

	var protection any
	if resource.Kongctl != nil && resource.Kongctl.Protected != nil {
		protection = *resource.Kongctl.Protected
	}
	return namespace, protection
}

func dashboardProtectionError(protectionErrors []error) error {
	var errMsg strings.Builder
	errMsg.WriteString("Cannot generate plan due to protected resources:\n")
	for _, err := range protectionErrors {
		fmt.Fprintf(&errMsg, "- %s\n", err.Error())
	}
	errMsg.WriteString("\nTo proceed, first update these resources to set protected: false")
	return fmt.Errorf("%s", errMsg.String())
}
