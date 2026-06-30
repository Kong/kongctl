package planner

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/util"
)

func (p *Planner) planAIGatewayConsumerChanges(
	ctx context.Context,
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	gatewayChangeID string,
	policyCreateDepsByRefOrName map[string]string,
	desired []resources.AIGatewayConsumerResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning AI Gateway Consumer changes",
		slog.String("gateway_ref", gatewayRef),
		slog.String("gateway_id", gatewayID),
		slog.String("gateway_change_id", gatewayChangeID),
		slog.Int("desired_count", len(desired)),
	)

	if gatewayID == "" {
		p.planAIGatewayConsumerCreatesForNewGateway(
			namespace,
			gatewayRef,
			gatewayName,
			gatewayChangeID,
			policyCreateDepsByRefOrName,
			desired,
			plan,
		)
		return nil
	}

	currentConsumers, err := p.client.ListAIGatewayConsumers(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway Consumers for gateway %s: %w", gatewayID, err)
	}

	currentByID, currentByName := indexAIGatewayConsumers(currentConsumers)
	desiredKeys := make(map[string]bool)

	for _, desiredConsumer := range desired {
		current, exists := matchCurrentAIGatewayConsumer(desiredConsumer, currentByID, currentByName)
		desiredKeys[desiredConsumer.Name] = true
		if id := aiGatewayConsumerDesiredID(desiredConsumer); id != "" {
			desiredKeys[id] = true
		}

		if !exists {
			dependsOn := aiGatewayConsumerPolicyCreateDependencies(
				desiredConsumer,
				policyCreateDepsByRefOrName,
			)
			p.planAIGatewayConsumerCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredConsumer, dependsOn, plan)
			continue
		}

		consumerID := resources.AIGatewayConsumerID(current.AIGatewayConsumer)
		if consumer := p.resources.GetAIGatewayConsumerByRef(desiredConsumer.Ref); consumer != nil {
			consumer.SetKonnectID(consumerID)
		}
		fullConsumer, err := p.client.GetAIGatewayConsumer(ctx, gatewayID, consumerID)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway Consumer %s: %w", consumerID, err)
		}
		if fullConsumer == nil {
			dependsOn := aiGatewayConsumerPolicyCreateDependencies(
				desiredConsumer,
				policyCreateDepsByRefOrName,
			)
			p.planAIGatewayConsumerCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredConsumer, dependsOn, plan)
			continue
		}

		needsUpdate, updateFields, changedFields, err := p.shouldUpdateAIGatewayConsumer(*fullConsumer, desiredConsumer)
		if err != nil {
			return err
		}
		if needsUpdate {
			p.planAIGatewayConsumerUpdate(
				namespace,
				gatewayRef,
				gatewayID,
				consumerID,
				desiredConsumer,
				updateFields,
				changedFields,
				aiGatewayConsumerPolicyCreateDependencies(
					desiredConsumer,
					policyCreateDepsByRefOrName,
				),
				plan,
			)
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		for _, current := range currentConsumers {
			consumerID := resources.AIGatewayConsumerID(current.AIGatewayConsumer)
			consumerName := resources.AIGatewayConsumerName(current.AIGatewayConsumer)
			if desiredKeys[consumerID] || desiredKeys[consumerName] {
				continue
			}
			p.planAIGatewayConsumerDelete(gatewayRef, gatewayID, consumerID, consumerName, plan)
		}
	}

	return nil
}

func (p *Planner) planAIGatewayConsumerCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	policyCreateDepsByRefOrName map[string]string,
	consumers []resources.AIGatewayConsumerResource,
	plan *Plan,
) {
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}
	for _, consumer := range consumers {
		consumerDependsOn := append([]string{}, dependsOn...)
		for _, dep := range aiGatewayConsumerPolicyCreateDependencies(consumer, policyCreateDepsByRefOrName) {
			consumerDependsOn = appendDependsOn(consumerDependsOn, dep)
		}
		p.planAIGatewayConsumerCreate(namespace, gatewayRef, gatewayName, "", consumer, consumerDependsOn, plan)
	}
}

