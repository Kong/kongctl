package planner

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
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

	currentServers, err := p.listAIGatewayMCPServers(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway MCP Servers for gateway %s: %w", gatewayID, err)
	}

	dependencies := func(server resources.AIGatewayMCPServerResource) []string {
		return aiGatewayMCPServerCreateDependencies(
			server, policyCreateDepsByName,
			aiGatewayMCPServerCreateDependenciesByName(plan, namespace, gatewayRef),
		)
	}
	return reconcileNameMatchedChildren(p, ResourceTypeAIGatewayMCPServer,
		orderAIGatewayMCPServersForPlanning(desired), currentServers,
		nameMatchedChildOperations[resources.AIGatewayMCPServerResource, state.AIGatewayMCPServer]{
			desiredName: func(desired resources.AIGatewayMCPServerResource) string { return desired.Name() },
			currentName: func(current state.AIGatewayMCPServer) string {
				return resources.AIGatewayMCPServerName(current.AIGatewayMCPServer)
			},
			fetch: func(_ resources.AIGatewayMCPServerResource, current state.AIGatewayMCPServer) (
				*state.AIGatewayMCPServer, error,
			) {
				id := resources.AIGatewayMCPServerID(current.AIGatewayMCPServer)
				full, err := p.client.GetAIGatewayMCPServer(ctx, gatewayID, id)
				if err != nil {
					return nil, fmt.Errorf("failed to get AI Gateway MCP Server %s: %w", id, err)
				}
				return full, nil
			},
			diff: p.shouldUpdateAIGatewayMCPServer,
			create: func(desired resources.AIGatewayMCPServerResource) {
				p.planAIGatewayMCPServerCreate(
					namespace, gatewayRef, gatewayName, gatewayID, desired, dependencies(desired), plan,
				)
			},
			update: func(current state.AIGatewayMCPServer, desired resources.AIGatewayMCPServerResource,
				fields map[string]any, changed map[string]FieldChange,
			) {
				p.planAIGatewayMCPServerUpdate(namespace, gatewayRef, gatewayID,
					resources.AIGatewayMCPServerID(current.AIGatewayMCPServer),
					desired, fields, changed, dependencies(desired), plan)
			},
			protected: func(current state.AIGatewayMCPServer) bool {
				return labels.IsProtectedResource(current.NormalizedLabels)
			},
			remove: func(current state.AIGatewayMCPServer) {
				p.planAIGatewayMCPServerDelete(namespace, gatewayRef, gatewayID,
					resources.AIGatewayMCPServerID(current.AIGatewayMCPServer),
					resources.AIGatewayMCPServerName(current.AIGatewayMCPServer), plan)
			},
			pruneOrder: orderCurrentAIGatewayMCPServersForDeletion,
		}, plan)
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
	for _, server := range orderAIGatewayMCPServersForPlanning(servers) {
		serverDependsOn := slices.Clone(dependsOn)
		for _, dep := range aiGatewayMCPServerCreateDependencies(
			server,
			policyCreateDepsByName,
			aiGatewayMCPServerCreateDependenciesByName(plan, namespace, gatewayRef),
		) {
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
	currentCompare = scrubAIGatewayUpstreamWriteOnlyFields(currentCompare).(map[string]any)
	desiredCompare = scrubAIGatewayUpstreamWriteOnlyFields(desiredCompare).(map[string]any)
	updateFields := clonePayloadMap(desiredPayload)
	pruneDefaultAIGatewayMCPServerAccessMissingFromPeer(currentCompare, desiredCompare)
	pruneDefaultAIGatewayMCPServerAccessMissingFromPeer(desiredCompare, currentCompare)
	currentCompare, desiredCompare = normalizeAIGatewayPolicyReferencesForComparison(
		currentCompare,
		desiredCompare,
		p.resources,
	)
	currentCompare, desiredCompare = normalizeAIGatewayAuthStrategyReferencesForComparison(
		currentCompare,
		desiredCompare,
		p.resources,
	)

	changedFields := diffAIGatewayPayloads(currentPayload, desiredPayload, currentCompare, desiredCompare)
	if len(changedFields) == 0 {
		return false, nil, nil, nil
	}

	return true, updateFields, changedFields, nil
}

func pruneDefaultAIGatewayMCPServerAccessMissingFromPeer(payload map[string]any, peer map[string]any) {
	if _, peerHasAccess := peer[FieldAccess]; peerHasAccess {
		return
	}
	access, ok := payload[FieldAccess].(map[string]any)
	if !ok {
		return
	}
	for key, value := range access {
		switch key {
		case FieldACLAttributeType:
			if !stringValueEqual(value, "consumer") {
				return
			}
		case FieldACLS, FieldDefaultToolACLS:
			if !emptyAIGatewayMCPACLs(value) {
				return
			}
		default:
			return
		}
	}
	delete(payload, FieldAccess)
}

func emptyAIGatewayMCPACLs(value any) bool {
	acls, ok := value.(map[string]any)
	if !ok {
		return false
	}
	for key, entries := range acls {
		if key != "allow" && key != "deny" {
			return false
		}
		if !emptySliceValue(entries) {
			return false
		}
	}
	return true
}

func aiGatewayMCPServerCreateDependencies(
	server resources.AIGatewayMCPServerResource,
	policyCreateDepsByName map[string]string,
	serverCreateDepsByName map[string]string,
) []string {
	payload, err := server.MutablePayloadMap()
	if err != nil {
		return nil
	}

	deps := aiGatewayPolicyReferenceDependencies(payload, policyCreateDepsByName)
	for _, source := range aiGatewayMCPServerSources(payload) {
		if dep := serverCreateDepsByName[source]; dep != "" {
			deps = appendDependsOn(deps, dep)
		}
	}
	return deps
}

func aiGatewayMCPServerCreateDependenciesByName(
	plan *Plan,
	namespace string,
	gatewayRef string,
) map[string]string {
	depsByName := make(map[string]string)
	for _, change := range plan.Changes {
		if change.Action != ActionCreate ||
			change.ResourceType != ResourceTypeAIGatewayMCPServer ||
			change.Namespace != namespace ||
			!aiGatewayChildChangeMatchesParent(change, gatewayRef) {
			continue
		}

		name, ok := change.Fields[FieldName].(string)
		if ok && name != "" {
			depsByName[name] = change.ID
		}
	}
	return depsByName
}

func orderAIGatewayMCPServersForPlanning(
	servers []resources.AIGatewayMCPServerResource,
) []resources.AIGatewayMCPServerResource {
	ordered := make([]resources.AIGatewayMCPServerResource, 0, len(servers))
	for _, server := range servers {
		payload, err := server.MutablePayloadMap()
		if err == nil && len(aiGatewayMCPServerSources(payload)) == 0 {
			ordered = append(ordered, server)
		}
	}
	for _, server := range servers {
		payload, err := server.MutablePayloadMap()
		if err != nil || len(aiGatewayMCPServerSources(payload)) > 0 {
			ordered = append(ordered, server)
		}
	}
	return ordered
}

func aiGatewayMCPServerSources(payload map[string]any) []string {
	values, ok := payload[FieldSources].([]any)
	if !ok {
		return nil
	}

	sources := make([]string, 0, len(values))
	for _, value := range values {
		if source, ok := value.(string); ok && source != "" {
			sources = append(sources, source)
		}
	}
	return sources
}

func orderCurrentAIGatewayMCPServersForDeletion(servers []state.AIGatewayMCPServer) []state.AIGatewayMCPServer {
	ordered := make([]state.AIGatewayMCPServer, 0, len(servers))
	for _, server := range servers {
		payload, err := resources.AIGatewayMCPServerMutablePayloadMap(server.AIGatewayMCPServer)
		if err == nil && len(aiGatewayMCPServerSources(payload)) > 0 {
			ordered = append(ordered, server)
		}
	}
	for _, server := range servers {
		payload, err := resources.AIGatewayMCPServerMutablePayloadMap(server.AIGatewayMCPServer)
		if err != nil || len(aiGatewayMCPServerSources(payload)) == 0 {
			ordered = append(ordered, server)
		}
	}
	return ordered
}
