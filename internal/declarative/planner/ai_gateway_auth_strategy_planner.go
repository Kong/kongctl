package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/util"
)

func (p *Planner) planAIGatewayAuthStrategyChanges(
	ctx context.Context,
	_ *Config,
	namespace string,
	gatewayName string,
	gatewayID string,
	gatewayRef string,
	gatewayChangeID string,
	desired []resources.AIGatewayAuthStrategyResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning AI Gateway Auth Strategy changes",
		slog.String("gateway_name", gatewayName),
		slog.String("gateway_id", gatewayID),
		slog.String("gateway_ref", gatewayRef),
		slog.String("gateway_change_id", gatewayChangeID),
		slog.Int("desired_count", len(desired)),
		slog.String("namespace", namespace),
	)

	if gatewayID == "" {
		p.planAIGatewayAuthStrategyCreatesForNewGateway(
			namespace,
			gatewayRef,
			gatewayName,
			gatewayChangeID,
			desired,
			plan,
		)
		return nil
	}

	currentProviders, err := p.client.ListAIGatewayAuthStrategies(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway Auth Strategies for gateway %s: %w", gatewayID, err)
	}

	currentByID, currentByName := indexAIGatewayAuthStrategies(currentProviders)

	desiredKeys := make(map[string]bool)
	for _, desiredProvider := range desired {
		desiredKeys[desiredProvider.Name] = true
		if id := aiGatewayAuthStrategyDesiredID(desiredProvider); id != "" {
			desiredKeys[id] = true
		}

		current, exists := matchCurrentAIGatewayAuthStrategy(desiredProvider, currentByID, currentByName)
		if !exists {
			p.planAIGatewayAuthStrategyCreate(
				namespace, gatewayRef, gatewayName, gatewayID, desiredProvider, nil, plan,
			)
			continue
		}

		fullProvider, err := p.client.GetAIGatewayAuthStrategy(ctx, gatewayID, current.ID)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway Auth Strategy %s: %w", current.ID, err)
		}
		if fullProvider == nil {
			p.planAIGatewayAuthStrategyCreate(
				namespace, gatewayRef, gatewayName, gatewayID, desiredProvider, nil, plan,
			)
			continue
		}

		needsUpdate, updateFields, changedFields, err := shouldUpdateAIGatewayAuthStrategy(*fullProvider, desiredProvider)
		if err != nil {
			return err
		}
		if !needsUpdate {
			continue
		}

		p.planAIGatewayAuthStrategyUpdate(
			namespace, gatewayRef, gatewayID, current.ID, desiredProvider, updateFields, changedFields, plan,
		)
	}

	if plan.Metadata.Mode == PlanModeSync && !p.isAIGatewayExternal(gatewayRef) {
		for _, current := range currentProviders {
			if desiredKeys[current.ID] || desiredKeys[current.Name] {
				continue
			}

			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			if err := p.validateProtection(
				ResourceTypeAIGatewayAuthStrategy,
				current.Name,
				isProtected,
				ActionDelete,
			); err != nil {
				return err
			}
			p.planAIGatewayAuthStrategyDelete(namespace, gatewayRef, gatewayID, current.ID, current.Name, plan)
		}
	}

	return nil
}

func indexAIGatewayAuthStrategies(
	providers []state.AIGatewayAuthStrategy,
) (map[string]state.AIGatewayAuthStrategy, map[string]state.AIGatewayAuthStrategy) {
	byID := make(map[string]state.AIGatewayAuthStrategy)
	byName := make(map[string]state.AIGatewayAuthStrategy)
	for _, provider := range providers {
		if provider.ID != "" {
			byID[provider.ID] = provider
		}
		if provider.Name != "" {
			byName[provider.Name] = provider
		}
	}
	return byID, byName
}

func matchCurrentAIGatewayAuthStrategy(
	desired resources.AIGatewayAuthStrategyResource,
	currentByID map[string]state.AIGatewayAuthStrategy,
	currentByName map[string]state.AIGatewayAuthStrategy,
) (state.AIGatewayAuthStrategy, bool) {
	if id := aiGatewayAuthStrategyDesiredID(desired); id != "" {
		current, exists := currentByID[id]
		return current, exists
	}
	current, exists := currentByName[desired.Name]
	return current, exists
}

