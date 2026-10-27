package planner

import (
	"context"
	"fmt"
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func aiGatewayUsesOutboundTLS(kind string) bool {
	return kind == ResourceTypeAIGatewayAgent || kind == ResourceTypeAIGatewayMCPServer
}

func addAIGatewayOutboundTLSDependencies(plan *Plan, namespace, gatewayRef string) {
	for _, certificate := range plan.Changes {
		if certificate.Namespace != namespace ||
			!aiGatewayChildChangeMatchesParent(certificate, gatewayRef) ||
			certificate.Action != ActionCreate ||
			(certificate.ResourceType != ResourceTypeAIGatewayCertificate &&
				certificate.ResourceType != ResourceTypeAIGatewayCACertificate) {
			continue
		}
		for i := range plan.Changes {
			change := &plan.Changes[i]
			if change.Namespace == namespace &&
				aiGatewayChildChangeMatchesParent(*change, gatewayRef) &&
				aiGatewayUsesOutboundTLS(change.ResourceType) &&
				change.Action != ActionDelete {
				change.DependsOn = appendDependsOn(change.DependsOn, certificate.ID)
			}
		}
	}
}

func (p *Planner) resolveAIGatewayOutboundTLSDeletes(
	ctx context.Context,
	namespace, gatewayRef, gatewayID, kind, field string,
	plan *Plan,
) error {
	names := map[string]string{}
	if p.resources != nil {
		for _, resource := range p.resources.GetAIGatewayCertificatesForGateway(gatewayRef) {
			names[resource.Ref] = resource.Name
		}
		for _, resource := range p.resources.GetAIGatewayCACertificatesForGateway(gatewayRef) {
			names[resource.Ref] = resource.Name
		}
	}
	references := func(deletion PlannedChange, values []string) bool {
		name, _ := deletion.Fields[FieldName].(string)
		return slices.Contains(values, name) || slices.Contains(values, deletion.ResourceID)
	}
	configured := func(payload map[string]any) []string {
		config, _ := payload[FieldConfig].(map[string]any)
		values := stringSliceFromField(config, field)
		for i, value := range values {
			if ref, _, ok := tags.ParseRefPlaceholder(value); ok && names[ref] != "" {
				values[i] = names[ref]
			}
		}
		return values
	}
	return resolveObservedReferenceDeletes(
		ctx,
		namespace,
		gatewayRef,
		plan,
		observedReferenceDeletePolicy{
			targetType: kind, usesTarget: aiGatewayUsesOutboundTLS,
			observe: func(ctx context.Context) ([]observedReferenceUser, error) {
				agents, err := p.listAIGatewayAgents(ctx, gatewayID)
				if err != nil {
					return nil, err
				}
				servers, err := p.listAIGatewayMCPServers(ctx, gatewayID)
				if err != nil {
					return nil, err
				}
				var users []observedReferenceUser
				for _, agent := range agents {
					payload, err := resources.AIGatewayAgentMutablePayloadMap(agent.AIGatewayAgent)
					if err != nil {
						return nil, err
					}
					users = append(
						users,
						observedReferenceUser{
							ResourceTypeAIGatewayAgent,
							agent.ID,
							agent.Name,
							configured(payload),
						},
					)
				}
				for _, server := range servers {
					payload, err := resources.AIGatewayMCPServerMutablePayloadMap(
						server.AIGatewayMCPServer,
					)
					if err != nil {
						return nil, err
					}
					users = append(
						users,
						observedReferenceUser{
							ResourceTypeAIGatewayMCPServer,
							resources.AIGatewayMCPServerID(server.AIGatewayMCPServer),
							resources.AIGatewayMCPServerName(server.AIGatewayMCPServer),
							configured(payload),
						},
					)
				}
				return users, nil
			},
			observedReferences: references,
			plannedReferences: func(deletion, change PlannedChange) bool {
				return references(deletion, configured(normalizeAIGatewayJSONMap(change.Fields)))
			},
			updateDetaches: func(deletion, change PlannedChange) bool {
				if _, changed := change.ChangedFields[FieldConfig]; !changed {
					return false
				}
				return !references(deletion, configured(normalizeAIGatewayJSONMap(change.Fields)))
			},
			conflict: func(deletion PlannedChange, user observedReferenceUser, _ bool) error {
				return fmt.Errorf(
					"cannot delete %s %q while %s %q still references it",
					kind,
					deletion.Fields[FieldName],
					user.resourceType,
					user.name,
				)
			},
		},
	)
}
