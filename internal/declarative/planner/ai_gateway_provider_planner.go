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
)

func (p *Planner) planAIGatewayProviderChanges(
	ctx context.Context,
	_ *Config,
	namespace string,
	gatewayName string,
	gatewayID string,
	gatewayRef string,
	gatewayChangeID string,
	desired []resources.AIGatewayProviderResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning AI Gateway Provider changes",
		slog.String("gateway_name", gatewayName),
		slog.String("gateway_id", gatewayID),
		slog.String("gateway_ref", gatewayRef),
		slog.String("gateway_change_id", gatewayChangeID),
		slog.Int("desired_count", len(desired)),
		slog.String("namespace", namespace),
	)

	if gatewayID == "" {
		p.planAIGatewayProviderCreatesForNewGateway(namespace, gatewayRef, gatewayName, gatewayChangeID, desired, plan)
		return nil
	}

	currentProviders, err := p.client.ListAIGatewayProviders(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway Providers for gateway %s: %w", gatewayID, err)
	}

	currentByName := make(map[string]state.AIGatewayProvider)
	for _, provider := range currentProviders {
		currentByName[provider.Name] = provider
	}

	desiredNames := make(map[string]bool)
	for _, desiredProvider := range desired {
		desiredNames[desiredProvider.Name] = true

		current, exists := currentByName[desiredProvider.Name]
		if !exists {
			p.planAIGatewayProviderCreate(
				namespace, gatewayRef, gatewayName, gatewayID, desiredProvider, nil, plan,
			)
			continue
		}

		fullProvider, err := p.client.GetAIGatewayProvider(ctx, gatewayID, current.ID)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway Provider %s: %w", current.ID, err)
		}
		if fullProvider == nil {
			p.planAIGatewayProviderCreate(
				namespace, gatewayRef, gatewayName, gatewayID, desiredProvider, nil, plan,
			)
			continue
		}

		needsUpdate, updateFields, changedFields := shouldUpdateAIGatewayProvider(*fullProvider, desiredProvider)
		if !needsUpdate {
			continue
		}

		if errMsg, ok := updateFields[FieldError].(string); ok {
			return fmt.Errorf("%s", errMsg)
		}

		p.planAIGatewayProviderUpdate(
			namespace, gatewayRef, gatewayID, current.ID, desiredProvider, updateFields, changedFields, plan,
		)
	}

	if plan.Metadata.Mode == PlanModeSync {
		for name, current := range currentByName {
			if desiredNames[name] {
				continue
			}

			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			if err := p.validateProtection(ResourceTypeAIGatewayProvider, name, isProtected, ActionDelete); err != nil {
				return err
			}
			p.planAIGatewayProviderDelete(gatewayRef, gatewayID, current.ID, name, plan)
		}
	}

	return nil
}

func (p *Planner) planAIGatewayProviderCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	providers []resources.AIGatewayProviderResource,
	plan *Plan,
) {
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}

	for _, provider := range providers {
		p.planAIGatewayProviderCreate(namespace, gatewayRef, gatewayName, "", provider, dependsOn, plan)
	}
}

func (p *Planner) planAIGatewayProviderCreate(
	namespace string,
	gatewayRef string,
	_ string,
	gatewayID string,
	provider resources.AIGatewayProviderResource,
	dependsOn []string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayProvider, provider.Ref),
		ResourceType: ResourceTypeAIGatewayProvider,
		ResourceRef:  provider.Ref,
		Action:       ActionCreate,
		Fields:       extractAIGatewayProviderFields(provider),
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

func (p *Planner) planAIGatewayProviderUpdate(
	namespace string,
	gatewayRef string,
	gatewayID string,
	providerID string,
	provider resources.AIGatewayProviderResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	if len(updateFields) == 0 {
		return
	}

	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeAIGatewayProvider, provider.Ref),
		ResourceType:  ResourceTypeAIGatewayProvider,
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

func (p *Planner) planAIGatewayProviderDelete(
	gatewayRef string,
	gatewayID string,
	providerID string,
	providerName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGatewayProvider, providerName),
		ResourceType: ResourceTypeAIGatewayProvider,
		ResourceRef:  providerName,
		ResourceID:   providerID,
		Action:       ActionDelete,
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

func shouldUpdateAIGatewayProvider(
	current state.AIGatewayProvider,
	desired resources.AIGatewayProviderResource,
) (bool, map[string]any, map[string]FieldChange) {
	updateFields := make(map[string]any)
	changedFields := make(map[string]FieldChange)

	if current.Type != desired.Type {
		updateFields[FieldError] = fmt.Sprintf(
			"changing AI Gateway Provider type from %s to %s is not supported. Please delete and recreate the provider",
			current.Type, desired.Type,
		)
		return true, updateFields, changedFields
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

	if desired.Config != nil && aiGatewayProviderConfigChanged(current.Config, desired.Config) {
		changedFields[FieldConfig] = FieldChange{Old: current.Config, New: desired.Config}
	}

	if len(changedFields) == 0 {
		return false, updateFields, changedFields
	}

	updateFields = extractAIGatewayProviderFields(desired)
	return true, updateFields, changedFields
}

func extractAIGatewayProviderFields(provider resources.AIGatewayProviderResource) map[string]any {
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

func aiGatewayProviderConfigChanged(current, desired map[string]any) bool {
	currentComparable := scrubAIGatewayProviderSecretFields(normalizeProviderConfigForCompare(current))
	desiredComparable := scrubAIGatewayProviderSecretFields(normalizeProviderConfigForCompare(desired))
	return !reflect.DeepEqual(currentComparable, desiredComparable)
}

func normalizeProviderConfigForCompare(config map[string]any) map[string]any {
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

func scrubAIGatewayProviderSecretFields(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, val := range typed {
			if isAIGatewayProviderSecretField(key) {
				continue
			}
			result[key] = scrubAIGatewayProviderSecretFields(val)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i := range typed {
			result[i] = scrubAIGatewayProviderSecretFields(typed[i])
		}
		return result
	default:
		return value
	}
}

func isAIGatewayProviderSecretField(key string) bool {
	switch strings.ToLower(key) {
	case "value", "client_secret", "secret_access_key", "service_account_json":
		return true
	default:
		return false
	}
}
