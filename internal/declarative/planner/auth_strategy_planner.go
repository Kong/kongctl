package planner

import (
	"context"
	"fmt"
	"maps"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/fieldcontracts"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
)

// authStrategyPlannerImpl implements planning logic for auth strategy resources
type authStrategyPlannerImpl struct {
	*BasePlanner
}

// NewAuthStrategyPlanner creates a new auth strategy planner
func NewAuthStrategyPlanner(base *BasePlanner) AuthStrategyPlanner {
	return &authStrategyPlannerImpl{
		BasePlanner: base,
	}
}

func (p *authStrategyPlannerImpl) PlannerComponent() string {
	return string(resources.ResourceTypeApplicationAuthStrategy)
}

// PlanChanges generates changes for auth strategy resources
func (p *authStrategyPlannerImpl) PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error {
	// Get namespace from planner context
	namespace := plannerCtx.Namespace
	desired := p.GetDesiredAuthStrategies(namespace)
	if namespace == resources.NamespaceExternal {
		return nil
	}

	// Skip if no auth strategies to plan and not in sync mode
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	// Fetch current managed auth strategies from the specific namespace
	namespaceFilter := []string{namespace}
	currentStrategies, err := p.planner.listManagedAuthStrategies(ctx, namespaceFilter)
	if err != nil {
		// If app auth client is not configured, skip auth strategy planning
		if err.Error() == "AppAuth client not configured" {
			return nil
		}
		return fmt.Errorf("failed to list current auth strategies: %w", err)
	}

	desiredRoots := make([]managedRoot[resources.ApplicationAuthStrategyResource], 0, len(desired))
	for _, strategy := range desired {
		name := strategy.GetMoniker()
		if name == "" {
			continue
		}
		desiredRoots = append(desiredRoots, managedRoot[resources.ApplicationAuthStrategyResource]{
			resource:  strategy,
			name:      name,
			protected: strategy.Kongctl.IsProtected(),
		})
	}

	currentRoots := make([]managedRoot[state.ApplicationAuthStrategy], 0, len(currentStrategies))
	for _, strategy := range currentStrategies {
		currentRoots = append(currentRoots, managedRoot[state.ApplicationAuthStrategy]{
			resource:  strategy,
			name:      strategy.Name,
			protected: labels.IsProtectedResource(strategy.NormalizedLabels),
		})
	}

	return reconcileManagedRoots(p.BasePlanner, ResourceTypeApplicationAuthStrategy, desiredRoots, currentRoots,
		managedRootOperations[resources.ApplicationAuthStrategyResource, state.ApplicationAuthStrategy]{
			diff: p.shouldUpdateAuthStrategy,
			create: func(desired resources.ApplicationAuthStrategyResource, plan *Plan) string {
				p.planAuthStrategyCreate(desired, plan)
				return ""
			},
			update:           p.planAuthStrategyUpdateWithFields,
			changeProtection: p.planAuthStrategyProtectionChangeWithFields,
			remove:           p.planAuthStrategyDelete,
		}, plan)
}

// planAuthStrategyCreate creates a CREATE change for an auth strategy
func (p *authStrategyPlannerImpl) planAuthStrategyCreate(
	strategy resources.ApplicationAuthStrategyResource, plan *Plan,
) {
	fields := make(map[string]any)

	// Extract fields based on strategy type
	var name string
	var displayName string
	var labels map[string]string
	var kongctl *resources.KongctlMeta

	switch strategy.Type {
	case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
		if strategy.AppAuthStrategyKeyAuthRequest != nil {
			name = strategy.AppAuthStrategyKeyAuthRequest.Name
			displayName = strategy.AppAuthStrategyKeyAuthRequest.DisplayName
			labels = strategy.AppAuthStrategyKeyAuthRequest.Labels

			// Set strategy type
			fields[FieldAuthStrategyType] = "key_auth"
		}
	case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
		if strategy.AppAuthStrategyOpenIDConnectRequest != nil {
			name = strategy.AppAuthStrategyOpenIDConnectRequest.Name
			displayName = strategy.AppAuthStrategyOpenIDConnectRequest.DisplayName
			labels = strategy.AppAuthStrategyOpenIDConnectRequest.Labels

			// Set strategy type
			fields[FieldAuthStrategyType] = "openid_connect"

			if providerID := strategy.GetDCRProviderID(); providerID != "" {
				fields[FieldDCRProviderID] = providerID
			}
		}
	}

	if configs := fieldcontracts.ExtractAuthStrategyConfigs(strategy.CreateAppAuthStrategyRequest); configs != nil {
		fields[FieldConfigs] = configs
	}

	kongctl = strategy.Kongctl

	fields[FieldName] = name
	if displayName != "" {
		fields[FieldDisplayName] = displayName
	}

	change := PlannedChange{
		ID:           p.NextChangeID(ActionCreate, ResourceTypeApplicationAuthStrategy, strategy.GetRef()),
		ResourceType: ResourceTypeApplicationAuthStrategy,
		ResourceRef:  strategy.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    []string{},
	}

	// Only set protection if explicitly specified
	if kongctl != nil && kongctl.Protected != nil {
		change.Protection = *kongctl.Protected
	}

	// Extract namespace
	if kongctl != nil && kongctl.Namespace != nil {
		change.Namespace = *kongctl.Namespace
	} else {
		// This should not happen as loader should have set default namespace
		change.Namespace = DefaultNamespace
	}

	// Copy user-defined labels only (protection label will be added during execution)
	if len(labels) > 0 {
		labelsMap := make(map[string]any)
		for k, v := range labels {
			labelsMap[k] = v
		}
		fields[FieldLabels] = labelsMap
	}

	plan.AddChange(change)
}