func aiGatewayAuthStrategyDesiredID(desired resources.AIGatewayAuthStrategyResource) string {
	if id := desired.GetKonnectID(); id != "" {
		return id
	}
	if util.IsValidUUID(desired.Ref) {
		return desired.Ref
	}
	return ""
}

func (p *Planner) planAIGatewayAuthStrategyCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	providers []resources.AIGatewayAuthStrategyResource,
	plan *Plan,
) {
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}

	for _, provider := range providers {
		p.planAIGatewayAuthStrategyCreate(namespace, gatewayRef, gatewayName, "", provider, dependsOn, plan)
	}
}

func (p *Planner) planAIGatewayAuthStrategyCreate(
	namespace string,
	gatewayRef string,
	_ string,
	gatewayID string,
	provider resources.AIGatewayAuthStrategyResource,
	dependsOn []string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayAuthStrategy, provider.Ref),
		ResourceType: ResourceTypeAIGatewayAuthStrategy,
		ResourceRef:  provider.Ref,
		Action:       ActionCreate,
		Fields:       extractAIGatewayAuthStrategyFields(provider),
		Namespace:    namespace,
		DependsOn:    dependsOn,
	}

	if gatewayID != "" {
		change.Parent = &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		}
	} else {
		change.References = map[string]ReferenceInfo{
			FieldAIGatewayID: {
				Ref: gatewayRef,
				LookupFields: map[string]string{
					FieldName: gatewayRef,
				},
			},
		}
	}

	plan.AddChange(change)
}

func (p *Planner) planAIGatewayAuthStrategyUpdate(
	namespace string,
	gatewayRef string,
	gatewayID string,
	providerID string,
	provider resources.AIGatewayAuthStrategyResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	if len(updateFields) == 0 {
		return
	}

	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeAIGatewayAuthStrategy, provider.Ref),
		ResourceType:  ResourceTypeAIGatewayAuthStrategy,
		ResourceRef:   provider.Ref,
		ResourceID:    providerID,
		Action:        ActionUpdate,
		Fields:        updateFields,
		ChangedFields: changedFields,
		Namespace:     namespace,
		Parent: &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		},
	}
	plan.AddChange(change)
}

func (p *Planner) planAIGatewayAuthStrategyDelete(
	namespace string,
	gatewayRef string,
	gatewayID string,
	providerID string,
	providerName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGatewayAuthStrategy, providerName),
		ResourceType: ResourceTypeAIGatewayAuthStrategy,
		ResourceRef:  providerName,
		ResourceID:   providerID,
		Action:       ActionDelete,
		Namespace:    namespace,
		Fields: map[string]any{
			FieldName: providerName,
		},
		Parent: &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		},
	}
	plan.AddChange(change)
}

func shouldUpdateAIGatewayAuthStrategy(
	current state.AIGatewayAuthStrategy,
	desired resources.AIGatewayAuthStrategyResource,
) (bool, map[string]any, map[string]FieldChange, error) {
	updateFields := make(map[string]any)
	changedFields := make(map[string]FieldChange)

	if current.Type != desired.Type {
		return false, nil, nil, fmt.Errorf(
			"changing AI Gateway Auth Strategy type from %s to %s is not supported. Please delete and recreate the provider",
			current.Type, desired.Type,
		)
	}

	if current.DisplayName != desired.DisplayName {
		changedFields[FieldDisplayName] = FieldChange{Old: current.DisplayName, New: desired.DisplayName}
	}

	if desired.Labels != nil && labels.CompareUserLabels(current.NormalizedLabels, desired.Labels) {
		changedFields[FieldLabels] = FieldChange{
			Old: labels.GetUserLabels(current.NormalizedLabels),
			New: labels.GetUserLabels(desired.Labels),
		}
	}

	if desired.ManagedBy != nil && !reflect.DeepEqual(current.ManagedBy, desired.ManagedBy) {
		changedFields[FieldManagedBy] = FieldChange{Old: current.ManagedBy, New: desired.ManagedBy}
	}

	if desired.Config != nil && aiGatewayAuthStrategyConfigChanged(current.Config, desired.Config) {
		changedFields[FieldConfig] = FieldChange{
			Old: scrubAIGatewayAuthStrategySecretFields(normalizeAuthStrategyConfigForCompare(current.Config)),
			New: scrubAIGatewayAuthStrategySecretFields(normalizeAuthStrategyConfigForCompare(desired.Config)),
		}
	}

	if len(changedFields) == 0 {
		return false, updateFields, changedFields, nil
	}

	updateFields = extractAIGatewayAuthStrategyUpdateFields(current, desired)
	return true, updateFields, changedFields, nil
}

