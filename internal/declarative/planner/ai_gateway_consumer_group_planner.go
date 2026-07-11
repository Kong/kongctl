package planner

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util"
)

func (p *Planner) planAIGatewayConsumerGroupChanges(
	ctx context.Context,
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	gatewayChangeID string,
	policyCreateDepsByRefOrName map[string]string,
	desired []resources.AIGatewayConsumerGroupResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning AI Gateway Consumer Group changes",
		slog.String("gateway_ref", gatewayRef),
		slog.String("gateway_id", gatewayID),
		slog.String("gateway_change_id", gatewayChangeID),
		slog.Int("desired_count", len(desired)),
	)

	if gatewayID == "" {
		p.planAIGatewayConsumerGroupCreatesForNewGateway(
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

	currentGroups, err := p.client.ListAIGatewayConsumerGroups(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway Consumer Groups for gateway %s: %w", gatewayID, err)
	}

	currentByID, currentByName := indexAIGatewayConsumerGroups(currentGroups)
	desiredKeys := make(map[string]bool)
	consumerCreateDepsByRefOrName := aiGatewayConsumerCreateDependencies(plan, namespace, gatewayRef)

	for _, desiredGroup := range desired {
		current, exists := matchCurrentAIGatewayConsumerGroup(desiredGroup, currentByID, currentByName)
		desiredKeys[desiredGroup.Name] = true
		if id := aiGatewayConsumerGroupDesiredID(desiredGroup); id != "" {
			desiredKeys[id] = true
		}

		if !exists {
			dependsOn := aiGatewayConsumerGroupPolicyCreateDependencies(
				desiredGroup,
				policyCreateDepsByRefOrName,
			)
			for _, dep := range aiGatewayConsumerGroupConsumerCreateDependencies(desiredGroup, consumerCreateDepsByRefOrName) {
				dependsOn = appendDependsOn(dependsOn, dep)
			}
			p.planAIGatewayConsumerGroupCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredGroup, dependsOn, plan)
			continue
		}

		groupID := resources.AIGatewayConsumerGroupID(current.AIGatewayConsumerGroup)
		if group := p.resources.GetAIGatewayConsumerGroupByRef(desiredGroup.Ref); group != nil {
			group.SetKonnectID(groupID)
		}
		fullGroup, err := p.client.GetAIGatewayConsumerGroup(ctx, gatewayID, groupID)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway Consumer Group %s: %w", groupID, err)
		}
		if fullGroup == nil {
			dependsOn := aiGatewayConsumerGroupPolicyCreateDependencies(
				desiredGroup,
				policyCreateDepsByRefOrName,
			)
			for _, dep := range aiGatewayConsumerGroupConsumerCreateDependencies(desiredGroup, consumerCreateDepsByRefOrName) {
				dependsOn = appendDependsOn(dependsOn, dep)
			}
			p.planAIGatewayConsumerGroupCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredGroup, dependsOn, plan)
			continue
		}

		var currentConsumers []state.AIGatewayConsumer
		if _, managesConsumers, err := desiredGroup.ConsumerNames(); err != nil {
			return err
		} else if managesConsumers {
			currentConsumers, err = p.client.ListAIGatewayConsumersInConsumerGroup(ctx, gatewayID, groupID)
			if err != nil {
				return fmt.Errorf("failed to list AI Gateway Consumers in Consumer Group %s: %w", groupID, err)
			}
		}

		needsUpdate, updateFields, changedFields, err := p.shouldUpdateAIGatewayConsumerGroup(
			*fullGroup,
			desiredGroup,
			currentConsumers,
		)
		if err != nil {
			return err
		}
		if needsUpdate {
			dependsOn := aiGatewayConsumerGroupPolicyCreateDependencies(
				desiredGroup,
				policyCreateDepsByRefOrName,
			)
			for _, dep := range aiGatewayConsumerGroupConsumerCreateDependencies(desiredGroup, consumerCreateDepsByRefOrName) {
				dependsOn = appendDependsOn(dependsOn, dep)
			}
			p.planAIGatewayConsumerGroupUpdate(
				namespace,
				gatewayRef,
				gatewayID,
				groupID,
				desiredGroup,
				updateFields,
				changedFields,
				dependsOn,
				plan,
			)
		}
	}

	if plan.Metadata.Mode == PlanModeSync && !p.isAIGatewayExternal(gatewayRef) {
		for _, current := range currentGroups {
			groupID := resources.AIGatewayConsumerGroupID(current.AIGatewayConsumerGroup)
			groupName := resources.AIGatewayConsumerGroupName(current.AIGatewayConsumerGroup)
			if desiredKeys[groupID] || desiredKeys[groupName] {
				continue
			}
			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			if err := p.validateProtection(
				ResourceTypeAIGatewayConsumerGroup,
				groupName,
				isProtected,
				ActionDelete,
			); err != nil {
				return err
			}
			p.planAIGatewayConsumerGroupDelete(namespace, gatewayRef, gatewayID, groupID, groupName, plan)
		}
	}

	return nil
}