// shouldUpdateAuthStrategy checks if auth strategy needs update based on configured fields only
func (p *authStrategyPlannerImpl) shouldUpdateAuthStrategy(
	current state.ApplicationAuthStrategy,
	desired resources.ApplicationAuthStrategyResource,
) (bool, map[string]any, map[string]FieldChange) {
	updateFields := make(map[string]any)
	changedFields := make(map[string]FieldChange)

	// First, check if strategy type is changing - this is not supported
	var desiredStrategyType string
	switch desired.Type {
	case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
		desiredStrategyType = "key_auth"
	case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
		desiredStrategyType = "openid_connect"
	}

	if current.StrategyType != desiredStrategyType {
		// Return error via updateFields to be handled by caller
		updateFields[FieldError] = fmt.Sprintf(
			"changing strategy_type from %s to %s is not supported. Please delete and recreate the auth strategy",
			current.StrategyType, desiredStrategyType,
		)
		return true, updateFields, changedFields
	}

	// Extract fields based on strategy type
	var displayName string
	var desiredLabels map[string]string

	switch desired.Type {
	case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
		if desired.AppAuthStrategyKeyAuthRequest != nil {
			displayName = desired.AppAuthStrategyKeyAuthRequest.DisplayName
			desiredLabels = desired.AppAuthStrategyKeyAuthRequest.Labels
		}
	case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
		if desired.AppAuthStrategyOpenIDConnectRequest != nil {
			displayName = desired.AppAuthStrategyOpenIDConnectRequest.DisplayName
			desiredLabels = desired.AppAuthStrategyOpenIDConnectRequest.Labels
		}
	}

	// Only compare fields present in desired configuration
	if displayName != "" {
		if current.DisplayName != displayName {
			updateFields[FieldDisplayName] = displayName
			changedFields[FieldDisplayName] = FieldChange{
				Old: current.DisplayName,
				New: displayName,
			}
		}
	}

	oldConfigs, newConfigs := fieldcontracts.CompareAuthStrategyConfigs(
		desired.CreateAppAuthStrategyRequest,
		current.Configs,
	)
	if newConfigs != nil {
		updateFields[FieldConfigs] = newConfigs
		changedFields[FieldConfigs] = FieldChange{Old: oldConfigs, New: newConfigs}
	}
	if desired.Type == kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect {
		if desired.AppAuthStrategyOpenIDConnectRequest != nil {
			desiredProviderID := desired.GetDCRProviderID()
			comparableProviderValue := p.resolveComparableDCRProviderValue(desiredProviderID)
			if desiredProviderID != "" &&
				current.DCRProviderID != comparableProviderValue &&
				current.DCRProviderName != comparableProviderValue {
				updateFields[FieldDCRProviderID] = desiredProviderID
				changedFields[FieldDCRProviderID] = FieldChange{
					Old: current.DCRProviderID,
					New: desiredProviderID,
				}
			}
		}
	}

	// Check if labels are defined in the desired state
	if desiredLabels != nil {
		// Compare only user labels to determine if update is needed
		if labels.CompareUserLabels(current.NormalizedLabels, desiredLabels) {
			// User labels differ, include all labels in update
			updateFields[FieldLabels] = desiredLabels
			changedFields[FieldLabels] = FieldChange{
				Old: labels.GetUserLabels(current.NormalizedLabels),
				New: labels.GetUserLabels(desiredLabels),
			}
		}
	}

	return len(updateFields) > 0, updateFields, changedFields
}

