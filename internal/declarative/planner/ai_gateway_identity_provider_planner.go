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

func (p *Planner) planAIGatewayIdentityProviderChanges(
	ctx context.Context,
	_ *Config,
	namespace string,
	gatewayName string,
	gatewayID string,
	gatewayRef string,
	gatewayChangeID string,
	desired []resources.AIGatewayIdentityProviderResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning AI Gateway Identity Provider changes",
		slog.String("gateway_name", gatewayName),
		slog.String("gateway_id", gatewayID),
		slog.String("gateway_ref", gatewayRef),
		slog.String("gateway_change_id", gatewayChangeID),
		slog.Int("desired_count", len(desired)),
		slog.String("namespace", namespace),
	)

	if gatewayID == "" {
		p.planAIGatewayIdentityProviderCreatesForNewGateway(
			namespace,
			gatewayRef,
			gatewayName,
			gatewayChangeID,
			desired,
			plan,
		)
		return nil
	}

	currentProviders, err := p.client.ListAIGatewayIdentityProviders(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway Identity Providers for gateway %s: %w", gatewayID, err)
	}

	currentByID, currentByName := indexAIGatewayIdentityProviders(currentProviders)

	desiredKeys := make(map[string]bool)
	for _, desiredProvider := range desired {
		desiredKeys[desiredProvider.Name] = true
		if id := aiGatewayIdentityProviderDesiredID(desiredProvider); id != "" {
			desiredKeys[id] = true
		}

		current, exists := matchCurrentAIGatewayIdentityProvider(desiredProvider, currentByID, currentByName)
		if !exists {
			p.planAIGatewayIdentityProviderCreate(
				namespace, gatewayRef, gatewayName, gatewayID, desiredProvider, nil, plan,
			)
			continue
		}

		fullProvider, err := p.client.GetAIGatewayIdentityProvider(ctx, gatewayID, current.ID)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway Identity Provider %s: %w", current.ID, err)
		}
		if fullProvider == nil {
			p.planAIGatewayIdentityProviderCreate(
				namespace, gatewayRef, gatewayName, gatewayID, desiredProvider, nil, plan,
			)
			continue
		}

		needsUpdate, updateFields, changedFields, err := shouldUpdateAIGatewayIdentityProvider(*fullProvider, desiredProvider)
		if err != nil {
			return err
		}
		if !needsUpdate {
			continue
		}

		p.planAIGatewayIdentityProviderUpdate(
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
				ResourceTypeAIGatewayIdentityProvider,
				current.Name,
				isProtected,
				ActionDelete,
			); err != nil {
				return err
			}
			p.planAIGatewayIdentityProviderDelete(namespace, gatewayRef, gatewayID, current.ID, current.Name, plan)
		}
	}

	return nil
}

func indexAIGatewayIdentityProviders(
	providers []state.AIGatewayIdentityProvider,
) (map[string]state.AIGatewayIdentityProvider, map[string]state.AIGatewayIdentityProvider) {
	byID := make(map[string]state.AIGatewayIdentityProvider)
	byName := make(map[string]state.AIGatewayIdentityProvider)
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

func matchCurrentAIGatewayIdentityProvider(
	desired resources.AIGatewayIdentityProviderResource,
	currentByID map[string]state.AIGatewayIdentityProvider,
	currentByName map[string]state.AIGatewayIdentityProvider,
) (state.AIGatewayIdentityProvider, bool) {
	if id := aiGatewayIdentityProviderDesiredID(desired); id != "" {
		current, exists := currentByID[id]
		return current, exists
	}
	current, exists := currentByName[desired.Name]
	return current, exists
}

func aiGatewayIdentityProviderDesiredID(desired resources.AIGatewayIdentityProviderResource) string {
	if id := desired.GetKonnectID(); id != "" {
		return id
	}
	if util.IsValidUUID(desired.Ref) {
		return desired.Ref
	}
	return ""
}

func (p *Planner) planAIGatewayIdentityProviderCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	providers []resources.AIGatewayIdentityProviderResource,
	plan *Plan,
) {
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}

	for _, provider := range providers {
		p.planAIGatewayIdentityProviderCreate(namespace, gatewayRef, gatewayName, "", provider, dependsOn, plan)
	}
}

