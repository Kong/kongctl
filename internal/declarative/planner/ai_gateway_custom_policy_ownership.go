package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) reconcileOwnedAIGatewayCustomPolicies(
	ctx context.Context, gatewayRef, gatewayName, gatewayID string,
	desired []resources.AIGatewayCustomPolicyResource, current []state.AIGatewayCustomPolicy, plan *Plan,
) error {
	if p.resourceCache != nil && p.resourceCache.aiGatewayCustomPoliciesPlanned[gatewayID] {
		return nil
	}
	rs := p.resources
	if rs == nil {
		rs = &resources.ResourceSet{AIGatewayCustomPolicies: desired}
		for i := range rs.AIGatewayCustomPolicies {
			rs.AIGatewayCustomPolicies[i].AIGateway = gatewayRef
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
			for namespace := range rs.SyncScope.OwnedChildNamespaces(resources.ResourceTypeAIGatewayCustomPolicy, alias) {
				namespaces[namespace] = true
			}
		}
	}
	byName := make(map[string]state.AIGatewayCustomPolicy)
	for _, policy := range current {
		byName[resources.AIGatewayCustomPolicyName(policy.AIGatewayCustomPolicy)] = policy
	}
	declared := make(map[string]string)
	for _, policy := range rs.AIGatewayCustomPolicies {
		if !aliases[resources.NormalizeResourceRef(policy.AIGateway)] {
			continue
		}
		if firstRef, exists := declared[policy.Name]; exists {
			return fmt.Errorf("duplicate desired AI Gateway custom policy name %q on gateway %q (refs %q and %q)",
				policy.Name, gatewayRef, firstRef, policy.Ref)
		}
		declared[policy.Name] = policy.Ref
	}
	retained := make(map[string]bool)
	for _, policy := range rs.AIGatewayCustomPolicies {
		if !aliases[resources.NormalizeResourceRef(policy.AIGateway)] {
			continue
		}
		namespace := resources.GetNamespace(policy.Kongctl)
		namespaces[namespace] = true
		observed, exists := byName[policy.Name]
		if !exists {
			if plan.Metadata.Mode != PlanModeDelete {
				p.planAIGatewayCustomPolicyCreate(namespace, gatewayRef, gatewayName, gatewayID, policy, nil, plan)
			}
			continue
		}
		observedID := resources.AIGatewayCustomPolicyID(observed.AIGatewayCustomPolicy)
		observedMetadata := resources.AIGatewayCustomPolicyManagedBy(observed.AIGatewayCustomPolicy)
		if err := resources.ValidateChildOwnership("AI Gateway custom policy", observedMetadata, namespace); err != nil {
			return fmt.Errorf("policy %q: %w", policy.Name, err)
		}
		full, err := p.client.GetAIGatewayCustomPolicy(ctx, gatewayID, observedID)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway Custom Policy %s: %w", observedID, err)
		}
		if full == nil {
			return fmt.Errorf("AI Gateway custom policy %q disappeared during planning; retry planning", policy.Name)
		}
		fullID := resources.AIGatewayCustomPolicyID(full.AIGatewayCustomPolicy)
		fullName := resources.AIGatewayCustomPolicyName(full.AIGatewayCustomPolicy)
		fullMetadata := resources.AIGatewayCustomPolicyManagedBy(full.AIGatewayCustomPolicy)
		if err := resources.ValidateChildOwnership("AI Gateway custom policy", fullMetadata, namespace); err != nil {
			return err
		}
		p.recordMatchedIdentity(&policy, full)
		if original := rs.GetAIGatewayCustomPolicyByRef(policy.Ref); original != nil {
			original.SetKonnectID(fullID)
		}
		retained[fullID] = true
		protected := resources.IsChildProtected(fullMetadata)
		if plan.Metadata.Mode == PlanModeDelete {
			if protected {
				return fmt.Errorf("AI Gateway custom policy %q is protected and cannot be deleted", policy.Name)
			}
			p.planAIGatewayCustomPolicyDelete(namespace, gatewayRef, gatewayID, fullID, fullName, plan)
			continue
		}
		policy.ManagedBy = resources.BuildChildManagedBy(
			fullMetadata,
			policy.ManagedBy,
			namespace,
			policy.Protected(),
			build.Version(ctx),
		)
		if err := resources.ValidateChildManagedBy(policy.ManagedBy, false); err != nil {
			return err
		}
		update, fields, changed, err := shouldUpdateAIGatewayCustomPolicy(*full, policy)
		if err != nil {
			return err
		}
		if !update {
			continue
		}
		if protected {
			if policy.Protected() || len(changed) != 1 || changed[FieldManagedBy].New == nil ||
				!resources.ChildUnprotectionMetadataOnly(fullMetadata, policy.ManagedBy) {
				return fmt.Errorf(
					"AI Gateway custom policy %q is protected; unprotect it in a separate update",
					policy.Name,
				)
			}
		}
		// Leave omitted labels out of the plan so execution preserves the latest public labels.
		if policy.Labels == nil {
			delete(fields, FieldLabels)
		}
		p.planAIGatewayCustomPolicyUpdate(namespace, gatewayRef, gatewayID, fullID, policy, fields, changed, plan)
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
			id := resources.AIGatewayCustomPolicyID(policy.AIGatewayCustomPolicy)
			name := resources.AIGatewayCustomPolicyName(policy.AIGatewayCustomPolicy)
			metadata := resources.AIGatewayCustomPolicyManagedBy(policy.AIGatewayCustomPolicy)
			namespace := metadata[resources.ChildNamespaceKey]
			if retained[id] || namespace == "" || !namespaces[namespace] {
				continue
			}
			if err := resources.ValidateChildOwnership("AI Gateway custom policy", metadata, namespace); err != nil {
				return err
			}
			if resources.IsChildProtected(metadata) {
				return fmt.Errorf("AI Gateway custom policy %q is protected and cannot be deleted", name)
			}
			p.planAIGatewayCustomPolicyDelete(namespace, gatewayRef, gatewayID, id, name, plan)
		}
	}
	if p.resourceCache != nil {
		p.resourceCache.aiGatewayCustomPoliciesPlanned[gatewayID] = true
	}
	return nil
}