func (p *authStrategyPlannerImpl) resolveComparableDCRProviderValue(desiredProviderID string) string {
	if desiredProviderID == "" {
		return ""
	}

	ref := desiredProviderID
	if tags.IsRefPlaceholder(desiredProviderID) {
		parsedRef, field, ok := tags.ParseRefPlaceholder(desiredProviderID)
		if !ok || (field != "" && field != FieldID && field != "ID") {
			return desiredProviderID
		}
		ref = parsedRef
	}

	provider := p.planner.resources.GetDCRProviderByRef(ref)
	if provider == nil {
		return desiredProviderID
	}

	if moniker := provider.GetMoniker(); moniker != "" {
		return moniker
	}
	if konnectID := provider.GetKonnectID(); konnectID != "" {
		return konnectID
	}

	return desiredProviderID
}

// planAuthStrategyUpdateWithFields creates an UPDATE change with specific fields
func (p *authStrategyPlannerImpl) planAuthStrategyUpdateWithFields(
	current state.ApplicationAuthStrategy,
	desired resources.ApplicationAuthStrategyResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	fields := make(map[string]any)

	// Store the fields that need updating
	for field, newValue := range updateFields {
		// Skip internal error field
		if field != FieldError {
			fields[field] = newValue
		}
	}

	// Pass strategy type to executor
	fields[FieldStrategyType] = current.StrategyType

	// Pass current labels so executor can properly handle removals
	if _, hasLabels := updateFields[FieldLabels]; hasLabels {
		fields[FieldCurrentLabels] = current.NormalizedLabels
	}

	change := PlannedChange{
		ID:            p.NextChangeID(ActionUpdate, ResourceTypeApplicationAuthStrategy, desired.GetRef()),
		ResourceType:  ResourceTypeApplicationAuthStrategy,
		ResourceRef:   desired.GetRef(),
		ResourceID:    current.ID,
		Action:        ActionUpdate,
		Fields:        fields,
		ChangedFields: changedFields,
		DependsOn:     []string{},
	}

	// Set protection status based on current state
	change.Protection = labels.IsProtectedResource(current.NormalizedLabels)

	// Extract namespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		change.Namespace = *desired.Kongctl.Namespace
	} else {
		// This should not happen as loader should have set default namespace
		change.Namespace = DefaultNamespace
	}

	plan.AddChange(change)
}

// planAuthStrategyProtectionChangeWithFields creates an UPDATE for protection status with optional field updates
func (p *authStrategyPlannerImpl) planAuthStrategyProtectionChangeWithFields(
	current state.ApplicationAuthStrategy,
	desired resources.ApplicationAuthStrategyResource,
	wasProtected, shouldProtect bool,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	fields := make(map[string]any)

	// Include any field updates if unprotecting
	if wasProtected && !shouldProtect && len(updateFields) > 0 {
		maps.Copy(fields, updateFields)
	}

	// Always include name for identification
	fields[FieldName] = current.Name

	// Don't add protection label here - it will be added during execution
	// based on the Protection field

	change := PlannedChange{
		ID:           p.NextChangeID(ActionUpdate, ResourceTypeApplicationAuthStrategy, desired.GetRef()),
		ResourceType: ResourceTypeApplicationAuthStrategy,
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		Protection: ProtectionChange{
			Old: wasProtected,
			New: shouldProtect,
		},
		DependsOn: []string{},
	}
	if len(changedFields) > 0 {
		change.ChangedFields = changedFields
	}

	// Extract namespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		change.Namespace = *desired.Kongctl.Namespace
	} else {
		// This should not happen as loader should have set default namespace
		change.Namespace = DefaultNamespace
	}

	plan.AddChange(change)
}

// planAuthStrategyDelete creates a DELETE change for an auth strategy
func (p *authStrategyPlannerImpl) planAuthStrategyDelete(strategy state.ApplicationAuthStrategy, plan *Plan) {
	change := PlannedChange{
		ID:           p.NextChangeID(ActionDelete, ResourceTypeApplicationAuthStrategy, strategy.Name),
		ResourceType: ResourceTypeApplicationAuthStrategy,
		ResourceRef:  strategy.Name,
		ResourceID:   strategy.ID,
		Action:       ActionDelete,
		Fields:       map[string]any{FieldName: strategy.Name},
		DependsOn:    []string{},
	}

	// Extract namespace from labels (for existing resources being deleted)
	if ns, ok := strategy.NormalizedLabels[labels.NamespaceKey]; ok {
		change.Namespace = ns
	} else {
		// Fallback to default
		change.Namespace = DefaultNamespace
	}

	plan.AddChange(change)
}
