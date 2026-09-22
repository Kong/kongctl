package planner

import (
	"context"

	"github.com/kong/kongctl/internal/declarative/resources"
)

// planAIGatewayChildren preserves dependency order for managed and external parents.
// A new managed gateway supplies its create change ID; existing parents supply their remote ID.
func (p *Planner) planAIGatewayChildren(
	ctx context.Context,
	plannerCtx *Config,
	namespace string,
	desiredGateway resources.AIGatewayResource,
	gatewayID string,
	gatewayChangeID string,
	plan *Plan,
) error {
	configStores := p.resources.GetAIGatewayConfigStoresForGateway(desiredGateway.Ref)
	if p.shouldPlanChild(
		plan,
		resources.ResourceTypeAIGateway,
		desiredGateway.Ref,
		resources.ResourceTypeAIGatewayConfigStore,
	) && (len(configStores) > 0 || plan.Metadata.Mode == PlanModeSync) {
		if err := p.planAIGatewayConfigStoreChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			gatewayChangeID,
			configStores,
			plan,
		); err != nil {
			return err
		}
	}

	vaults := p.resources.GetAIGatewayVaultsForGateway(desiredGateway.Ref)
	if p.shouldPlanChild(
		plan,
		resources.ResourceTypeAIGateway,
		desiredGateway.Ref,
		resources.ResourceTypeAIGatewayVault,
	) && (len(vaults) > 0 || plan.Metadata.Mode == PlanModeSync) {
		if err := p.planAIGatewayVaultChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			gatewayChangeID,
			vaults,
			plan,
		); err != nil {
			return err
		}
	}

	dataPlaneCertificates := p.resources.GetAIGatewayDataPlaneCertificatesForGateway(desiredGateway.Ref)
	if p.shouldPlanChild(
		plan,
		resources.ResourceTypeAIGateway,
		desiredGateway.Ref,
		resources.ResourceTypeAIGatewayDataPlaneCertificate,
	) && (len(dataPlaneCertificates) > 0 || plan.Metadata.Mode == PlanModeSync) {
		if err := p.planAIGatewayDataPlaneCertificateChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			gatewayChangeID,
			dataPlaneCertificates,
			plan,
		); err != nil {
			return err
		}
	}

	if err := p.planAIGatewayTLSChanges(
		ctx, namespace, desiredGateway.Ref, gatewayID, gatewayChangeID, plan,
	); err != nil {
		return err
	}

	providers := p.resources.GetAIGatewayProvidersForGateway(desiredGateway.Ref)
	if p.shouldPlanChild(
		plan,
		resources.ResourceTypeAIGateway,
		desiredGateway.Ref,
		resources.ResourceTypeAIGatewayProvider,
	) && (len(providers) > 0 || plan.Metadata.Mode == PlanModeSync) {
		if err := p.planAIGatewayProviderChanges(
			ctx, plannerCtx, namespace, desiredGateway.DisplayName, gatewayID, desiredGateway.Ref,
			gatewayChangeID, providers, plan,
		); err != nil {
			return err
		}
	}
	providerCreateDepsByName := aiGatewayProviderCreateDependencies(plan, namespace, desiredGateway.Ref)

	authStrategies := p.resources.GetAIGatewayAuthStrategiesForGateway(desiredGateway.Ref)
	if p.shouldPlanChild(
		plan,
		resources.ResourceTypeAIGateway,
		desiredGateway.Ref,
		resources.ResourceTypeAIGatewayAuthStrategy,
	) && (len(authStrategies) > 0 || plan.Metadata.Mode == PlanModeSync) {
		if err := p.planAIGatewayAuthStrategyChanges(
			ctx, plannerCtx, namespace, desiredGateway.DisplayName, gatewayID, desiredGateway.Ref,
			gatewayChangeID, authStrategies, plan,
		); err != nil {
			return err
		}
	}

	policies := p.resources.GetAIGatewayPoliciesForGateway(desiredGateway.Ref)
	if p.shouldPlanChild(
		plan,
		resources.ResourceTypeAIGateway,
		desiredGateway.Ref,
		resources.ResourceTypeAIGatewayPolicy,
	) && (len(policies) > 0 || plan.Metadata.Mode == PlanModeSync) {
		if err := p.planAIGatewayPolicyChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			gatewayChangeID,
			policies,
			plan,
		); err != nil {
			return err
		}
	}
	policyCreateDepsByName := aiGatewayPolicyCreateDependencies(plan, namespace, desiredGateway.Ref)

	agents := p.resources.GetAIGatewayAgentsForGateway(desiredGateway.Ref)
	if p.shouldPlanChild(
		plan,
		resources.ResourceTypeAIGateway,
		desiredGateway.Ref,
		resources.ResourceTypeAIGatewayAgent,
	) && (len(agents) > 0 || plan.Metadata.Mode == PlanModeSync) {
		if err := p.planAIGatewayAgentChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			gatewayChangeID,
			policyCreateDepsByName,
			agents,
			plan,
		); err != nil {
			return err
		}
	}

	consumers := p.resources.GetAIGatewayConsumersForGateway(desiredGateway.Ref)
	if p.shouldPlanChild(
		plan,
		resources.ResourceTypeAIGateway,
		desiredGateway.Ref,
		resources.ResourceTypeAIGatewayConsumer,
	) && (len(consumers) > 0 || plan.Metadata.Mode == PlanModeSync) {
		if err := p.planAIGatewayConsumerChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			gatewayChangeID,
			policyCreateDepsByName,
			consumers,
			plan,
		); err != nil {
			return err
		}
	}

	consumerGroups := p.resources.GetAIGatewayConsumerGroupsForGateway(desiredGateway.Ref)
	if p.shouldPlanChild(
		plan,
		resources.ResourceTypeAIGateway,
		desiredGateway.Ref,
		resources.ResourceTypeAIGatewayConsumerGroup,
	) && (len(consumerGroups) > 0 || plan.Metadata.Mode == PlanModeSync) {
		if err := p.planAIGatewayConsumerGroupChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			gatewayChangeID,
			policyCreateDepsByName,
			consumerGroups,
			plan,
		); err != nil {
			return err
		}
	}

	models := p.resources.GetAIGatewayModelsForGateway(desiredGateway.Ref)
	if p.shouldPlanChild(
		plan,
		resources.ResourceTypeAIGateway,
		desiredGateway.Ref,
		resources.ResourceTypeAIGatewayModel,
	) && (len(models) > 0 || plan.Metadata.Mode == PlanModeSync) {
		if err := p.planAIGatewayModelChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			gatewayChangeID,
			providerCreateDepsByName,
			policyCreateDepsByName,
			models,
			plan,
		); err != nil {
			return err
		}
	}

	mcpServers := p.resources.GetAIGatewayMCPServersForGateway(desiredGateway.Ref)
	if p.shouldPlanChild(
		plan,
		resources.ResourceTypeAIGateway,
		desiredGateway.Ref,
		resources.ResourceTypeAIGatewayMCPServer,
	) && (len(mcpServers) > 0 || plan.Metadata.Mode == PlanModeSync) {
		if err := p.planAIGatewayMCPServerChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			gatewayChangeID,
			policyCreateDepsByName,
			mcpServers,
			plan,
		); err != nil {
			return err
		}
	}

	return p.resolveAIGatewayProviderDeletes(ctx, namespace, desiredGateway.Ref, gatewayID, plan)
}