func extractAIGatewayAuthStrategyUpdateFields(
	current state.AIGatewayAuthStrategy,
	desired resources.AIGatewayAuthStrategyResource,
) map[string]any {
	fields := extractAIGatewayAuthStrategyFields(desired)
	fields[FieldConfig] = mergeAIGatewayAuthStrategyConfig(current.Config, desired.Config)
	if desired.Labels == nil && current.Labels != nil {
		fields[FieldLabels] = current.Labels
	}
	if desired.ManagedBy == nil && current.ManagedBy != nil {
		fields[FieldManagedBy] = current.ManagedBy
	}
	return fields
}

func extractAIGatewayAuthStrategyFields(provider resources.AIGatewayAuthStrategyResource) map[string]any {
	fields := map[string]any{
		FieldName:        provider.Name,
		FieldType:        provider.Type,
		FieldDisplayName: provider.DisplayName,
		FieldConfig:      provider.Config,
	}
	if provider.Labels != nil {
		fields[FieldLabels] = provider.Labels
	}
	if provider.ManagedBy != nil {
		fields[FieldManagedBy] = provider.ManagedBy
	}
	return fields
}

func aiGatewayAuthStrategyConfigChanged(current, desired map[string]any) bool {
	currentComparable := normalizeAuthStrategyConfigForCompare(current)
	desiredComparable := normalizeAuthStrategyConfigForCompare(desired)
	projectAIGatewayAuthStrategyConfigForComparison(currentComparable, desiredComparable)
	currentComparable = scrubAIGatewayAuthStrategySecretFields(currentComparable).(map[string]any)
	desiredComparable = scrubAIGatewayAuthStrategySecretFields(desiredComparable).(map[string]any)
	return !reflect.DeepEqual(currentComparable, desiredComparable)
}

func projectAIGatewayAuthStrategyConfigForComparison(currentCompare map[string]any, desiredCompare map[string]any) {
	if currentCompare == nil || desiredCompare == nil {
		return
	}
	for key := range currentCompare {
		desiredValue, declared := desiredCompare[key]
		if !declared {
			delete(currentCompare, key)
			continue
		}
		currentMap, currentIsMap := currentCompare[key].(map[string]any)
		desiredMap, desiredIsMap := desiredValue.(map[string]any)
		if currentIsMap && desiredIsMap {
			projectAIGatewayAuthStrategyConfigForComparison(currentMap, desiredMap)
		}
	}
}

func mergeAIGatewayAuthStrategyConfig(current, desired map[string]any) map[string]any {
	merged := normalizeAuthStrategyConfigForCompare(current)
	if merged == nil {
		merged = make(map[string]any)
	}
	mergeAIGatewayAuthStrategyConfigValues(merged, normalizeAuthStrategyConfigForCompare(desired))
	return merged
}

func mergeAIGatewayAuthStrategyConfigValues(current, desired map[string]any) {
	for key, desiredValue := range desired {
		currentMap, currentIsMap := current[key].(map[string]any)
		desiredMap, desiredIsMap := desiredValue.(map[string]any)
		if currentIsMap && desiredIsMap {
			mergeAIGatewayAuthStrategyConfigValues(currentMap, desiredMap)
			continue
		}
		current[key] = desiredValue
	}
}

func normalizeAuthStrategyConfigForCompare(config map[string]any) map[string]any {
	if config == nil {
		return nil
	}
	data, err := json.Marshal(config)
	if err != nil {
		return config
	}
	var normalized map[string]any
	if err := json.Unmarshal(data, &normalized); err != nil {
		return config
	}
	return normalized
}

func scrubAIGatewayAuthStrategySecretFields(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, val := range typed {
			if isAIGatewayAuthStrategySecretField(key) {
				if references, ok := projectPublicVaultReferences(val); ok {
					result[key] = references
				}
				continue
			}
			result[key] = scrubAIGatewayAuthStrategySecretFields(val)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i := range typed {
			result[i] = scrubAIGatewayAuthStrategySecretFields(typed[i])
		}
		return result
	default:
		return value
	}
}

func isAIGatewayAuthStrategySecretField(key string) bool {
	switch strings.ToLower(key) {
	case FieldClientSecret:
		return true
	default:
		return false
	}
}