func (p *Planner) planAIGatewayIdentityProviderCreate(
	namespace string,
	gatewayRef string,
	_ string,
	gatewayID string,
	provider resources.AIGatewayIdentityProviderResource,
	dependsOn []string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayIdentityProvider, provider.Ref),
		ResourceType: ResourceTypeAIGatewayIdentityProvider,
		ResourceRef:  provider.Ref,
		Action:       ActionCreate,
		Fields:       extractAIGatewayIdentityProviderFields(provider),
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

func (p *Planner) planAIGatewayIdentityProviderUpdate(
	namespace string,
	gatewayRef string,
	gatewayID string,
	providerID string,
	provider resources.AIGatewayIdentityProviderResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	if len(updateFields) == 0 {
		return
	}

	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeAIGatewayIdentityProvider, provider.Ref),
		ResourceType:  ResourceTypeAIGatewayIdentityProvider,
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

func (p *Planner) planAIGatewayIdentityProviderDelete(
	namespace string,
	gatewayRef string,
	gatewayID string,
	providerID string,
	providerName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGatewayIdentityProvider, providerName),
		ResourceType: ResourceTypeAIGatewayIdentityProvider,
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

func shouldUpdateAIGatewayIdentityProvider(
	current state.AIGatewayIdentityProvider,
	desired resources.AIGatewayIdentityProviderResource,
) (bool, map[string]any, map[string]FieldChange, error) {
	updateFields := make(map[string]any)
	changedFields := make(map[string]FieldChange)

	if current.Type != desired.Type {
		return false, nil, nil, fmt.Errorf(
			"changing AI Gateway Identity Provider type from %s to %s is not supported. Please delete and recreate the provider",
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

	if desired.Config != nil && aiGatewayIdentityProviderConfigChanged(current.Config, desired.Config) {
		changedFields[FieldConfig] = FieldChange{
			Old: scrubAIGatewayIdentityProviderSecretFields(normalizeIdentityProviderConfigForCompare(current.Config)),
			New: scrubAIGatewayIdentityProviderSecretFields(normalizeIdentityProviderConfigForCompare(desired.Config)),
		}
	}

	if len(changedFields) == 0 {
		return false, updateFields, changedFields, nil
	}

	updateFields = extractAIGatewayIdentityProviderFields(desired)
	return true, updateFields, changedFields, nil
}

func extractAIGatewayIdentityProviderFields(provider resources.AIGatewayIdentityProviderResource) map[string]any {
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

func aiGatewayIdentityProviderConfigChanged(current, desired map[string]any) bool {
	currentComparable := normalizeIdentityProviderConfigForCompare(current)
	desiredComparable := normalizeIdentityProviderConfigForCompare(desired)
	projectAIGatewayIdentityProviderConfigForComparison(currentComparable, desiredComparable)
	currentComparable = scrubAIGatewayIdentityProviderSecretFields(currentComparable).(map[string]any)
	desiredComparable = scrubAIGatewayIdentityProviderSecretFields(desiredComparable).(map[string]any)
	return !reflect.DeepEqual(currentComparable, desiredComparable)
}

func projectAIGatewayIdentityProviderConfigForComparison(currentCompare map[string]any, desiredCompare map[string]any) {
	if currentCompare == nil || desiredCompare == nil {
		return
	}
	for key := range currentCompare {
		if _, declared := desiredCompare[key]; !declared {
			delete(currentCompare, key)
		}
	}
}

func normalizeIdentityProviderConfigForCompare(config map[string]any) map[string]any {
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

func scrubAIGatewayIdentityProviderSecretFields(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, val := range typed {
			if isAIGatewayIdentityProviderSecretField(key) {
				continue
			}
			result[key] = scrubAIGatewayIdentityProviderSecretFields(val)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i := range typed {
			result[i] = scrubAIGatewayIdentityProviderSecretFields(typed[i])
		}
		return result
	default:
		return value
	}
}

func isAIGatewayIdentityProviderSecretField(key string) bool {
	switch strings.ToLower(key) {
	case "client_secret":
		return true
	default:
		return false
	}
}
