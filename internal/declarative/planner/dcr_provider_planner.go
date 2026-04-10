package planner

import (
	"context"
	"fmt"
	"maps"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

type dcrProviderPlannerImpl struct {
	*BasePlanner
}

func NewDCRProviderPlanner(base *BasePlanner) DCRProviderPlanner {
	return &dcrProviderPlannerImpl{BasePlanner: base}
}

func (p *dcrProviderPlannerImpl) PlannerComponent() string {
	return string(resources.ResourceTypeDCRProvider)
}

func (p *dcrProviderPlannerImpl) PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error {
	namespace := plannerCtx.Namespace
	desired := p.GetDesiredDCRProviders(namespace)

	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	currentProviders, err := p.planner.listManagedDCRProviders(ctx, []string{namespace})
	if err != nil {
		if state.IsAPIClientError(err) {
			return nil
		}
		return fmt.Errorf("failed to list current DCR providers: %w", err)
	}

	currentByName := make(map[string]state.DCRProvider)
	for _, provider := range currentProviders {
		currentByName[provider.Name] = provider
	}

	protectionErrors := &ProtectionErrorCollector{}

	if plan.Metadata.Mode == PlanModeDelete {
		for _, desiredProvider := range desired {
			current, exists := currentByName[desiredProvider.Name]
			if !exists {
				plan.AddWarning("", fmt.Sprintf("dcr_provider %q not found in Konnect, skipping delete", desiredProvider.Name))
				continue
			}

			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			err := p.ValidateProtection("dcr_provider", desiredProvider.Name, isProtected, ActionDelete)
			protectionErrors.Add(err)
			if err == nil {
				p.planDCRProviderDelete(current, plan)
			}
		}

		if protectionErrors.HasErrors() {
			return protectionErrors.Error()
		}
		return nil
	}

	for _, desiredProvider := range desired {
		current, exists := currentByName[desiredProvider.Name]
		if !exists {
			p.planDCRProviderCreate(desiredProvider, plan)
			continue
		}

		isProtected := labels.IsProtectedResource(current.NormalizedLabels)
		shouldProtect := false
		if desiredProvider.Kongctl != nil && desiredProvider.Kongctl.Protected != nil && *desiredProvider.Kongctl.Protected {
			shouldProtect = true
		}

		needsUpdate, updateFields, changedFields := p.shouldUpdateDCRProvider(current, desiredProvider)
		if isProtected != shouldProtect {
			protectionChange := &ProtectionChange{Old: isProtected, New: shouldProtect}
			err := p.ValidateProtectionWithChange(
				"dcr_provider", desiredProvider.Name, isProtected, ActionUpdate, protectionChange, needsUpdate,
			)
			protectionErrors.Add(err)
			if err == nil {
				p.planDCRProviderProtectionChangeWithFields(
					current, desiredProvider, isProtected, shouldProtect, updateFields, changedFields, plan,
				)
			}
			continue
		}

		if needsUpdate {
			if errMsg, hasError := updateFields[FieldError].(string); hasError {
				protectionErrors.Add(fmt.Errorf("%s", errMsg))
			} else {
				err := p.ValidateProtection("dcr_provider", desiredProvider.Name, isProtected, ActionUpdate)
				protectionErrors.Add(err)
				if err == nil {
					p.planDCRProviderUpdateWithFields(current, desiredProvider, updateFields, changedFields, plan)
				}
			}
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		desiredNames := make(map[string]bool)
		for _, provider := range desired {
			desiredNames[provider.Name] = true
		}

		for name, current := range currentByName {
			if desiredNames[name] {
				continue
			}

			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			err := p.ValidateProtection("dcr_provider", name, isProtected, ActionDelete)
			protectionErrors.Add(err)
			if err == nil {
				p.planDCRProviderDelete(current, plan)
			}
		}
	}

	if protectionErrors.HasErrors() {
		return protectionErrors.Error()
	}

	return nil
}

func (p *dcrProviderPlannerImpl) planDCRProviderCreate(
	provider resources.DCRProviderResource,
	plan *Plan,
) {
	fields := provider.ToCreatePayload()
	fields[FieldName] = provider.Name

	change := PlannedChange{
		ID:           p.NextChangeID(ActionCreate, "dcr_provider", provider.GetRef()),
		ResourceType: "dcr_provider",
		ResourceRef:  provider.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    []string{},
	}

	if provider.Kongctl != nil && provider.Kongctl.Protected != nil {
		change.Protection = *provider.Kongctl.Protected
	}
	change.Namespace = resources.GetNamespace(provider.Kongctl)

	plan.AddChange(change)
}

func (p *dcrProviderPlannerImpl) shouldUpdateDCRProvider(
	current state.DCRProvider,
	desired resources.DCRProviderResource,
) (bool, map[string]any, map[string]FieldChange) {
	updateFields := make(map[string]any)
	changedFields := make(map[string]FieldChange)

	if current.ProviderType != desired.ProviderType {
		updateFields[FieldError] = fmt.Sprintf(
			"changing provider_type from %s to %s is not supported. Please delete and recreate the dcr provider",
			current.ProviderType, desired.ProviderType,
		)
		return true, updateFields, changedFields
	}

	if desired.DisplayName != "" && current.DisplayName != desired.DisplayName {
		updateFields[FieldDisplayName] = desired.DisplayName
		changedFields[FieldDisplayName] = FieldChange{Old: current.DisplayName, New: desired.DisplayName}
	}

	if desired.Issuer != "" && current.Issuer != desired.Issuer {
		updateFields[FieldDCRProviderIssuer] = desired.Issuer
		changedFields[FieldDCRProviderIssuer] = FieldChange{Old: current.Issuer, New: desired.Issuer}
	}

	if desired.DCRConfig != nil && !reflect.DeepEqual(current.DCRConfig, desired.DCRConfig) {
		updateFields[FieldDCRProviderConfig] = desired.DCRConfig
		changedFields[FieldDCRProviderConfig] = FieldChange{Old: current.DCRConfig, New: desired.DCRConfig}
	}

	if desired.Labels != nil && labels.CompareUserLabels(current.NormalizedLabels, desired.Labels) {
		updateFields[FieldLabels] = desired.Labels
		changedFields[FieldLabels] = FieldChange{
			Old: labels.GetUserLabels(current.NormalizedLabels),
			New: labels.GetUserLabels(desired.Labels),
		}
	}

	return len(updateFields) > 0, updateFields, changedFields
}

func (p *dcrProviderPlannerImpl) planDCRProviderUpdateWithFields(
	current state.DCRProvider,
	desired resources.DCRProviderResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	fields := make(map[string]any)
	for field, value := range updateFields {
		if field != FieldError {
			fields[field] = value
		}
	}
	fields[FieldName] = current.Name
	if _, hasLabels := updateFields[FieldLabels]; hasLabels {
		fields[FieldCurrentLabels] = current.NormalizedLabels
	}

	change := PlannedChange{
		ID:            p.NextChangeID(ActionUpdate, "dcr_provider", desired.GetRef()),
		ResourceType:  "dcr_provider",
		ResourceRef:   desired.GetRef(),
		ResourceID:    current.ID,
		Action:        ActionUpdate,
		Fields:        fields,
		ChangedFields: changedFields,
		DependsOn:     []string{},
		Namespace:     resources.GetNamespace(desired.Kongctl),
	}
	change.Protection = labels.IsProtectedResource(current.NormalizedLabels)

	plan.AddChange(change)
}

func (p *dcrProviderPlannerImpl) planDCRProviderProtectionChangeWithFields(
	current state.DCRProvider,
	desired resources.DCRProviderResource,
	wasProtected, shouldProtect bool,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	fields := make(map[string]any)
	if wasProtected && !shouldProtect && len(updateFields) > 0 {
		maps.Copy(fields, updateFields)
	}
	fields[FieldName] = current.Name

	change := PlannedChange{
		ID:           p.NextChangeID(ActionUpdate, "dcr_provider", desired.GetRef()),
		ResourceType: "dcr_provider",
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		Protection: ProtectionChange{
			Old: wasProtected,
			New: shouldProtect,
		},
		DependsOn: []string{},
		Namespace: resources.GetNamespace(desired.Kongctl),
	}
	if len(changedFields) > 0 {
		change.ChangedFields = changedFields
	}

	plan.AddChange(change)
}

func (p *dcrProviderPlannerImpl) planDCRProviderDelete(provider state.DCRProvider, plan *Plan) {
	change := PlannedChange{
		ID:           p.NextChangeID(ActionDelete, "dcr_provider", provider.Name),
		ResourceType: "dcr_provider",
		ResourceRef:  provider.Name,
		ResourceID:   provider.ID,
		Action:       ActionDelete,
		Fields:       map[string]any{FieldName: provider.Name},
		DependsOn:    []string{},
	}
	if ns, ok := provider.NormalizedLabels[labels.NamespaceKey]; ok {
		change.Namespace = ns
	} else {
		change.Namespace = DefaultNamespace
	}

	plan.AddChange(change)
}
