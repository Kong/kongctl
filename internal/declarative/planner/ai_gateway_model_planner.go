package planner

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/util"
)

func (p *Planner) planAIGatewayModelChanges(
	ctx context.Context,
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	gatewayChangeID string,
	providerCreateDepsByName map[string]string,
	policyCreateDepsByName map[string]string,
	desired []resources.AIGatewayModelResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning AI Gateway model changes",
		slog.String("gateway_ref", gatewayRef),
		slog.String("gateway_id", gatewayID),
		slog.String("gateway_change_id", gatewayChangeID),
		slog.Int("desired_count", len(desired)),
	)

	if gatewayID == "" {
		p.planAIGatewayModelCreatesForNewGateway(
			namespace,
			gatewayRef,
			gatewayName,
			gatewayChangeID,
			providerCreateDepsByName,
			policyCreateDepsByName,
			desired,
			plan,
		)
		return nil
	}

	currentModels, err := p.client.ListAIGatewayModels(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway models for gateway %s: %w", gatewayID, err)
	}

	currentByID, currentByName := indexAIGatewayModels(currentModels)
	desiredKeys := make(map[string]bool)

	for _, desiredModel := range desired {
		current, exists := matchCurrentAIGatewayModel(desiredModel, currentByID, currentByName)
		desiredKeys[desiredModel.Name()] = true
		if id := aiGatewayModelDesiredID(desiredModel); id != "" {
			desiredKeys[id] = true
		}

		if !exists {
			dependsOn := aiGatewayModelCreateDependencies(
				desiredModel,
				providerCreateDepsByName,
				policyCreateDepsByName,
			)
			p.planAIGatewayModelCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredModel, dependsOn, plan)
			continue
		}

		modelID := resources.AIGatewayModelID(current.AIGatewayModel)
		fullModel, err := p.client.GetAIGatewayModel(ctx, gatewayID, modelID)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway model %s: %w", modelID, err)
		}
		if fullModel == nil {
			dependsOn := aiGatewayModelCreateDependencies(
				desiredModel,
				providerCreateDepsByName,
				policyCreateDepsByName,
			)
			p.planAIGatewayModelCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredModel, dependsOn, plan)
			continue
		}

		needsUpdate, updateFields, changedFields, err := p.shouldUpdateAIGatewayModel(*fullModel, desiredModel)
		if err != nil {
			return err
		}
		if needsUpdate {
			p.planAIGatewayModelUpdate(
				namespace,
				gatewayRef,
				gatewayID,
				modelID,
				desiredModel,
				updateFields,
				changedFields,
				aiGatewayModelCreateDependencies(
					desiredModel,
					providerCreateDepsByName,
					policyCreateDepsByName,
				),
				plan,
			)
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		for _, current := range currentModels {
			modelID := resources.AIGatewayModelID(current.AIGatewayModel)
			modelName := resources.AIGatewayModelName(current.AIGatewayModel)
			if desiredKeys[modelID] || desiredKeys[modelName] {
				continue
			}
			p.planAIGatewayModelDelete(gatewayRef, gatewayID, modelID, modelName, plan)
		}
	}

	return nil
}

func (p *Planner) planAIGatewayModelCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	providerCreateDepsByName map[string]string,
	policyCreateDepsByName map[string]string,
	models []resources.AIGatewayModelResource,
	plan *Plan,
) {
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}
	for _, model := range models {
		modelDependsOn := slices.Clone(dependsOn)
		for _, dep := range aiGatewayModelCreateDependencies(model, providerCreateDepsByName, policyCreateDepsByName) {
			modelDependsOn = appendDependsOn(modelDependsOn, dep)
		}
		p.planAIGatewayModelCreate(namespace, gatewayRef, gatewayName, "", model, modelDependsOn, plan)
	}
}

func (p *Planner) planAIGatewayModelCreate(
	namespace string,
	gatewayRef string,
	_ string,
	gatewayID string,
	model resources.AIGatewayModelResource,
	dependsOn []string,
	plan *Plan,
) {
	fields, err := model.MutablePayloadMap()
	if err != nil {
		plan.AddWarning(model.GetRef(), fmt.Sprintf("failed to build AI Gateway model create payload: %s", err))
		return
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayModel, model.Ref),
		ResourceType: ResourceTypeAIGatewayModel,
		ResourceRef:  model.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		Namespace:    namespace,
		DependsOn:    dependsOn,
	}
	if gatewayID != "" {
		change.Parent = &ParentInfo{Ref: gatewayRef, ID: gatewayID}
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

func (p *Planner) planAIGatewayModelUpdate(
	namespace string,
	gatewayRef string,
	gatewayID string,
	modelID string,
	model resources.AIGatewayModelResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	dependsOn []string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeAIGatewayModel, model.Ref),
		ResourceType:  ResourceTypeAIGatewayModel,
		ResourceRef:   model.Ref,
		ResourceID:    modelID,
		Action:        ActionUpdate,
		Fields:        updateFields,
		ChangedFields: changedFields,
		Namespace:     namespace,
		DependsOn:     dependsOn,
		Parent:        &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func (p *Planner) planAIGatewayModelDelete(
	gatewayRef string,
	gatewayID string,
	modelID string,
	modelName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGatewayModel, modelName),
		ResourceType: ResourceTypeAIGatewayModel,
		ResourceRef:  modelName,
		ResourceID:   modelID,
		Action:       ActionDelete,
		Fields: map[string]any{
			FieldName: modelName,
		},
		Parent: &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func (p *Planner) shouldUpdateAIGatewayModel(
	current state.AIGatewayModel,
	desired resources.AIGatewayModelResource,
) (bool, map[string]any, map[string]FieldChange, error) {
	currentPayload, err := resources.AIGatewayModelMutablePayloadMap(current.AIGatewayModel)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to normalize current AI Gateway model: %w", err)
	}
	desiredPayload, err := desired.MutablePayloadMap()
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to normalize desired AI Gateway model %q: %w", desired.Ref, err)
	}

	currentCompare, desiredCompare := normalizeAIGatewayPayloadsForComparison(currentPayload, desiredPayload)
	currentCompare, desiredCompare = normalizeAIGatewayPolicyReferencesForComparison(
		currentCompare,
		desiredCompare,
		p.resources,
	)

	changedFields := diffAIGatewayPayloads(currentPayload, desiredPayload, currentCompare, desiredCompare)
	if len(changedFields) == 0 {
		return false, nil, nil, nil
	}

	return true, clonePayloadMap(desiredPayload), changedFields, nil
}

