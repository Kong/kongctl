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

func (p *Planner) planAIGatewayMCPServerChanges(
	ctx context.Context,
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	gatewayChangeID string,
	desired []resources.AIGatewayMCPServerResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning AI Gateway MCP Server changes",
		slog.String("gateway_ref", gatewayRef),
		slog.String("gateway_id", gatewayID),
		slog.String("gateway_change_id", gatewayChangeID),
		slog.Int("desired_count", len(desired)),
	)

	if gatewayID == "" {
		p.planAIGatewayMCPServerCreatesForNewGateway(namespace, gatewayRef, gatewayName, gatewayChangeID, desired, plan)
		return nil
	}

	currentServers, err := p.client.ListAIGatewayMCPServers(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway MCP Servers for gateway %s: %w", gatewayID, err)
	}

	currentByID, currentByName := indexAIGatewayMCPServers(currentServers)
	desiredKeys := make(map[string]bool)

	for _, desiredServer := range desired {
		current, exists := matchCurrentAIGatewayMCPServer(desiredServer, currentByID, currentByName)
		desiredKeys[desiredServer.Name()] = true
		if id := aiGatewayMCPServerDesiredID(desiredServer); id != "" {
			desiredKeys[id] = true
		}

		if !exists {
			p.planAIGatewayMCPServerCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredServer, nil, plan)
			continue
		}

		serverID := resources.AIGatewayMCPServerID(current.AIGatewayMCPServer)
		fullServer, err := p.client.GetAIGatewayMCPServer(ctx, gatewayID, serverID)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway MCP Server %s: %w", serverID, err)
		}
		if fullServer == nil {
			p.planAIGatewayMCPServerCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredServer, nil, plan)
			continue
		}

		needsUpdate, updateFields, changedFields, err := shouldUpdateAIGatewayMCPServer(*fullServer, desiredServer)
		if err != nil {
			return err
		}
		if needsUpdate {
			p.planAIGatewayMCPServerUpdate(
				namespace,
				gatewayRef,
				gatewayID,
				serverID,
				desiredServer,
				updateFields,
				changedFields,
				plan,
			)
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		for _, current := range currentServers {
			serverID := resources.AIGatewayMCPServerID(current.AIGatewayMCPServer)
			serverName := resources.AIGatewayMCPServerName(current.AIGatewayMCPServer)
			if desiredKeys[serverID] || desiredKeys[serverName] {
				continue
			}
			p.planAIGatewayMCPServerDelete(gatewayRef, gatewayID, serverID, serverName, plan)
		}
	}

	return nil
}

func (p *Planner) planAIGatewayMCPServerCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	servers []resources.AIGatewayMCPServerResource,
	plan *Plan,
) {
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}
	for _, server := range servers {
		p.planAIGatewayMCPServerCreate(namespace, gatewayRef, gatewayName, "", server, dependsOn, plan)
	}
}

func (p *Planner) planAIGatewayMCPServerCreate(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	server resources.AIGatewayMCPServerResource,
	dependsOn []string,
	plan *Plan,
) {
	fields, err := server.MutablePayloadMap()
	if err != nil {
		plan.AddWarning(server.GetRef(), fmt.Sprintf("failed to build AI Gateway MCP Server create payload: %s", err))
		return
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayMCPServer, server.Ref),
		ResourceType: ResourceTypeAIGatewayMCPServer,
		ResourceRef:  server.Ref,
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

func (p *Planner) planAIGatewayMCPServerUpdate(
	namespace string,
	gatewayRef string,
	gatewayID string,
	serverID string,
	server resources.AIGatewayMCPServerResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeAIGatewayMCPServer, server.Ref),
		ResourceType:  ResourceTypeAIGatewayMCPServer,
		ResourceRef:   server.Ref,
		ResourceID:    serverID,
		Action:        ActionUpdate,
		Fields:        updateFields,
		ChangedFields: changedFields,
		Namespace:     namespace,
		Parent:        &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func (p *Planner) planAIGatewayMCPServerDelete(
	gatewayRef string,
	gatewayID string,
	serverID string,
	serverName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGatewayMCPServer, serverName),
		ResourceType: ResourceTypeAIGatewayMCPServer,
		ResourceRef:  serverName,
		ResourceID:   serverID,
		Action:       ActionDelete,
		Fields: map[string]any{
			FieldName: serverName,
		},
		Parent: &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func shouldUpdateAIGatewayMCPServer(
	current state.AIGatewayMCPServer,
	desired resources.AIGatewayMCPServerResource,
) (bool, map[string]any, map[string]FieldChange, error) {
	currentPayload, err := resources.AIGatewayMCPServerMutablePayloadMap(current.AIGatewayMCPServer)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to normalize current AI Gateway MCP Server: %w", err)
	}
	desiredPayload, err := desired.MutablePayloadMap()
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to normalize desired AI Gateway MCP Server %q: %w", desired.Ref, err)
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

func indexAIGatewayMCPServers(
	servers []state.AIGatewayMCPServer,
) (map[string]state.AIGatewayMCPServer, map[string]state.AIGatewayMCPServer) {
	byID := make(map[string]state.AIGatewayMCPServer)
	byName := make(map[string]state.AIGatewayMCPServer)
	for _, server := range servers {
		if id := resources.AIGatewayMCPServerID(server.AIGatewayMCPServer); id != "" {
			byID[id] = server
		}
		if name := resources.AIGatewayMCPServerName(server.AIGatewayMCPServer); name != "" {
			byName[name] = server
		}
	}
	return byID, byName
}

func matchCurrentAIGatewayMCPServer(
	desired resources.AIGatewayMCPServerResource,
	currentByID map[string]state.AIGatewayMCPServer,
	currentByName map[string]state.AIGatewayMCPServer,
) (state.AIGatewayMCPServer, bool) {
	if id := aiGatewayMCPServerDesiredID(desired); id != "" {
		current, exists := currentByID[id]
		return current, exists
	}
	current, exists := currentByName[desired.Name()]
	return current, exists
}

func aiGatewayMCPServerDesiredID(desired resources.AIGatewayMCPServerResource) string {
	if id := desired.GetKonnectID(); id != "" {
		return id
	}
	if util.IsValidUUID(desired.Ref) {
		return desired.Ref
	}
	return ""
}