func (p *Planner) planAIGatewayConsumerCreate(
	namespace string,
	gatewayRef string,
	_ string,
	gatewayID string,
	consumer resources.AIGatewayConsumerResource,
	dependsOn []string,
	plan *Plan,
) {
	fields, err := consumer.MutablePayloadMap()
	if err != nil {
		plan.AddWarning(consumer.GetRef(), fmt.Sprintf("failed to build AI Gateway Consumer create payload: %s", err))
		return
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayConsumer, consumer.Ref),
		ResourceType: ResourceTypeAIGatewayConsumer,
		ResourceRef:  consumer.Ref,
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

func (p *Planner) planAIGatewayConsumerUpdate(
	namespace string,
	gatewayRef string,
	gatewayID string,
	consumerID string,
	consumer resources.AIGatewayConsumerResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	dependsOn []string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeAIGatewayConsumer, consumer.Ref),
		ResourceType:  ResourceTypeAIGatewayConsumer,
		ResourceRef:   consumer.Ref,
		ResourceID:    consumerID,
		Action:        ActionUpdate,
		Fields:        updateFields,
		ChangedFields: changedFields,
		Namespace:     namespace,
		DependsOn:     dependsOn,
		Parent:        &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func (p *Planner) planAIGatewayConsumerDelete(
	gatewayRef string,
	gatewayID string,
	consumerID string,
	consumerName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGatewayConsumer, consumerName),
		ResourceType: ResourceTypeAIGatewayConsumer,
		ResourceRef:  consumerName,
		ResourceID:   consumerID,
		Action:       ActionDelete,
		Fields: map[string]any{
			FieldName: consumerName,
		},
		Parent: &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func (p *Planner) shouldUpdateAIGatewayConsumer(
	current state.AIGatewayConsumer,
	desired resources.AIGatewayConsumerResource,
) (bool, map[string]any, map[string]FieldChange, error) {
	currentPayload, err := resources.AIGatewayConsumerMutablePayloadMap(current.AIGatewayConsumer)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to normalize current AI Gateway Consumer: %w", err)
	}
	desiredPayload, err := desired.MutablePayloadMap()
	if err != nil {
		return false, nil, nil, fmt.Errorf(
			"failed to normalize desired AI Gateway Consumer %q: %w",
			desired.Ref,
			err,
		)
	}

	currentCompare, desiredCompare := normalizeAIGatewayPolicyReferencesForComparison(
		currentPayload,
		desiredPayload,
		p.resources,
	)

	changedFields := make(map[string]FieldChange)
	keys := make(map[string]struct{}, len(currentCompare)+len(desiredCompare))
	for key := range currentCompare {
		keys[key] = struct{}{}
	}
	for key := range desiredCompare {
		keys[key] = struct{}{}
	}
	for key := range keys {
		if !reflect.DeepEqual(currentCompare[key], desiredCompare[key]) {
			changedFields[key] = FieldChange{Old: currentPayload[key], New: desiredPayload[key]}
		}
	}
	if len(changedFields) == 0 {
		return false, nil, nil, nil
	}

	updateFields := make(map[string]any, len(desiredPayload))
	maps.Copy(updateFields, desiredPayload)
	return true, updateFields, changedFields, nil
}

func indexAIGatewayConsumers(
	consumers []state.AIGatewayConsumer,
) (map[string]state.AIGatewayConsumer, map[string]state.AIGatewayConsumer) {
	byID := make(map[string]state.AIGatewayConsumer)
	byName := make(map[string]state.AIGatewayConsumer)
	for _, consumer := range consumers {
		if id := resources.AIGatewayConsumerID(consumer.AIGatewayConsumer); id != "" {
			byID[id] = consumer
		}
		if name := resources.AIGatewayConsumerName(consumer.AIGatewayConsumer); name != "" {
			byName[name] = consumer
		}
	}
	return byID, byName
}

func matchCurrentAIGatewayConsumer(
	desired resources.AIGatewayConsumerResource,
	currentByID map[string]state.AIGatewayConsumer,
	currentByName map[string]state.AIGatewayConsumer,
) (state.AIGatewayConsumer, bool) {
	if id := aiGatewayConsumerDesiredID(desired); id != "" {
		current, exists := currentByID[id]
		return current, exists
	}
	current, exists := currentByName[desired.Name]
	return current, exists
}

func aiGatewayConsumerDesiredID(desired resources.AIGatewayConsumerResource) string {
	if id := desired.GetKonnectID(); id != "" {
		return id
	}
	if util.IsValidUUID(desired.Ref) {
		return desired.Ref
	}
	return ""
}

func aiGatewayConsumerPolicyCreateDependencies(
	consumer resources.AIGatewayConsumerResource,
	policyCreateDepsByRefOrName map[string]string,
) []string {
	payload, err := consumer.MutablePayloadMap()
	if err != nil {
		return nil
	}
	return aiGatewayPolicyReferenceDependencies(payload, policyCreateDepsByRefOrName)
}
