package planner

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) reconcileOwnedAIGatewayModels(
	ctx context.Context,
	gatewayRef, gatewayName, gatewayID string,
	providerDeps, policyDeps map[string]string,
	desired []resources.AIGatewayModelResource,
	current []state.AIGatewayModel,
	namespaces map[string]bool,
	plan *Plan,
) error {
	byName := make(map[string]state.AIGatewayModel)
	for _, model := range current {
		byName[resources.AIGatewayModelName(model.AIGatewayModel)] = model
	}
	retained := make(map[string]bool)
	for _, model := range desired {
		namespace := resources.GetNamespace(model.Kongctl)
		namespaces[namespace] = true
		observed, exists := byName[model.Name()]
		if !exists {
			if plan.Metadata.Mode != PlanModeDelete {
				p.planAIGatewayModelCreate(ctx, namespace, gatewayRef, gatewayName, gatewayID, model,
					aiGatewayModelCreateDependencies(model, providerDeps, policyDeps), plan)
			}
			continue
		}
		listedMetadata := resources.AIGatewayModelManagedBy(observed.AIGatewayModel)
		if err := resources.ValidateModelOwnership(listedMetadata, namespace); err != nil {
			return fmt.Errorf("AI Gateway model %q on gateway %q: %w", model.Name(), gatewayRef, err)
		}
		id := resources.AIGatewayModelID(observed.AIGatewayModel)
		full, err := p.client.GetAIGatewayModel(ctx, gatewayID, id)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway model %s: %w", id, err)
		}
		if full == nil {
			return fmt.Errorf("AI Gateway model %q disappeared during planning; retry planning", model.Name())
		}
		metadata := resources.AIGatewayModelManagedBy(full.AIGatewayModel)
		if err := resources.ValidateModelOwnership(metadata, namespace); err != nil {
			return fmt.Errorf("AI Gateway model %q: %w", model.Name(), err)
		}
		p.recordMatchedIdentity(&model, full)
		retained[id] = true
		protected := metadata[resources.ModelProtectedKey] == "true"
		if plan.Metadata.Mode == PlanModeDelete {
			if protected {
				return fmt.Errorf("AI Gateway model %q is protected and cannot be deleted", model.Name())
			}
			p.planAIGatewayModelDelete(namespace, gatewayRef, gatewayID, id, model.Name(), plan)
			continue
		}
		// Clone the SDK branch before injecting generated ownership into the payload.
		owned := model
		if model.AIGatewayModelAPI != nil {
			owned.AIGatewayModelAPI = new(*model.AIGatewayModelAPI)
		}
		if model.AIGatewayModelModel != nil {
			owned.AIGatewayModelModel = new(*model.AIGatewayModelModel)
		}
		owned.SetManagedBy(resources.BuildModelManagedBy(
			metadata, model.ManagedBy(), namespace, model.Protected(), build.Version(ctx),
		))
		if err := resources.ValidateChildManagedBy(owned.ManagedBy(), false); err != nil {
			return err
		}
		update, fields, changed, err := p.shouldUpdateAIGatewayModel(*full, owned)
		if err != nil {
			return err
		}
		if !update {
			continue
		}
		if protected && (model.Protected() || len(changed) != 1 || changed[FieldManagedBy].New == nil) {
			return fmt.Errorf(
				"AI Gateway model %q is protected; unprotect it in a separate update before changing its payload",
				model.Name(),
			)
		}
		if protected {
			if !resources.ChildUnprotectionMetadataOnly(metadata, owned.ManagedBy()) {
				return fmt.Errorf(
					"AI Gateway model %q is protected; only protection can change while unprotecting",
					model.Name(),
				)
			}
			fields, err = resources.AIGatewayModelMutablePayloadMap(full.AIGatewayModel)
			if err != nil {
				return err
			}
			fields[FieldManagedBy] = owned.ManagedBy()
		}
		p.planAIGatewayModelUpdate(namespace, gatewayRef, gatewayID, id, owned, fields, changed,
			aiGatewayModelCreateDependencies(model, providerDeps, policyDeps), plan)
		if protected != model.Protected() {
			plan.Changes[len(plan.Changes)-1].Protection = ProtectionChange{Old: protected, New: model.Protected()}
		}
	}
	if plan.Metadata.Mode != PlanModeSync {
		return nil
	}
	// Directly constructed ResourceSets cannot record source namespace presence.
	// Only use defaults as a fallback for an explicitly scoped empty collection.
	if len(namespaces) == 0 && len(desired) == 0 {
		for _, namespace := range p.resources.DefaultNamespaces {
			namespaces[namespace] = true
		}
		if len(namespaces) == 0 {
			namespaces[DefaultNamespace] = true
		}
	}
	for _, model := range current {
		id := resources.AIGatewayModelID(model.AIGatewayModel)
		metadata := resources.AIGatewayModelManagedBy(model.AIGatewayModel)
		namespace := metadata[resources.ModelNamespaceKey]
		if retained[id] || namespace == "" || !namespaces[namespace] {
			continue
		}
		if err := resources.ValidateModelOwnership(metadata, namespace); err != nil {
			return err
		}
		if metadata[resources.ModelProtectedKey] == "true" {
			return fmt.Errorf(
				"AI Gateway model %q is protected and cannot be deleted",
				resources.AIGatewayModelName(model.AIGatewayModel),
			)
		}
		p.planAIGatewayModelDelete(namespace, gatewayRef, gatewayID, id,
			resources.AIGatewayModelName(model.AIGatewayModel), plan)
	}
	return nil
}

// Different external selectors can resolve to the same gateway. Reconcile its
// complete desired collection once, so aliases cannot prune each other's models.
func (p *Planner) modelGatewayDeclarations(gatewayRef, gatewayID string) (
	[]resources.AIGatewayModelResource, map[string]bool, error,
) {
	aliases := map[string]bool{gatewayRef: true}
	for _, gateway := range p.resources.AIGateways {
		if gateway.GetKonnectID() == gatewayID {
			aliases[gateway.Ref] = true
		}
	}
	namespaces := make(map[string]bool)
	if p.resources.SyncScope != nil {
		for alias := range aliases {
			for namespace := range p.resources.SyncScope.OwnedChildNamespaces(resources.ResourceTypeAIGatewayModel, alias) {
				namespaces[namespace] = true
			}
		}
	}
	var desired []resources.AIGatewayModelResource
	byName := make(map[string]string)
	for _, model := range p.resources.AIGatewayModels {
		if !aliases[resources.NormalizeResourceRef(model.AIGateway)] {
			continue
		}
		if ref, exists := byName[model.Name()]; exists {
			return nil, nil, fmt.Errorf("duplicate desired AI Gateway model name %q on gateway %q (refs %q and %q)",
				model.Name(), gatewayRef, ref, model.Ref)
		}
		byName[model.Name()] = model.Ref
		desired = append(desired, model)
	}
	return desired, namespaces, nil
}

// Sorted namespaces make scope metadata deterministic across multi-file loads.
func modelScopeNamespaces(scope *resources.SyncScope) map[string][]string {
	if scope == nil || len(scope.ModelNamespaces) == 0 {
		return nil
	}
	result := make(map[string][]string)
	for parent, namespaces := range scope.ModelNamespaces {
		result[parent] = slices.Sorted(maps.Keys(namespaces))
	}
	return result
}
