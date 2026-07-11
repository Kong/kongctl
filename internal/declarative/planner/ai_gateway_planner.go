package planner

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/util"
)

type aiGatewayPlannerImpl struct {
	*BasePlanner
}

// NewAIGatewayPlanner creates a new AI Gateway planner.
func NewAIGatewayPlanner(base *BasePlanner) AIGatewayPlanner {
	return &aiGatewayPlannerImpl{BasePlanner: base}
}

func (p *aiGatewayPlannerImpl) PlannerComponent() string {
	return string(resources.ResourceTypeAIGateway)
}

// PlanChanges generates changes for AI Gateway resources.
func (p *aiGatewayPlannerImpl) PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error {
	namespace := plannerCtx.Namespace
	desired := p.GetDesiredAIGateways(namespace)

	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	return p.planner.planAIGatewayChanges(ctx, plannerCtx, desired, plan)
}

func (p *Planner) planAIGatewayChanges(
	ctx context.Context,
	plannerCtx *Config,
	desired []resources.AIGatewayResource,
	plan *Plan,
) error {
	namespace := plannerCtx.Namespace
	p.logger.Debug("planAIGatewayChanges called",
		slog.Int("desiredCount", len(desired)),
		slog.String("namespace", namespace))

	var currentGateways []state.AIGateway
	if namespace != resources.NamespaceExternal {
		var err error
		currentGateways, err = p.listManagedAIGateways(ctx, []string{namespace})
		if err != nil {
			if state.IsAPIClientError(err) {
				return nil
			}
			return fmt.Errorf("failed to list AI Gateways: %w", err)
		}
	}

	currentByID, currentByName, currentByDisplayName := indexAIGateways(currentGateways)

	if plan.Metadata.Mode == PlanModeDelete {
		var protectionErrors []error
		for _, desiredGateway := range desired {
			if desiredGateway.IsExternal() {
				continue
			}
			current, exists, err := matchCurrentAIGateway(desiredGateway, currentByID, currentByName, currentByDisplayName)
			if err != nil {
				return err
			}
			if !exists {
				plan.AddWarning("", fmt.Sprintf(
					"ai_gateway %q not found in Konnect, skipping delete", desiredGateway.DisplayName,
				))
				continue
			}

			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			if err := p.validateProtection(
				ResourceTypeAIGateway, desiredGateway.DisplayName, isProtected, ActionDelete,
			); err != nil {
				protectionErrors = append(protectionErrors, err)
			} else {
				p.planAIGatewayDelete(current, plan)
			}
		}

		if len(protectionErrors) > 0 {
			return aiGatewayProtectionError(protectionErrors)
		}
		return nil
	}

	var protectionErrors []error
	matchedCurrent := make(map[string]bool)

	for _, desiredGateway := range desired {
		if desiredGateway.IsExternal() {
			if err := p.planExternalAIGatewayChildren(ctx, plannerCtx, namespace, desiredGateway, plan); err != nil {
				return err
			}
			continue
		}

		current, exists, err := matchCurrentAIGateway(desiredGateway, currentByID, currentByName, currentByDisplayName)
		if err != nil {
			return err
		}
		gatewayID := ""
		gatewayChangeID := ""
		if !exists {
			gatewayChangeID = p.planAIGatewayCreate(desiredGateway, plan)
		} else {
			gatewayID = current.ID
			matchedCurrent[aiGatewayIdentity(current)] = true

			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			shouldProtect := desiredGateway.Kongctl != nil &&
				desiredGateway.Kongctl.Protected != nil &&
				*desiredGateway.Kongctl.Protected

			needsUpdate, updateFields, changedFields := p.shouldUpdateAIGateway(current, desiredGateway)
			if isProtected != shouldProtect {
				protectionChange := &ProtectionChange{Old: isProtected, New: shouldProtect}
				if err := p.validateProtectionWithChange(
					ResourceTypeAIGateway,
					desiredGateway.DisplayName,
					isProtected,
					ActionUpdate,
					protectionChange,
					needsUpdate,
				); err != nil {
					protectionErrors = append(protectionErrors, err)
				} else {
					p.planAIGatewayUpdate(current, desiredGateway, updateFields, changedFields, plan)
				}
			} else if needsUpdate {
				if err := p.validateProtection(
					ResourceTypeAIGateway,
					desiredGateway.DisplayName,
					isProtected,
					ActionUpdate,
				); err != nil {
					protectionErrors = append(protectionErrors, err)
				} else {
					p.planAIGatewayUpdate(current, desiredGateway, updateFields, changedFields, plan)
				}
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

		identityProviders := p.resources.GetAIGatewayIdentityProvidersForGateway(desiredGateway.Ref)
		if p.shouldPlanChild(
			plan,
			resources.ResourceTypeAIGateway,
			desiredGateway.Ref,
			resources.ResourceTypeAIGatewayIdentityProvider,
		) && (len(identityProviders) > 0 || plan.Metadata.Mode == PlanModeSync) {
			if err := p.planAIGatewayIdentityProviderChanges(
				ctx, plannerCtx, namespace, desiredGateway.DisplayName, gatewayID, desiredGateway.Ref,
				gatewayChangeID, identityProviders, plan,
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
	}

	if plan.Metadata.Mode == PlanModeSync {
		for _, current := range currentGateways {
			if matchedCurrent[aiGatewayIdentity(current)] {
				continue
			}

			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			if err := p.validateProtection(ResourceTypeAIGateway, current.DisplayName, isProtected, ActionDelete); err != nil {
				protectionErrors = append(protectionErrors, err)
			} else {
				p.planAIGatewayDelete(current, plan)
			}
		}
	}

	if len(protectionErrors) > 0 {
		return aiGatewayProtectionError(protectionErrors)
	}

	return nil
}

func (p *Planner) planExternalAIGatewayChildren(
	ctx context.Context,
	plannerCtx *Config,
	namespace string,
	desiredGateway resources.AIGatewayResource,
	plan *Plan,
) error {
	gatewayID := desiredGateway.GetKonnectID()
	if gatewayID == "" {
		plan.AddWarning("", fmt.Sprintf(
			"external ai_gateway %q has no resolved ID; skipping AI Gateway child planning",
			desiredGateway.GetRef(),
		))
		return nil
	}

	vaults := p.resources.GetAIGatewayVaultsForGateway(desiredGateway.Ref)
	if p.shouldPlanChild(
		plan,
		resources.ResourceTypeAIGateway,
		desiredGateway.Ref,
		resources.ResourceTypeAIGatewayVault,
	) && len(vaults) > 0 {
		if err := p.planAIGatewayVaultChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			"",
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
	) && len(dataPlaneCertificates) > 0 {
		if err := p.planAIGatewayDataPlaneCertificateChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			"",
			dataPlaneCertificates,
			plan,
		); err != nil {
			return err
		}
	}

	providers := p.resources.GetAIGatewayProvidersForGateway(desiredGateway.Ref)
	if p.shouldPlanChild(
		plan,
		resources.ResourceTypeAIGateway,
		desiredGateway.Ref,
		resources.ResourceTypeAIGatewayProvider,
	) && len(providers) > 0 {
		if err := p.planAIGatewayProviderChanges(
			ctx, plannerCtx, namespace, desiredGateway.DisplayName, gatewayID, desiredGateway.Ref, "", providers, plan,
		); err != nil {
			return err
		}
	}
	providerCreateDepsByName := aiGatewayProviderCreateDependencies(plan, namespace, desiredGateway.Ref)

	identityProviders := p.resources.GetAIGatewayIdentityProvidersForGateway(desiredGateway.Ref)
	if p.shouldPlanChild(
		plan,
		resources.ResourceTypeAIGateway,
		desiredGateway.Ref,
		resources.ResourceTypeAIGatewayIdentityProvider,
	) && len(identityProviders) > 0 {
		if err := p.planAIGatewayIdentityProviderChanges(
			ctx, plannerCtx, namespace, desiredGateway.DisplayName, gatewayID, desiredGateway.Ref, "", identityProviders, plan,
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
	) && len(policies) > 0 {
		if err := p.planAIGatewayPolicyChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			"",
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
	) && len(agents) > 0 {
		if err := p.planAIGatewayAgentChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			"",
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
	) && len(consumers) > 0 {
		if err := p.planAIGatewayConsumerChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			"",
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
	) && len(consumerGroups) > 0 {
		if err := p.planAIGatewayConsumerGroupChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			"",
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
	) && len(models) > 0 {
		if err := p.planAIGatewayModelChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			"",
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
	) && len(mcpServers) > 0 {
		if err := p.planAIGatewayMCPServerChanges(
			ctx,
			namespace,
			desiredGateway.Ref,
			desiredGateway.DisplayName,
			gatewayID,
			"",
			policyCreateDepsByName,
			mcpServers,
			plan,
		); err != nil {
			return err
		}
	}

	return nil
}

func (p *Planner) isAIGatewayExternal(gatewayRef string) bool {
	if p == nil || p.resources == nil {
		return false
	}
	gateway := p.resources.GetAIGatewayByRef(gatewayRef)
	return gateway != nil && gateway.IsExternal()
}

func (p *Planner) shouldUpdateAIGateway(
	current state.AIGateway,
	desired resources.AIGatewayResource,
) (bool, map[string]any, map[string]FieldChange) {
	updates := make(map[string]any)
	changedFields := make(map[string]FieldChange)

	if current.Name != desired.Name {
		updates[FieldName] = desired.Name
		changedFields[FieldName] = FieldChange{Old: current.Name, New: desired.Name}
	}

	if current.DisplayName != desired.DisplayName {
		updates[FieldDisplayName] = desired.DisplayName
		changedFields[FieldDisplayName] = FieldChange{Old: current.DisplayName, New: desired.DisplayName}
	}

	if desired.Description != nil {
		currentDescription := getString(current.Description)
		if currentDescription != *desired.Description {
			updates[FieldDescription] = *desired.Description
			changedFields[FieldDescription] = FieldChange{Old: currentDescription, New: *desired.Description}
		}
	}

	if desired.ProxyUrls != nil && !reflect.DeepEqual(current.ProxyUrls, desired.ProxyUrls) {
		updates[FieldProxyURLs] = desired.ProxyUrls
		changedFields[FieldProxyURLs] = FieldChange{Old: current.ProxyUrls, New: desired.ProxyUrls}
	}

	if desired.Labels != nil && labels.CompareUserLabels(current.NormalizedLabels, desired.GetLabels()) {
		updates[FieldLabels] = desired.GetLabels()
		changedFields[FieldLabels] = FieldChange{
			Old: labels.GetUserLabels(current.NormalizedLabels),
			New: labels.GetUserLabels(desired.GetLabels()),
		}
	}

	return len(updates) > 0, updates, changedFields
}

func indexAIGateways(
	gateways []state.AIGateway,
) (map[string]state.AIGateway, map[string]state.AIGateway, map[string][]state.AIGateway) {
	byID := make(map[string]state.AIGateway)
	byName := make(map[string]state.AIGateway)
	byDisplayName := make(map[string][]state.AIGateway)
	for _, gateway := range gateways {
		if gateway.ID != "" {
			byID[gateway.ID] = gateway
		}
		if gateway.Name != "" {
			byName[gateway.Name] = gateway
		}
		byDisplayName[gateway.DisplayName] = append(byDisplayName[gateway.DisplayName], gateway)
	}
	return byID, byName, byDisplayName
}

func matchCurrentAIGateway(
	desired resources.AIGatewayResource,
	currentByID map[string]state.AIGateway,
	currentByName map[string]state.AIGateway,
	currentByDisplayName map[string][]state.AIGateway,
) (state.AIGateway, bool, error) {
	if id := aiGatewayDesiredID(desired); id != "" {
		current, exists := currentByID[id]
		return current, exists, nil
	}

	if desired.Name != "" {
		current, exists := currentByName[desired.Name]
		if exists {
			return current, true, nil
		}
	}

	if desired.Ref != "" && desired.Ref != desired.Name {
		current, exists := currentByName[desired.Ref]
		if exists {
			return current, true, nil
		}
	}

	matches := currentByDisplayName[desired.DisplayName]
	switch len(matches) {
	case 0:
		return state.AIGateway{}, false, nil
	case 1:
		return matches[0], true, nil
	default:
		return state.AIGateway{}, false, fmt.Errorf(
			"multiple managed AI Gateways with display_name %q found in namespace; use a UUID ref or remove duplicates",
			desired.DisplayName,
		)
	}
}

func aiGatewayDesiredID(desired resources.AIGatewayResource) string {
	if id := desired.GetKonnectID(); id != "" {
		return id
	}
	if util.IsValidUUID(desired.Ref) {
		return desired.Ref
	}
	return ""
}

func aiGatewayIdentity(gateway state.AIGateway) string {
	if gateway.ID != "" {
		return "id:" + gateway.ID
	}
	if gateway.Name != "" {
		return "name:" + gateway.Name
	}
	return "display_name:" + gateway.DisplayName
}

func (p *Planner) planAIGatewayCreate(resource resources.AIGatewayResource, plan *Plan) string {
	namespace, protection := aiGatewayNamespaceAndProtection(resource)
	changeID := p.nextChangeID(ActionCreate, ResourceTypeAIGateway, resource.GetRef())

	change := PlannedChange{
		ID:           changeID,
		Action:       ActionCreate,
		ResourceType: ResourceTypeAIGateway,
		ResourceRef:  resource.GetRef(),
		Fields:       extractAIGatewayFields(resource),
		Namespace:    namespace,
		Protection:   protection,
	}
	plan.AddChange(change)
	return changeID
}

func (p *Planner) planAIGatewayUpdate(
	current state.AIGateway,
	desired resources.AIGatewayResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	namespace, _ := aiGatewayNamespaceAndProtection(desired)
	fields := make(map[string]any)
	maps.Copy(fields, updateFields)
	fields[FieldName] = desired.Name
	if fields[FieldName] == "" {
		fields[FieldName] = desired.GetRef()
	}
	fields[FieldDisplayName] = desired.DisplayName
	if _, hasLabels := fields[FieldLabels]; hasLabels {
		fields[FieldCurrentLabels] = current.NormalizedLabels
	}

	protection := ProtectionChange{
		Old: labels.IsProtectedResource(current.NormalizedLabels),
		New: desired.Kongctl != nil && desired.Kongctl.Protected != nil && *desired.Kongctl.Protected,
	}

	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeAIGateway, desired.GetRef()),
		Action:        ActionUpdate,
		ResourceType:  ResourceTypeAIGateway,
		ResourceRef:   desired.GetRef(),
		ResourceID:    current.ID,
		Fields:        fields,
		ChangedFields: changedFields,
		Namespace:     namespace,
		Protection:    protection,
	}
	plan.AddChange(change)
}

func (p *Planner) planAIGatewayDelete(current state.AIGateway, plan *Plan) {
	namespace := DefaultNamespace
	if ns, ok := current.NormalizedLabels[labels.NamespaceKey]; ok && ns != "" {
		namespace = ns
	}
	resourceRef := current.Name
	if resourceRef == "" {
		resourceRef = current.DisplayName
	}
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGateway, resourceRef),
		Action:       ActionDelete,
		ResourceType: ResourceTypeAIGateway,
		ResourceRef:  resourceRef,
		ResourceID:   current.ID,
		Fields: map[string]any{
			FieldName:        resourceRef,
			FieldDisplayName: current.DisplayName,
		},
		Namespace: namespace,
	}
	plan.AddChange(change)
}