func (p *Planner) planAIGatewayConsumerGroupCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	policyCreateDepsByRefOrName map[string]string,
	groups []resources.AIGatewayConsumerGroupResource,
	plan *Plan,
) {
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}
	for _, group := range groups {
		groupDependsOn := slices.Clone(dependsOn)
		for _, dep := range aiGatewayConsumerGroupPolicyCreateDependencies(group, policyCreateDepsByRefOrName) {
			groupDependsOn = appendDependsOn(groupDependsOn, dep)
		}
		consumerCreateDepsByRefOrName := aiGatewayConsumerCreateDependencies(plan, namespace, gatewayRef)
		for _, dep := range aiGatewayConsumerGroupConsumerCreateDependencies(group, consumerCreateDepsByRefOrName) {
			groupDependsOn = appendDependsOn(groupDependsOn, dep)
		}
		p.planAIGatewayConsumerGroupCreate(namespace, gatewayRef, gatewayName, "", group, groupDependsOn, plan)
	}
}

func (p *Planner) planAIGatewayConsumerGroupCreate(
	namespace string,
	gatewayRef string,
	_ string,
	gatewayID string,
	group resources.AIGatewayConsumerGroupResource,
	dependsOn []string,
	plan *Plan,
) {
	fields, err := group.MutablePayloadMap()
	if err != nil {
		plan.AddWarning(group.GetRef(), fmt.Sprintf("failed to build AI Gateway Consumer Group create payload: %s", err))
		return
	}
	p.resolveAIGatewayConsumerGroupConsumerRefs(fields)

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayConsumerGroup, group.Ref),
		ResourceType: ResourceTypeAIGatewayConsumerGroup,
		ResourceRef:  group.Ref,
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

func (p *Planner) planAIGatewayConsumerGroupUpdate(
	namespace string,
	gatewayRef string,
	gatewayID string,
	groupID string,
	group resources.AIGatewayConsumerGroupResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	dependsOn []string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeAIGatewayConsumerGroup, group.Ref),
		ResourceType:  ResourceTypeAIGatewayConsumerGroup,
		ResourceRef:   group.Ref,
		ResourceID:    groupID,
		Action:        ActionUpdate,
		Fields:        updateFields,
		ChangedFields: changedFields,
		Namespace:     namespace,
		DependsOn:     dependsOn,
		Parent:        &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func (p *Planner) planAIGatewayConsumerGroupDelete(
	namespace string,
	gatewayRef string,
	gatewayID string,
	groupID string,
	groupName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGatewayConsumerGroup, groupName),
		ResourceType: ResourceTypeAIGatewayConsumerGroup,
		ResourceRef:  groupName,
		ResourceID:   groupID,
		Action:       ActionDelete,
		Namespace:    namespace,
		Fields: map[string]any{
			FieldName: groupName,
		},
		Parent: &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func (p *Planner) shouldUpdateAIGatewayConsumerGroup(
	current state.AIGatewayConsumerGroup,
	desired resources.AIGatewayConsumerGroupResource,
	currentConsumers []state.AIGatewayConsumer,
) (bool, map[string]any, map[string]FieldChange, error) {
	currentPayload, err := resources.AIGatewayConsumerGroupMutablePayloadMap(current.AIGatewayConsumerGroup)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to normalize current AI Gateway Consumer Group: %w", err)
	}
	desiredPayload, err := desired.MutablePayloadMap()
	if err != nil {
		return false, nil, nil, fmt.Errorf(
			"failed to normalize desired AI Gateway Consumer Group %q: %w",
			desired.Ref,
			err,
		)
	}
	desiredConsumers, managesConsumers, err := desired.ConsumerNames()
	if err != nil {
		return false, nil, nil, fmt.Errorf(
			"failed to normalize desired AI Gateway Consumer Group consumers %q: %w",
			desired.Ref,
			err,
		)
	}
	if managesConsumers {
		desiredConsumers = p.resolveAIGatewayConsumerGroupConsumerRefs(desiredPayload)
	}
	resources.StripAIGatewayConsumerGroupMembershipFields(currentPayload)
	resources.StripAIGatewayConsumerGroupMembershipFields(desiredPayload)

	currentCompare, desiredCompare := normalizeAIGatewayPayloadsForComparison(currentPayload, desiredPayload)
	currentCompare, desiredCompare = normalizeAIGatewayPolicyReferencesForComparison(
		currentCompare,
		desiredCompare,
		p.resources,
	)

	changedFields := diffAIGatewayPayloads(currentPayload, desiredPayload, currentCompare, desiredCompare)
	if managesConsumers {
		currentConsumerNames := aiGatewayConsumerNamesFromState(currentConsumers)
		if !slices.Equal(currentConsumerNames, normalizedAIGatewayConsumerGroupConsumers(desiredConsumers)) {
			changedFields[FieldConsumers] = FieldChange{
				Old: currentConsumerNames,
				New: normalizedAIGatewayConsumerGroupConsumers(desiredConsumers),
			}
		}
	}
	if len(changedFields) == 0 {
		return false, nil, nil, nil
	}

	updateFields := clonePayloadMap(desiredPayload)
	if managesConsumers {
		updateFields[FieldConsumers] = normalizedAIGatewayConsumerGroupConsumers(desiredConsumers)
	}
	return true, updateFields, changedFields, nil
}

