package planner

import (
	"context"
	"fmt"
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util"
)

func (p *Planner) resolveAIGatewayMCPSourceDeletes(
	ctx context.Context, namespace, gatewayRef, gatewayID string, plan *Plan,
) error {
	references := func(deletion PlannedChange, values []string) bool {
		name, _ := deletion.Fields[FieldName].(string)
		return slices.Contains(values, name)
	}
	return resolveObservedReferenceDeletes(ctx, namespace, gatewayRef, plan, observedReferenceDeletePolicy{
		targetType: ResourceTypeAIGatewayMCPServer,
		usesTarget: func(kind string) bool { return kind == ResourceTypeAIGatewayMCPServer },
		observe: func(ctx context.Context) ([]observedReferenceUser, error) {
			servers, err := p.listAIGatewayMCPServers(ctx, gatewayID)
			if err != nil {
				return nil, fmt.Errorf("failed to inspect MCP sources in AI Gateway %q: %w", gatewayRef, err)
			}
			users := make([]observedReferenceUser, 0, len(servers))
			for _, server := range servers {
				payload, err := resources.AIGatewayMCPServerMutablePayloadMap(server.AIGatewayMCPServer)
				if err != nil {
					return nil, fmt.Errorf("failed to inspect MCP server sources in AI Gateway %q: %w", gatewayRef, err)
				}
				users = append(users, observedReferenceUser{
					ResourceTypeAIGatewayMCPServer,
					resources.AIGatewayMCPServerID(server.AIGatewayMCPServer),
					resources.AIGatewayMCPServerName(server.AIGatewayMCPServer),
					aiGatewayMCPServerSources(payload),
				})
			}
			return users, nil
		},
		observedReferences: references,
		plannedReferences: func(deletion, change PlannedChange) bool {
			return references(deletion, aiGatewayMCPServerSources(change.Fields))
		},
		updateDetaches: func(deletion, change PlannedChange) bool {
			// Listener sources are required in the SDK update payload. Absence
			// alone does not prove that an existing attachment will be removed.
			sources, valid := util.StringSliceFromAny(change.Fields[FieldSources])
			return valid && !references(deletion, sources)
		},
		conflict: func(deletion PlannedChange, user observedReferenceUser, planned bool) error {
			qualifier := ""
			if planned {
				qualifier = "planned "
			}
			return fmt.Errorf(
				"cannot delete AI Gateway MCP Server %q in gateway %q while %sMCP server %q still references it in sources",
				deletion.Fields[FieldName], gatewayRef, qualifier, user.name,
			)
		},
	})
}
