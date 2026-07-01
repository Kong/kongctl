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

func (p *Planner) planAIGatewayMCPServerChanges(
	ctx context.Context,
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	gatewayChangeID string,
	policyCreateDepsByName map[string]string,
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
		p.planAIGatewayMCPServerCreatesForNewGateway(
			namespace,
			gatewayRef,
			gatewayName,
			gatewayChangeID,
			policyCreateDepsByName,
			desired,
			plan,
		)
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
			dependsOn := aiGatewayMCPServerPolicyCreateDependencies(desiredServer, policyCreateDepsByName)
			p.planAIGatewayMCPServerCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredServer, dependsOn, plan)
			continue
		}

		serverID := resources.AIGatewayMCPServerID(current.AIGatewayMCPServer)
		fullServer, err := p.client.GetAIGatewayMCPServer(ctx, gatewayID, serverID)
		if err != nil {
			return fmt.Errorf("failed to get AI Gateway MCP Server %s: %w", serverID, err)
		}
		if fullServer == nil {
			dependsOn := aiGatewayMCPServerPolicyCreateDependencies(desiredServer, policyCreateDepsByName)
			p.planAIGatewayMCPServerCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredServer, dependsOn, plan)
			continue
		}

		needsUpdate, updateFields, changedFields, err := p.shouldUpdateAIGatewayMCPServer(*fullServer, desiredServer)
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
				aiGatewayMCPServerPolicyCreateDependencies(desiredServer, policyCreateDepsByName),
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
			p.planAIGatewayMCPServerDelete(namespace, gatewayRef, gatewayID, serverID, serverName, plan)
		}
	}

	return nil
}

func (p *Planner) planAIGatewayMCPServerCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	policyCreateDepsByName map[string]string,
	servers []resources.AIGatewayMCPServerResource,
	plan *Plan,
) {
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}
	for _, server := range servers {
		serverDependsOn := slices.Clone(dependsOn)
		for _, dep := range aiGatewayMCPServerPolicyCreateDependencies(server, policyCreateDepsByName) {
			serverDependsOn = appendDependsOn(serverDependsOn, dep)
		}
		p.planAIGatewayMCPServerCreate(namespace, gatewayRef, gatewayName, "", server, serverDependsOn, plan)
	}
}

func (p *Planner) planAIGatewayMCPServerCreate(
	namespace string,
	gatewayRef string,
	_ string,
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
					FieldName: gatewayRef,
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
	dependsOn []string,
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
		DependsOn:     dependsOn,
		Parent:        &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func (p *Planner) planAIGatewayMCPServerDelete(
	namespace string,
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
		Namespace:    namespace,
		Fields: map[string]any{
			FieldName: serverName,
		},
		Parent: &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func (p *Planner) shouldUpdateAIGatewayMCPServer(
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

func aiGatewayMCPServerPolicyCreateDependencies(
	server resources.AIGatewayMCPServerResource,
	policyCreateDepsByName map[string]string,
) []string {
	payload, err := server.MutablePayloadMap()
	if err != nil {
		return nil
	}
	return aiGatewayPolicyReferenceDependencies(payload, policyCreateDepsByName)
}