func extractAIGatewayFields(resource resources.AIGatewayResource) map[string]any {
	fields := map[string]any{
		FieldName:        resource.Name,
		FieldDisplayName: resource.DisplayName,
	}
	if resource.Description != nil {
		fields[FieldDescription] = *resource.Description
	}
	if resource.ProxyUrls != nil {
		fields[FieldProxyURLs] = resource.ProxyUrls
	}
	if resource.Labels != nil {
		fields[FieldLabels] = resource.GetLabels()
	}
	return fields
}

func aiGatewayNamespaceAndProtection(resource resources.AIGatewayResource) (string, any) {
	namespace := DefaultNamespace
	if resource.Kongctl != nil && resource.Kongctl.Namespace != nil {
		namespace = *resource.Kongctl.Namespace
	}

	var protection any
	if resource.Kongctl != nil && resource.Kongctl.Protected != nil {
		protection = *resource.Kongctl.Protected
	}
	return namespace, protection
}

func aiGatewayProtectionError(protectionErrors []error) error {
	var errMsg strings.Builder
	errMsg.WriteString("Cannot generate plan due to protected resources:\n")
	for _, err := range protectionErrors {
		fmt.Fprintf(&errMsg, "- %s\n", err.Error())
	}
	errMsg.WriteString("\nTo proceed, first update these resources to set protected: false")
	return fmt.Errorf("%s", errMsg.String())
}
