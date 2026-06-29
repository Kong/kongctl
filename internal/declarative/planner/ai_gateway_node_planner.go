package planner

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) planAIGatewayNodeChanges(
	ctx context.Context,
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	gatewayChangeID string,
	desired []resources.AIGatewayNodeResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning AI Gateway Node changes",
		slog.String("gateway_ref", gatewayRef),
		slog.String("gateway_id", gatewayID),
		slog.String("gateway_change_id", gatewayChangeID),
		slog.Int("desired_count", len(desired)),
	)

	if gatewayID == "" {
		p.planAIGatewayNodeCreatesForNewGateway(namespace, gatewayRef, gatewayName, gatewayChangeID, desired, plan)
		return nil
	}

	currentNodes, err := p.client.ListAIGatewayNodes(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway Nodes for gateway %s: %w", gatewayID, err)
	}

	currentByID := indexAIGatewayNodes(currentNodes)
	desiredIDs := make(map[string]bool)

	for _, desiredNode := range desired {
		desiredIDs[desiredNode.ID] = true
		current, exists := currentByID[desiredNode.ID]

		if !exists {
			p.planAIGatewayNodeCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredNode, nil, plan)
			continue
		}

		nodeID := resources.AIGatewayNodeID(current.AIGatewayDataPlaneNode)
		fullNode, err := p.client.GetAIGatewayNode(ctx, gatewayID, nodeID)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway Node %s: %w", nodeID, err)
		}
		if fullNode == nil {
			p.planAIGatewayNodeCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredNode, nil, plan)
			continue
		}

		needsUpdate, updateFields, changedFields, err := shouldUpdateAIGatewayNode(*fullNode, desiredNode)
		if err != nil {
			return err
		}
		if needsUpdate {
			p.planAIGatewayNodeUpdate(
				namespace,
				gatewayRef,
				gatewayID,
				nodeID,
				desiredNode,
				updateFields,
				changedFields,
				nil,
				plan,
			)
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		for _, current := range currentNodes {
			nodeID := resources.AIGatewayNodeID(current.AIGatewayDataPlaneNode)
			if desiredIDs[nodeID] {
				continue
			}
			p.planAIGatewayNodeDelete(gatewayRef, gatewayID, nodeID, plan)
		}
	}

	return nil
}

func (p *Planner) planAIGatewayNodeCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	nodes []resources.AIGatewayNodeResource,
	plan *Plan,
) {
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}
	for _, node := range nodes {
		p.planAIGatewayNodeCreate(namespace, gatewayRef, gatewayName, "", node, dependsOn, plan)
	}
}

func (p *Planner) planAIGatewayNodeCreate(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	node resources.AIGatewayNodeResource,
	dependsOn []string,
	plan *Plan,
) {
	fields, err := node.MutablePayloadMap()
	if err != nil {
		plan.AddWarning(node.GetRef(), fmt.Sprintf("failed to build AI Gateway Node create payload: %s", err))
		return
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayNode, node.Ref),
		ResourceType: ResourceTypeAIGatewayNode,
		ResourceRef:  node.Ref,
		ResourceID:   node.ID,
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

func (p *Planner) planAIGatewayNodeUpdate(
	namespace string,
	gatewayRef string,
	gatewayID string,
	nodeID string,
	node resources.AIGatewayNodeResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	dependsOn []string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeAIGatewayNode, node.Ref),
		ResourceType:  ResourceTypeAIGatewayNode,
		ResourceRef:   node.Ref,
		ResourceID:    nodeID,
		Action:        ActionUpdate,
		Fields:        updateFields,
		ChangedFields: changedFields,
		Namespace:     namespace,
		DependsOn:     dependsOn,
		Parent:        &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func (p *Planner) planAIGatewayNodeDelete(
	gatewayRef string,
	gatewayID string,
	nodeID string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGatewayNode, nodeID),
		ResourceType: ResourceTypeAIGatewayNode,
		ResourceRef:  nodeID,
		ResourceID:   nodeID,
		Action:       ActionDelete,
		Fields: map[string]any{
			FieldID: nodeID,
		},
		Parent: &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func shouldUpdateAIGatewayNode(
	current state.AIGatewayNode,
	desired resources.AIGatewayNodeResource,
) (bool, map[string]any, map[string]FieldChange, error) {
	currentPayload, err := resources.AIGatewayNodeMutablePayloadMap(current.AIGatewayDataPlaneNode)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to normalize current AI Gateway Node: %w", err)
	}
	desiredPayload, err := desired.MutablePayloadMap()
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to normalize desired AI Gateway Node %q: %w", desired.Ref, err)
	}

	changedFields := make(map[string]FieldChange)
	keys := make(map[string]struct{}, len(currentPayload)+len(desiredPayload))
	for key := range currentPayload {
		keys[key] = struct{}{}
	}
	for key := range desiredPayload {
		keys[key] = struct{}{}
	}
	for key := range keys {
		if !reflect.DeepEqual(currentPayload[key], desiredPayload[key]) {
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

func indexAIGatewayNodes(nodes []state.AIGatewayNode) map[string]state.AIGatewayNode {
	byID := make(map[string]state.AIGatewayNode)
	for _, node := range nodes {
		if id := resources.AIGatewayNodeID(node.AIGatewayDataPlaneNode); id != "" {
			byID[id] = node
		}
	}
	return byID
}