func indexAIGatewayConsumerGroups(
	groups []state.AIGatewayConsumerGroup,
) (map[string]state.AIGatewayConsumerGroup, map[string]state.AIGatewayConsumerGroup) {
	byID := make(map[string]state.AIGatewayConsumerGroup)
	byName := make(map[string]state.AIGatewayConsumerGroup)
	for _, group := range groups {
		if id := resources.AIGatewayConsumerGroupID(group.AIGatewayConsumerGroup); id != "" {
			byID[id] = group
		}
		if name := resources.AIGatewayConsumerGroupName(group.AIGatewayConsumerGroup); name != "" {
			byName[name] = group
		}
	}
	return byID, byName
}

func matchCurrentAIGatewayConsumerGroup(
	desired resources.AIGatewayConsumerGroupResource,
	currentByID map[string]state.AIGatewayConsumerGroup,
	currentByName map[string]state.AIGatewayConsumerGroup,
) (state.AIGatewayConsumerGroup, bool) {
	if id := aiGatewayConsumerGroupDesiredID(desired); id != "" {
		current, exists := currentByID[id]
		return current, exists
	}
	current, exists := currentByName[desired.Name]
	return current, exists
}

func aiGatewayConsumerGroupDesiredID(desired resources.AIGatewayConsumerGroupResource) string {
	if id := desired.GetKonnectID(); id != "" {
		return id
	}
	if util.IsValidUUID(desired.Ref) {
		return desired.Ref
	}
	return ""
}

func aiGatewayConsumerGroupPolicyCreateDependencies(
	group resources.AIGatewayConsumerGroupResource,
	policyCreateDepsByRefOrName map[string]string,
) []string {
	payload, err := group.MutablePayloadMap()
	if err != nil {
		return nil
	}
	return aiGatewayPolicyReferenceDependencies(payload, policyCreateDepsByRefOrName)
}

func aiGatewayConsumerCreateDependencies(
	plan *Plan,
	namespace string,
	gatewayRef string,
) map[string]string {
	if plan == nil {
		return nil
	}

	depsByRefOrName := make(map[string]string)
	for _, change := range plan.Changes {
		if change.Action != ActionCreate ||
			change.ResourceType != ResourceTypeAIGatewayConsumer ||
			change.Namespace != namespace ||
			!aiGatewayChildChangeMatchesParent(change, gatewayRef) {
			continue
		}
		if change.ResourceRef != "" {
			depsByRefOrName[change.ResourceRef] = change.ID
		}
		if name, ok := change.Fields[FieldName].(string); ok && name != "" {
			depsByRefOrName[name] = change.ID
		}
	}
	return depsByRefOrName
}

func aiGatewayConsumerGroupConsumerCreateDependencies(
	group resources.AIGatewayConsumerGroupResource,
	consumerCreateDepsByRefOrName map[string]string,
) []string {
	consumers, managesConsumers, err := group.ConsumerNames()
	if err != nil || !managesConsumers {
		return nil
	}

	var deps []string
	for _, consumer := range consumers {
		ref := consumer
		if parsedRef, _, ok := tags.ParseRefPlaceholder(consumer); ok {
			ref = parsedRef
		}
		if dep := consumerCreateDepsByRefOrName[ref]; dep != "" {
			deps = appendDependsOn(deps, dep)
		}
	}
	return deps
}

func (p *Planner) resolveAIGatewayConsumerGroupConsumerRefs(payload map[string]any) []string {
	consumers, ok := stringSliceFromValue(payload[FieldConsumers])
	if !ok {
		return nil
	}
	for i, consumer := range consumers {
		ref, field, ok := tags.ParseRefPlaceholder(consumer)
		if !ok {
			continue
		}
		if field != FieldName {
			continue
		}
		if resource := p.resources.GetAIGatewayConsumerByRef(ref); resource != nil && resource.Name != "" {
			consumers[i] = resource.Name
		}
	}
	consumers = normalizedAIGatewayConsumerGroupConsumers(consumers)
	payload[FieldConsumers] = consumers
	return consumers
}

func aiGatewayConsumerNamesFromState(consumers []state.AIGatewayConsumer) []string {
	names := make([]string, 0, len(consumers))
	for _, consumer := range consumers {
		if name := resources.AIGatewayConsumerName(consumer.AIGatewayConsumer); name != "" {
			names = append(names, name)
		}
	}
	return normalizedAIGatewayConsumerGroupConsumers(names)
}

func normalizedAIGatewayConsumerGroupConsumers(consumers []string) []string {
	normalized := make([]string, 0, len(consumers))
	seen := make(map[string]struct{}, len(consumers))
	for _, consumer := range consumers {
		if consumer == "" {
			continue
		}
		if _, ok := seen[consumer]; ok {
			continue
		}
		seen[consumer] = struct{}{}
		normalized = append(normalized, consumer)
	}
	slices.Sort(normalized)
	return normalized
}
