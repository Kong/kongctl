package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) reconcileOwnedAIGatewayAuthStrategies(
	ctx context.Context, gatewayRef, gatewayName, gatewayID string,
	desired []resources.AIGatewayAuthStrategyResource, current []state.AIGatewayAuthStrategy, plan *Plan,
) error {
	if p.resourceCache != nil && p.resourceCache.aiGatewayAuthStrategiesPlanned[gatewayID] {
		return nil
	}
	rs := p.resources
	if rs == nil {
		rs = &resources.ResourceSet{AIGatewayAuthStrategies: desired}
		for i := range rs.AIGatewayAuthStrategies {
			rs.AIGatewayAuthStrategies[i].AIGateway = gatewayRef
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
			for namespace := range rs.SyncScope.OwnedChildNamespaces(resources.ResourceTypeAIGatewayAuthStrategy, alias) {
				namespaces[namespace] = true
			}
		}
	}
	byName := make(map[string]state.AIGatewayAuthStrategy)
	for _, strategy := range current {
		byName[strategy.Name] = strategy
	}
	declared := make(map[string]bool)
	for _, strategy := range rs.AIGatewayAuthStrategies {
		if !aliases[resources.NormalizeResourceRef(strategy.AIGateway)] {
			continue
		}
		if declared[strategy.Name] {
			return fmt.Errorf("duplicate desired AI Gateway auth strategy name %q on gateway %q", strategy.Name, gatewayRef)
		}
		declared[strategy.Name] = true
	}
	retained := make(map[string]bool)
	for _, strategy := range rs.AIGatewayAuthStrategies {
		if !aliases[resources.NormalizeResourceRef(strategy.AIGateway)] {
			continue
		}
		namespace := resources.GetNamespace(strategy.Kongctl)
		namespaces[namespace] = true
		observed, exists := byName[strategy.Name]
		if !exists {
			if plan.Metadata.Mode != PlanModeDelete {
				p.planAIGatewayAuthStrategyCreate(namespace, gatewayRef, gatewayName, gatewayID, strategy, nil, plan)
			}
			continue
		}
		if err := resources.ValidateChildOwnership("AI Gateway auth strategy", observed.ManagedBy, namespace); err != nil {
			return fmt.Errorf("strategy %q: %w", strategy.Name, err)
		}
		full, err := p.client.GetAIGatewayAuthStrategy(ctx, gatewayID, observed.ID)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway Auth Strategy %s: %w", observed.ID, err)
		}
		if full == nil {
			return fmt.Errorf("AI Gateway auth strategy %q disappeared during planning; retry planning", strategy.Name)
		}
		if err := resources.ValidateChildOwnership("AI Gateway auth strategy", full.ManagedBy, namespace); err != nil {
			return err
		}
		p.recordMatchedIdentity(&strategy, full)
		retained[full.ID] = true
		protected := full.ManagedBy[resources.ChildProtectedKey] == "true"
		if plan.Metadata.Mode == PlanModeDelete {
			if protected {
				return fmt.Errorf("AI Gateway auth strategy %q is protected and cannot be deleted", strategy.Name)
			}
			p.planAIGatewayAuthStrategyDelete(namespace, gatewayRef, gatewayID, full.ID, full.Name, plan)
			continue
		}
		strategy.ManagedBy = resources.BuildChildManagedBy(
			namespace,
			strategy.Protected(),
			build.Version(ctx),
		)
		if err := resources.ValidateChildManagedBy(strategy.ManagedBy, false); err != nil {
			return err
		}
		update, fields, changed, err := shouldUpdateAIGatewayAuthStrategy(*full, strategy)
		if err != nil {
			return err
		}
		if !update {
			continue
		}
		if protected {
			if strategy.Protected() || len(changed) != 1 || changed[FieldManagedBy].New == nil ||
				!resources.ChildUnprotectionMetadataOnly(full.ManagedBy, strategy.ManagedBy) {
				return fmt.Errorf(
					"AI Gateway auth strategy %q is protected; unprotect it in a separate update",
					strategy.Name,
				)
			}
			// Use the observed public payload for a protection-only PUT; secret values remain omitted.
			fields = authStrategyObservedFields(*full)
			fields[FieldManagedBy] = strategy.ManagedBy
		}
		// Leave omitted labels out of the plan so execution preserves the latest public labels.
		if strategy.Labels == nil {
			delete(fields, FieldLabels)
		}
		p.planAIGatewayAuthStrategyUpdate(namespace, gatewayRef, gatewayID, full.ID, strategy, fields, changed, plan)
		if protected != strategy.Protected() {
			plan.Changes[len(plan.Changes)-1].Protection = ProtectionChange{Old: protected, New: strategy.Protected()}
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
		for _, strategy := range current {
			namespace := strategy.ManagedBy[resources.ChildNamespaceKey]
			if retained[strategy.ID] || namespace == "" || !namespaces[namespace] {
				continue
			}
			if err := resources.ValidateChildOwnership("AI Gateway auth strategy", strategy.ManagedBy, namespace); err != nil {
				return err
			}
			if strategy.ManagedBy[resources.ChildProtectedKey] == "true" {
				return fmt.Errorf("AI Gateway auth strategy %q is protected and cannot be deleted", strategy.Name)
			}
			p.planAIGatewayAuthStrategyDelete(namespace, gatewayRef, gatewayID, strategy.ID, strategy.Name, plan)
		}
	}
	if p.resourceCache != nil {
		p.resourceCache.aiGatewayAuthStrategiesPlanned[gatewayID] = true
	}
	return nil
}

func authStrategyObservedFields(strategy state.AIGatewayAuthStrategy) map[string]any {
	return map[string]any{
		FieldName: strategy.Name, FieldType: strategy.Type, FieldDisplayName: strategy.DisplayName,
		FieldConfig: strategy.Config, FieldLabels: strategy.Labels, FieldManagedBy: strategy.ManagedBy,
	}
}
