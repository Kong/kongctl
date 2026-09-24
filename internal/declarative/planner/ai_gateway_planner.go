package planner

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"reflect"

	"github.com/Masterminds/semver/v3"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

type aiGatewayPlannerImpl struct {
	*BasePlanner
}

// NewAIGatewayPlanner creates a new AI Gateway planner.
func NewAIGatewayPlanner(base *BasePlanner) AIGatewayPlanner {
	return &aiGatewayPlannerImpl{BasePlanner: base}
}

func (p *aiGatewayPlannerImpl) PlannerComponent() string {
	return string(resources.ResourceTypeAIGateway)
}

// PlanChanges generates changes for AI Gateway resources.
func (p *aiGatewayPlannerImpl) PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error {
	namespace := plannerCtx.Namespace
	desired := p.GetDesiredAIGateways(namespace)

	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	return p.planner.planAIGatewayChanges(ctx, plannerCtx, desired, plan)
}

func (p *Planner) planAIGatewayChanges(
	ctx context.Context,
	plannerCtx *Config,
	desired []resources.AIGatewayResource,
	plan *Plan,
) error {
	namespace := plannerCtx.Namespace
	p.logger.Debug("planAIGatewayChanges called",
		slog.Int("desiredCount", len(desired)),
		slog.String("namespace", namespace))

	var currentGateways []state.AIGateway
	if namespace != resources.NamespaceExternal {
		var err error
		currentGateways, err = p.listManagedAIGateways(ctx, []string{namespace})
		if err != nil {
			if state.IsAPIClientError(err) {
				return nil
			}
			return fmt.Errorf("failed to list AI Gateways: %w", err)
		}
	}

	currentByName := indexAIGateways(currentGateways)

	// The adapter tracks the actual update change for runtime/child ordering.
	// Reset it for each parent; blocked updates must not inherit a prior ID.
	var gatewayUpdateID string
	updateGateway := func(
		current state.AIGateway, desired resources.AIGatewayResource,
		fields map[string]any, changed map[string]FieldChange, plan *Plan,
	) {
		gatewayUpdateID = p.planAIGatewayUpdate(current, desired, fields, changed, plan)
	}
	reconciler := newManagedRootReconciler(NewBasePlanner(p), ResourceTypeAIGateway,
		managedRootOperations[resources.AIGatewayResource, state.AIGateway]{
			diff:   p.shouldUpdateAIGateway,
			create: p.planAIGatewayCreate,
			update: updateGateway,
			changeProtection: func(
				current state.AIGateway, desired resources.AIGatewayResource,
				_, _ bool, fields map[string]any, changed map[string]FieldChange, plan *Plan,
			) {
				updateGateway(current, desired, fields, changed, plan)
			},
			remove: p.planAIGatewayDelete,
		}, plan)

	if plan.Metadata.Mode == PlanModeDelete {
		for _, desiredGateway := range desired {
			if desiredGateway.IsExternal() {
				continue
			}
			reconciler.deleteDesired(desiredGateway.Name, findManagedRoot(currentByName, desiredGateway.Name))
		}
		return reconciler.errors.Error()
	}

	matchedCurrent := make(map[string]bool)

	for _, desiredGateway := range desired {
		if desiredGateway.IsExternal() {
			if err := p.planExternalAIGatewayChildren(ctx, plannerCtx, namespace, desiredGateway, plan); err != nil {
				return err
			}
			continue
		}

		current := findManagedRoot(currentByName, desiredGateway.Name)
		gatewayID := ""
		if current != nil {
			gatewayID = current.resource.ID
			matchedCurrent[aiGatewayIdentity(current.resource)] = true
		}
		gatewayUpdateID = ""
		gatewayChangeID := reconciler.reconcile(managedRoot[resources.AIGatewayResource]{
			resource: desiredGateway, name: desiredGateway.Name,
			protected: desiredGateway.Kongctl != nil &&
				desiredGateway.Kongctl.Protected != nil && *desiredGateway.Kongctl.Protected,
		}, current)

		childStart := len(plan.Changes)
		if err := p.planAIGatewayChildren(
			ctx, plannerCtx, namespace, desiredGateway, gatewayID, gatewayChangeID, plan,
		); err != nil {
			return err
		}
		if current != nil && gatewayUpdateID != "" && desiredGateway.MinRuntimeVersion != nil &&
			getString(current.resource.MinRuntimeVersion) != *desiredGateway.MinRuntimeVersion {
			orderAIGatewayRuntimeChange(plan, gatewayUpdateID, childStart,
				getString(current.resource.MinRuntimeVersion), *desiredGateway.MinRuntimeVersion)
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		for _, current := range currentGateways {
			if matchedCurrent[aiGatewayIdentity(current)] {
				continue
			}

			reconciler.remove(observedAIGatewayRoot(current))
		}
	}
	return reconciler.errors.Error()
}

func (p *Planner) planExternalAIGatewayChildren(
	ctx context.Context,
	plannerCtx *Config,
	namespace string,
	desiredGateway resources.AIGatewayResource,
	plan *Plan,
) error {
	gatewayID := desiredGateway.GetKonnectID()
	if gatewayID == "" {
		plan.AddWarning("", fmt.Sprintf(
			"external ai_gateway %q has no resolved ID; skipping AI Gateway child planning",
			desiredGateway.GetRef(),
		))
		return nil
	}

	return p.planAIGatewayChildren(ctx, plannerCtx, namespace, desiredGateway, gatewayID, "", plan)
}

func (p *Planner) shouldUpdateAIGateway(
	current state.AIGateway,
	desired resources.AIGatewayResource,
) (bool, map[string]any, map[string]FieldChange) {
	updates := make(map[string]any)
	changedFields := make(map[string]FieldChange)
	if desired.MinRuntimeVersion != nil && getString(current.MinRuntimeVersion) != *desired.MinRuntimeVersion {
		updates[FieldMinRuntimeVersion] = *desired.MinRuntimeVersion
		changedFields[FieldMinRuntimeVersion] = FieldChange{
			Old: getString(current.MinRuntimeVersion), New: *desired.MinRuntimeVersion,
		}
	}
	if desired.RuntimeAutoUpgrade != nil &&
		(current.RuntimeAutoUpgrade == nil || *current.RuntimeAutoUpgrade != *desired.RuntimeAutoUpgrade) {
		updates[FieldRuntimeAutoUpgrade] = *desired.RuntimeAutoUpgrade
		changedFields[FieldRuntimeAutoUpgrade] = FieldChange{
			Old: current.RuntimeAutoUpgrade, New: *desired.RuntimeAutoUpgrade,
		}
	}

	if current.DisplayName != desired.DisplayName {
		updates[FieldDisplayName] = desired.DisplayName
		changedFields[FieldDisplayName] = FieldChange{Old: current.DisplayName, New: desired.DisplayName}
	}

	if desired.Description != nil {
		currentDescription := getString(current.Description)
		if currentDescription != *desired.Description {
			updates[FieldDescription] = *desired.Description
			changedFields[FieldDescription] = FieldChange{Old: currentDescription, New: *desired.Description}
		}
	}

	if desired.ProxyUrls != nil && !reflect.DeepEqual(current.ProxyUrls, desired.ProxyUrls) {
		updates[FieldProxyURLs] = desired.ProxyUrls
		changedFields[FieldProxyURLs] = FieldChange{Old: current.ProxyUrls, New: desired.ProxyUrls}
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

func indexAIGateways(gateways []state.AIGateway) map[string]managedRoot[state.AIGateway] {
	byName := make(map[string]managedRoot[state.AIGateway], len(gateways))
	for _, gateway := range gateways {
		if gateway.Name != "" {
			byName[gateway.Name] = observedAIGatewayRoot(gateway)
		}
	}
	return byName
}

func observedAIGatewayRoot(gateway state.AIGateway) managedRoot[state.AIGateway] {
	return managedRoot[state.AIGateway]{
		resource: gateway, name: gateway.Name, protected: labels.IsProtectedResource(gateway.NormalizedLabels),
	}
}

func aiGatewayIdentity(gateway state.AIGateway) string {
	if gateway.ID != "" {
		return "id:" + gateway.ID
	}
	if gateway.Name != "" {
		return "name:" + gateway.Name
	}
	return "display_name:" + gateway.DisplayName
}

func (p *Planner) planAIGatewayCreate(resource resources.AIGatewayResource, plan *Plan) string {
	namespace, protection := aiGatewayNamespaceAndProtection(resource)
	changeID := p.nextChangeID(ActionCreate, ResourceTypeAIGateway, resource.GetRef())

	change := PlannedChange{
		ID:           changeID,
		Action:       ActionCreate,
		ResourceType: ResourceTypeAIGateway,
		ResourceRef:  resource.GetRef(),
		Fields:       extractAIGatewayFields(resource),
		Namespace:    namespace,
		Protection:   protection,
	}
	plan.AddChange(change)
	return changeID
}

func (p *Planner) planAIGatewayUpdate(
	current state.AIGateway,
	desired resources.AIGatewayResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) string {
	namespace, _ := aiGatewayNamespaceAndProtection(desired)
	fields := make(map[string]any)
	maps.Copy(fields, updateFields)
	if _, changed := fields[FieldRuntimeAutoUpgrade]; !changed && current.RuntimeAutoUpgrade != nil {
		fields[FieldRuntimeAutoUpgrade] = *current.RuntimeAutoUpgrade
	}
	fields[FieldName] = current.Name
	if fields[FieldName] == "" {
		fields[FieldName] = desired.Name
	}
	fields[FieldDisplayName] = desired.DisplayName
	if _, hasLabels := fields[FieldLabels]; hasLabels {
		fields[FieldCurrentLabels] = current.NormalizedLabels
	}

	protection := ProtectionChange{
		Old: labels.IsProtectedResource(current.NormalizedLabels),
		New: desired.Kongctl != nil && desired.Kongctl.Protected != nil && *desired.Kongctl.Protected,
	}

	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeAIGateway, desired.GetRef()),
		Action:        ActionUpdate,
		ResourceType:  ResourceTypeAIGateway,
		ResourceRef:   desired.GetRef(),
		ResourceID:    current.ID,
		Fields:        fields,
		ChangedFields: changedFields,
		Namespace:     namespace,
		Protection:    protection,
	}
	plan.AddChange(change)
	return change.ID
}

// Runtime upgrades must precede new features; downgrades must follow their removal.
func orderAIGatewayRuntimeChange(plan *Plan, gatewayChangeID string, childStart int, oldVersion, newVersion string) {
	oldRuntime, oldErr := semver.NewVersion(oldVersion)
	newRuntime, newErr := semver.NewVersion(newVersion)
	downgrade := oldErr == nil && newErr == nil && newRuntime.LessThan(oldRuntime)
	for i := childStart; i < len(plan.Changes); i++ {
		child := &plan.Changes[i]
		if downgrade {
			for j := range plan.Changes[:childStart] {
				if plan.Changes[j].ID == gatewayChangeID {
					plan.Changes[j].DependsOn = append(plan.Changes[j].DependsOn, child.ID)
					break
				}
			}
		} else if child.Action != ActionDelete {
			child.DependsOn = append(child.DependsOn, gatewayChangeID)
		}
	}
}

func (p *Planner) planAIGatewayDelete(current state.AIGateway, plan *Plan) {
	namespace := DefaultNamespace
	if ns, ok := current.NormalizedLabels[labels.NamespaceKey]; ok && ns != "" {
		namespace = ns
	}
	resourceRef := current.Name
	if resourceRef == "" {
		resourceRef = current.DisplayName
	}
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGateway, resourceRef),
		Action:       ActionDelete,
		ResourceType: ResourceTypeAIGateway,
		ResourceRef:  resourceRef,
		ResourceID:   current.ID,
		Fields: map[string]any{
			FieldName:        resourceRef,
			FieldDisplayName: current.DisplayName,
		},
		Namespace: namespace,
	}
	plan.AddChange(change)
}

func extractAIGatewayFields(resource resources.AIGatewayResource) map[string]any {
	fields := make(map[string]any)
	if resource.MinRuntimeVersion != nil {
		fields[FieldMinRuntimeVersion] = *resource.MinRuntimeVersion
	}
	if resource.RuntimeAutoUpgrade != nil {
		fields[FieldRuntimeAutoUpgrade] = *resource.RuntimeAutoUpgrade
	}
	fields[FieldName] = resource.Name
	fields[FieldDisplayName] = resource.DisplayName
	if resource.DeploymentType != nil {
		fields[FieldDeploymentType] = *resource.DeploymentType
	}
	if resource.Description != nil {
		fields[FieldDescription] = *resource.Description
	}
	if resource.ProxyUrls != nil {
		fields[FieldProxyURLs] = resource.ProxyUrls
	}
	if resource.Labels != nil {
		fields[FieldLabels] = resource.GetLabels()
	}
	return fields
}

func aiGatewayNamespaceAndProtection(resource resources.AIGatewayResource) (string, any) {
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