func indexAIGatewayModels(
	models []state.AIGatewayModel,
) (map[string]state.AIGatewayModel, map[string]state.AIGatewayModel) {
	byID := make(map[string]state.AIGatewayModel)
	byName := make(map[string]state.AIGatewayModel)
	for _, model := range models {
		if id := resources.AIGatewayModelID(model.AIGatewayModel); id != "" {
			byID[id] = model
		}
		if name := resources.AIGatewayModelName(model.AIGatewayModel); name != "" {
			byName[name] = model
		}
	}
	return byID, byName
}

func matchCurrentAIGatewayModel(
	desired resources.AIGatewayModelResource,
	currentByID map[string]state.AIGatewayModel,
	currentByName map[string]state.AIGatewayModel,
) (state.AIGatewayModel, bool) {
	if id := aiGatewayModelDesiredID(desired); id != "" {
		current, exists := currentByID[id]
		return current, exists
	}
	current, exists := currentByName[desired.Name()]
	return current, exists
}

func aiGatewayModelDesiredID(desired resources.AIGatewayModelResource) string {
	if id := desired.GetKonnectID(); id != "" {
		return id
	}
	if util.IsValidUUID(desired.Ref) {
		return desired.Ref
	}
	return ""
}

func aiGatewayProviderCreateDependencies(plan *Plan, namespace string, gatewayRef string) map[string]string {
	if plan == nil {
		return nil
	}

	depsByName := make(map[string]string)
	for _, change := range plan.Changes {
		if change.Action != ActionCreate ||
			change.ResourceType != ResourceTypeAIGatewayProvider ||
			change.Namespace != namespace ||
			!aiGatewayChildChangeMatchesParent(change, gatewayRef) {
			continue
		}

		name, ok := change.Fields[FieldName].(string)
		if ok && name != "" {
			depsByName[name] = change.ID
		}
	}
	if len(depsByName) == 0 {
		return nil
	}
	return depsByName
}

func aiGatewayChildChangeMatchesParent(change PlannedChange, gatewayRef string) bool {
	if change.Parent != nil && change.Parent.Ref == gatewayRef {
		return true
	}
	if refInfo, ok := change.References[FieldAIGatewayID]; ok && refInfo.Ref == gatewayRef {
		return true
	}
	return false
}

func aiGatewayModelCreateDependencies(
	model resources.AIGatewayModelResource,
	providerCreateDepsByName map[string]string,
	policyCreateDepsByName map[string]string,
) []string {
	var deps []string
	for _, dep := range aiGatewayModelProviderCreateDependencies(model, providerCreateDepsByName) {
		deps = appendDependsOn(deps, dep)
	}

	payload, err := model.MutablePayloadMap()
	if err != nil {
		return deps
	}
	for _, dep := range aiGatewayPolicyReferenceDependencies(payload, policyCreateDepsByName) {
		deps = appendDependsOn(deps, dep)
	}
	return deps
}

func aiGatewayModelProviderCreateDependencies(
	model resources.AIGatewayModelResource,
	providerCreateDepsByName map[string]string,
) []string {
	if len(providerCreateDepsByName) == 0 {
		return nil
	}

	payload, err := model.MutablePayloadMap()
	if err != nil {
		return nil
	}
	targetModels, ok := payload[FieldTargets].([]any)
	if !ok {
		targetModels, ok = payload[FieldTargetModels].([]any)
	}
	if !ok {
		return nil
	}

	var deps []string
	for _, targetModel := range targetModels {
		target, ok := targetModel.(map[string]any)
		if !ok {
			continue
		}
		providerName, ok := target["provider"].(string)
		if !ok || providerName == "" {
			continue
		}
		if dep := providerCreateDepsByName[providerName]; dep != "" {
			deps = appendDependsOn(deps, dep)
		}
	}
	return deps
}
