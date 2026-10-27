package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) reconcileOwnedAIGatewayProviders(
	ctx context.Context, gatewayRef, gatewayName, gatewayID string,
	desired []resources.AIGatewayProviderResource, current []state.AIGatewayProvider, plan *Plan,
) error {
	if p.resourceCache != nil && p.resourceCache.aiGatewayProvidersPlanned[gatewayID] {
		return nil
	}
	rs := p.resources
	if rs == nil {
		rs = &resources.ResourceSet{AIGatewayProviders: desired}
		for i := range rs.AIGatewayProviders {
			rs.AIGatewayProviders[i].AIGateway = gatewayRef
		}
	}
	aliases := map[string]bool{gatewayRef: true}
	for _, gateway := range rs.AIGateways {
		if gateway.GetKonnectID() == gatewayID {
			aliases[gateway.Ref] = true
		}
	}
	namespaces := make(map[string]bool)
	if rs.SyncScope != nil {
		for alias := range aliases {
			for namespace := range rs.SyncScope.OwnedChildNamespaces(resources.ResourceTypeAIGatewayProvider, alias) {
				namespaces[namespace] = true
			}
		}
	}
	byName := make(map[string]state.AIGatewayProvider)
	for _, provider := range current {
		byName[provider.Name] = provider
	}
	declared := make(map[string]bool)
	for _, provider := range rs.AIGatewayProviders {
		if !aliases[resources.NormalizeResourceRef(provider.AIGateway)] {
			continue
		}
		if declared[provider.Name] {
			return fmt.Errorf("duplicate desired AI Gateway provider name %q on gateway %q", provider.Name, gatewayRef)
		}
		declared[provider.Name] = true
	}
	retained := make(map[string]bool)
	for _, provider := range rs.AIGatewayProviders {
		if !aliases[resources.NormalizeResourceRef(provider.AIGateway)] {
			continue
		}
		namespace := resources.GetNamespace(provider.Kongctl)
		namespaces[namespace] = true
		observed, exists := byName[provider.Name]
		if !exists {
			if plan.Metadata.Mode != PlanModeDelete {
				p.planAIGatewayProviderCreate(namespace, gatewayRef, gatewayName, gatewayID, provider, nil, plan)
			}
			continue
		}
		if err := resources.ValidateChildOwnership("AI Gateway provider", observed.ManagedBy, namespace); err != nil {
			return fmt.Errorf("provider %q: %w", provider.Name, err)
		}
		full, err := p.client.GetAIGatewayProvider(ctx, gatewayID, observed.ID)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway Model Provider %s: %w", observed.ID, err)
		}
		if full == nil {
			return fmt.Errorf("AI Gateway provider %q disappeared during planning; retry planning", provider.Name)
		}
		if err := resources.ValidateChildOwnership("AI Gateway provider", full.ManagedBy, namespace); err != nil {
			return err
		}
		p.recordMatchedIdentity(&provider, full)
		retained[full.ID] = true
		protected := full.ManagedBy[resources.ChildProtectedKey] == "true"
		if plan.Metadata.Mode == PlanModeDelete {
			if protected {
				return fmt.Errorf("AI Gateway provider %q is protected and cannot be deleted", provider.Name)
			}
			p.planAIGatewayProviderDelete(namespace, gatewayRef, gatewayID, full.ID, full.Name, plan)
			continue
		}
		provider.ManagedBy = resources.BuildChildManagedBy(
			namespace,
			provider.Protected(),
			build.Version(ctx),
		)
		if err := resources.ValidateChildManagedBy(provider.ManagedBy, false); err != nil {
			return err
		}
		update, fields, changed, err := shouldUpdateAIGatewayProvider(*full, provider, p.resources)
		if err != nil {
			return err
		}
		if !update {
			continue
		}
		if protected {
			if provider.Protected() || len(changed) != 1 || changed[FieldManagedBy].New == nil ||
				!resources.ChildUnprotectionMetadataOnly(full.ManagedBy, provider.ManagedBy) {
				return fmt.Errorf(
					"AI Gateway provider %q is protected; unprotect it in a separate update",
					provider.Name,
				)
			}
			// Use the observed public payload for a protection-only PUT; secret values remain omitted.
			fields = providerObservedFields(*full)
			fields[FieldManagedBy] = provider.ManagedBy
		}
		// Leave omitted labels out of the plan so execution preserves the latest public labels.
		if provider.Labels == nil {
			delete(fields, FieldLabels)
		}
		p.planAIGatewayProviderUpdate(namespace, gatewayRef, gatewayID, full.ID, provider, fields, changed, plan)
		if protected != provider.Protected() {
			plan.Changes[len(plan.Changes)-1].Protection = ProtectionChange{Old: protected, New: provider.Protected()}
		}
	}
	if plan.Metadata.Mode == PlanModeSync {
		// Directly constructed ResourceSets have no source capture, matching model fallback semantics.
		if len(namespaces) == 0 && rs.SyncScope == nil {
			for _, namespace := range rs.DefaultNamespaces {
				namespaces[namespace] = true
			}
			if len(namespaces) == 0 {
				namespaces[DefaultNamespace] = true
			}
		}
		for _, provider := range current {
			namespace := provider.ManagedBy[resources.ChildNamespaceKey]
			if retained[provider.ID] || namespace == "" || !namespaces[namespace] {
				continue
			}
			if err := resources.ValidateChildOwnership("AI Gateway provider", provider.ManagedBy, namespace); err != nil {
				return err
			}
			if provider.ManagedBy[resources.ChildProtectedKey] == "true" {
				return fmt.Errorf("AI Gateway provider %q is protected and cannot be deleted", provider.Name)
			}
			p.planAIGatewayProviderDelete(namespace, gatewayRef, gatewayID, provider.ID, provider.Name, plan)
		}
	}
	if p.resourceCache != nil {
		p.resourceCache.aiGatewayProvidersPlanned[gatewayID] = true
	}
	return nil
}

func providerObservedFields(provider state.AIGatewayProvider) map[string]any {
	return map[string]any{
		FieldName: provider.Name, FieldType: provider.Type, FieldDisplayName: provider.DisplayName,
		FieldConfig: provider.Config, FieldLabels: provider.Labels, FieldManagedBy: provider.ManagedBy,
	}
}
