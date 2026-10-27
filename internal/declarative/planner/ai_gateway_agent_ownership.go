package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) reconcileOwnedAIGatewayAgents(
	ctx context.Context,
	gatewayRef, gatewayName, gatewayID string,
	desired []resources.AIGatewayAgentResource,
	current []state.AIGatewayAgent,
	policyCreateDepsByRefOrName map[string]string,
	plan *Plan,
) error {
	if p.resourceCache != nil && p.resourceCache.aiGatewayAgentsPlanned[gatewayID] {
		return nil
	}
	rs := p.resources
	if rs == nil {
		rs = &resources.ResourceSet{AIGatewayAgents: desired}
		for i := range rs.AIGatewayAgents {
			rs.AIGatewayAgents[i].AIGateway = gatewayRef
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
			for namespace := range rs.SyncScope.OwnedChildNamespaces(resources.ResourceTypeAIGatewayAgent, alias) {
				namespaces[namespace] = true
			}
		}
	}
	byName := make(map[string]state.AIGatewayAgent)
	for _, agent := range current {
		byName[agent.Name] = agent
	}
	declared := make(map[string]string)
	for _, agent := range rs.AIGatewayAgents {
		if !aliases[resources.NormalizeResourceRef(agent.AIGateway)] {
			continue
		}
		if firstRef, exists := declared[agent.Name]; exists {
			return fmt.Errorf("duplicate desired AI Gateway agent name %q on gateway %q (refs %q and %q)",
				agent.Name, gatewayRef, firstRef, agent.Ref)
		}
		declared[agent.Name] = agent.Ref
	}
	retained := make(map[string]bool)
	for _, agent := range rs.AIGatewayAgents {
		if !aliases[resources.NormalizeResourceRef(agent.AIGateway)] {
			continue
		}
		namespace := resources.GetNamespace(agent.Kongctl)
		namespaces[namespace] = true
		observed, exists := byName[agent.Name]
		if !exists {
			if plan.Metadata.Mode != PlanModeDelete {
				p.planAIGatewayAgentCreate(
					namespace,
					gatewayRef,
					gatewayName,
					gatewayID,
					agent,
					aiGatewayAgentPolicyCreateDependencies(agent, policyCreateDepsByRefOrName),
					plan,
				)
			}
			continue
		}
		if err := resources.ValidateChildOwnership("AI Gateway agent", observed.ManagedBy, namespace); err != nil {
			return fmt.Errorf("agent %q: %w", agent.Name, err)
		}
		full, err := p.client.GetAIGatewayAgent(ctx, gatewayID, observed.ID)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway Agent %s: %w", observed.ID, err)
		}
		if full == nil {
			return fmt.Errorf("AI Gateway agent %q disappeared during planning; retry planning", agent.Name)
		}
		if err := resources.ValidateChildOwnership("AI Gateway agent", full.ManagedBy, namespace); err != nil {
			return err
		}
		p.recordMatchedIdentity(&agent, full)
		if original := rs.GetAIGatewayAgentByRef(agent.Ref); original != nil {
			original.SetKonnectID(full.ID)
		}
		retained[full.ID] = true
		protected := resources.IsChildProtected(full.ManagedBy)
		if plan.Metadata.Mode == PlanModeDelete {
			if protected {
				return fmt.Errorf("AI Gateway agent %q is protected and cannot be deleted", agent.Name)
			}
			p.planAIGatewayAgentDelete(namespace, gatewayRef, gatewayID, full.ID, full.Name, plan)
			continue
		}
		agent.ManagedBy = resources.BuildChildManagedBy(
			full.ManagedBy,
			agent.ManagedBy,
			namespace,
			agent.Protected(),
			build.Version(ctx),
		)
		if err := resources.ValidateChildManagedBy(agent.ManagedBy, false); err != nil {
			return err
		}
		update, fields, changed, err := p.shouldUpdateAIGatewayAgent(*full, agent)
		if err != nil {
			return err
		}
		if !update {
			continue
		}
		if protected {
			if agent.Protected() || len(changed) != 1 || changed[FieldManagedBy].New == nil ||
				!resources.ChildUnprotectionMetadataOnly(full.ManagedBy, agent.ManagedBy) {
				return fmt.Errorf(
					"AI Gateway agent %q is protected; unprotect it in a separate update",
					agent.Name,
				)
			}
			// Execution preserves the freshly observed payload for metadata-only PUTs.
			// Do not copy arbitrary observed config (which can contain credentials) into the plan.
		}
		// Leave omitted labels out of the plan so execution preserves the latest public labels.
		if agent.Labels == nil {
			delete(fields, FieldLabels)
		}
		p.planAIGatewayAgentUpdate(
			namespace,
			gatewayRef,
			gatewayID,
			full.ID,
			agent,
			fields,
			changed,
			aiGatewayAgentPolicyCreateDependencies(agent, policyCreateDepsByRefOrName),
			plan,
		)
		if protected != agent.Protected() {
			plan.Changes[len(plan.Changes)-1].Protection = ProtectionChange{Old: protected, New: agent.Protected()}
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
		for _, agent := range current {
			namespace := agent.ManagedBy[resources.ChildNamespaceKey]
			if retained[agent.ID] || namespace == "" || !namespaces[namespace] {
				continue
			}
			if err := resources.ValidateChildOwnership("AI Gateway agent", agent.ManagedBy, namespace); err != nil {
				return err
			}
			if resources.IsChildProtected(agent.ManagedBy) {
				return fmt.Errorf("AI Gateway agent %q is protected and cannot be deleted", agent.Name)
			}
			p.planAIGatewayAgentDelete(namespace, gatewayRef, gatewayID, agent.ID, agent.Name, plan)
		}
	}
	if p.resourceCache != nil {
		p.resourceCache.aiGatewayAgentsPlanned[gatewayID] = true
	}
	return nil
}
