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
			p.planAIGatewayConsumerGroupCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredGroup, dependsOn, plan)
			continue
		}

		needsUpdate, updateFields, changedFields, err := p.shouldUpdateAIGatewayConsumerGroup(*fullGroup, desiredGroup)
		if err != nil {
			return err
		}
		if needsUpdate {
			p.planAIGatewayConsumerGroupUpdate(
				namespace,
				gatewayRef,
				gatewayID,
				groupID,
				desiredGroup,
				updateFields,
				changedFields,
				aiGatewayConsumerGroupPolicyCreateDependencies(
					desiredGroup,
					policyCreateDepsByRefOrName,
				),
				plan,
			)
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		for _, current := range currentGroups {
			groupID := resources.AIGatewayConsumerGroupID(current.AIGatewayConsumerGroup)
			groupName := resources.AIGatewayConsumerGroupName(current.AIGatewayConsumerGroup)
			if desiredKeys[groupID] || desiredKeys[groupName] {
				continue
			}
			p.planAIGatewayConsumerGroupDelete(gatewayRef, gatewayID, groupID, groupName, plan)
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
		groupDependsOn := append([]string{}, dependsOn...)
		for _, dep := range aiGatewayConsumerGroupPolicyCreateDependencies(group, policyCreateDepsByRefOrName) {
			groupDependsOn = appendDependsOn(groupDependsOn, dep)
		}
		p.planAIGatewayConsumerGroupCreate(namespace, gatewayRef, gatewayName, "", group, groupDependsOn, plan)
	}
}

func (p *Planner) planAIGatewayConsumerGroupCreate(
	namespace string,
	gatewayRef string,
	gatewayName string,
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
					FieldDisplayName: gatewayName,
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
