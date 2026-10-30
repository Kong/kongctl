package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) reconcileOwnedAIGatewayPolicies(
	ctx context.Context, gatewayRef, gatewayName, gatewayID string,
	desired []resources.AIGatewayPolicyResource, current []state.AIGatewayPolicy, plan *Plan,
) error {
	if p.resourceCache != nil && p.resourceCache.aiGatewayPoliciesPlanned[gatewayID] {
		return nil
	}
	rs := p.resources
	if rs == nil {
		rs = &resources.ResourceSet{AIGatewayPolicies: desired}
		for i := range rs.AIGatewayPolicies {
			rs.AIGatewayPolicies[i].AIGateway = gatewayRef
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
			for namespace := range rs.SyncScope.OwnedChildNamespaces(resources.ResourceTypeAIGatewayPolicy, alias) {
				namespaces[namespace] = true
			}
		}
	}
	byName := make(map[string]state.AIGatewayPolicy)
	for _, policy := range current {
		byName[policy.Name] = policy
	}
	declared := make(map[string]string)
	for _, policy := range rs.AIGatewayPolicies {
		if !aliases[resources.NormalizeResourceRef(policy.AIGateway)] {
			continue
		}
		if firstRef, exists := declared[policy.Name]; exists {
			return fmt.Errorf("duplicate desired AI Gateway policy name %q on gateway %q (refs %q and %q)",
				policy.Name, gatewayRef, firstRef, policy.Ref)
		}
		declared[policy.Name] = policy.Ref
	}
	retained := make(map[string]bool)
	for _, policy := range rs.AIGatewayPolicies {
		if !aliases[resources.NormalizeResourceRef(policy.AIGateway)] {
			continue
		}
		namespace := resources.GetNamespace(policy.Kongctl)
		namespaces[namespace] = true
		observed, exists := byName[policy.Name]
		if !exists {
			if plan.Metadata.Mode != PlanModeDelete {
				p.planAIGatewayPolicyCreate(namespace, gatewayRef, gatewayName, gatewayID, policy, nil, plan)
			}
			continue
		}
		if err := resources.ValidateChildOwnership("AI Gateway policy", observed.ManagedBy, namespace); err != nil {
			return fmt.Errorf("policy %q: %w", policy.Name, err)
		}
		full, err := p.client.GetAIGatewayPolicy(ctx, gatewayID, observed.ID)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway Policy %s: %w", observed.ID, err)
		}
		if full == nil {
			return fmt.Errorf("AI Gateway policy %q disappeared during planning; retry planning", policy.Name)
		}
		if err := resources.ValidateChildOwnership("AI Gateway policy", full.ManagedBy, namespace); err != nil {
			return err
		}
		p.recordMatchedIdentity(&policy, full)
		if original := rs.GetAIGatewayPolicyByRef(policy.Ref); original != nil {
			original.SetKonnectID(full.ID)
		}
		retained[full.ID] = true
		protected := resources.IsChildProtected(full.ManagedBy)
		if plan.Metadata.Mode == PlanModeDelete {
			if protected {
				return fmt.Errorf("AI Gateway policy %q is protected and cannot be deleted", policy.Name)
			}
			p.planAIGatewayPolicyDelete(namespace, gatewayRef, gatewayID, full.ID, full.Name, plan)
			continue
		}
		policy.ManagedBy = resources.BuildChildManagedBy(
			namespace,
			policy.Protected(),
			build.Version(ctx),
		)
		if err := resources.ValidateChildManagedBy(policy.ManagedBy, false); err != nil {
			return err
		}
		update, fields, changed, err := shouldUpdateAIGatewayPolicy(*full, policy, p.resources)
		if err != nil {
			return err
		}
		if !update {
			continue
		}
		if protected {
			if policy.Protected() || len(changed) != 1 || changed[FieldManagedBy].New == nil ||
				!resources.ChildUnprotectionMetadataOnly(full.ManagedBy, policy.ManagedBy) {
				return fmt.Errorf(
					"AI Gateway policy %q is protected; unprotect it in a separate update",
					policy.Name,
				)
			}
			// Execution preserves the freshly observed payload for metadata-only PUTs.
			// Do not copy arbitrary observed config (which can contain credentials) into the plan.
		}
		// Leave omitted labels out of the plan so execution preserves the latest public labels.
		if policy.Labels == nil {
			delete(fields, FieldLabels)
		}
		p.planAIGatewayPolicyUpdate(namespace, gatewayRef, gatewayID, full.ID, policy, fields, changed, plan)
		if protected != policy.Protected() {
			plan.Changes[len(plan.Changes)-1].Protection = ProtectionChange{Old: protected, New: policy.Protected()}
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
		for _, policy := range current {
			namespace := policy.ManagedBy[resources.ChildNamespaceKey]
			if retained[policy.ID] || namespace == "" || !namespaces[namespace] {
				continue
			}
			if err := resources.ValidateChildOwnership("AI Gateway policy", policy.ManagedBy, namespace); err != nil {
				return err
			}
			if resources.IsChildProtected(policy.ManagedBy) {
				return fmt.Errorf("AI Gateway policy %q is protected and cannot be deleted", policy.Name)
			}
			p.planAIGatewayPolicyDelete(namespace, gatewayRef, gatewayID, policy.ID, policy.Name, plan)
		}
	}
	if p.resourceCache != nil {
		p.resourceCache.aiGatewayPoliciesPlanned[gatewayID] = true
	}
	return nil
}
