package planner

import (
	"context"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/declarative/tags"
)

// Remote identity wins over manifest aliases. Unknown IDs must not conflate
// two gateways which merely share a display name.
func aiGatewayChangeMatchesGateway(change PlannedChange, ref, id string) bool {
	if id != "" {
		if change.Parent != nil && change.Parent.ID != "" {
			return change.Parent.ID == id
		}
		if parent, ok := change.References[FieldAIGatewayID]; ok && parent.ID != "" {
			return parent.ID == id
		}
	}
	return aiGatewayChildChangeMatchesParent(change, ref)
}

func aiGatewayChangeGateway(change PlannedChange) (string, string) {
	if change.Parent != nil {
		return change.Parent.Ref, change.Parent.ID
	}
	parent := change.References[FieldAIGatewayID]
	return parent.Ref, parent.ID
}

// Run only after all namespaces and aliases have contributed their changes.
func (p *Planner) resolveFinalAIGatewayDependencies(ctx context.Context, plan *Plan) error {
	seen := make(map[string]bool)
	for _, change := range plan.Changes {
		if !strings.HasPrefix(change.ResourceType, ResourceTypeAIGateway+"_") {
			continue
		}
		ref, id := aiGatewayChangeGateway(change)
		key := "id:" + id
		if id == "" {
			key = "ref:" + ref
		}
		if (id == "" && ref == "") || seen[key] {
			continue
		}
		seen[key] = true
		addAIGatewayDatastoreDependencies(plan, ref, id)
		addAIGatewayOutboundTLSDependencies(plan, ref, id)
		if err := p.resolveAIGatewayReferenceDeletes(ctx, "", ref, id, plan); err != nil {
			return err
		}
	}
	// Name-based API relationships need edges even when they are plain strings,
	// rather than payload reference expressions. Replacement creates participate.
	for i := range plan.Changes {
		user := &plan.Changes[i]
		if user.Action == ActionDelete {
			continue
		}
		ref, id := aiGatewayChangeGateway(*user)
		if ref == "" && id == "" {
			continue
		}
		for _, target := range plan.Changes {
			if !aiGatewayChangeMatchesGateway(target, ref, id) {
				continue
			}
			if user.Action == ActionCreate && target.Action == ActionDelete && target.ResourceType == user.ResourceType {
				name, named := user.Fields[FieldName].(string)
				if named && name != "" && target.Fields[FieldName] == name {
					user.DependsOn = appendDependsOn(user.DependsOn, target.ID)
				}
			}
			if target.Action != ActionCreate {
				continue
			}
			name, _ := target.Fields[FieldName].(string)
			references := func(values []string) bool {
				for _, value := range values {
					if ref, selector, ok := tags.ParseRefPlaceholder(value); ok && ref == target.ResourceRef && selector == FieldName {
						return true
					}
					if canonicalAIGatewayReference(value, nil) == name {
						return true
					}
				}
				return false
			}
			uses := false
			switch target.ResourceType {
			case ResourceTypeAIGatewayVault,
				ResourceTypeAIGatewayCACertificate,
				ResourceTypeAIGatewaySNI:
				uses = references(publicChildReferences(target.ResourceType, user.Fields))
				if target.ResourceType == ResourceTypeAIGatewaySNI {
					hostname, _ := target.Fields[FieldHostname].(string)
					uses = uses ||
						slices.Contains(
							publicChildReferences(target.ResourceType, user.Fields),
							hostname,
						)
				}
			case ResourceTypeAIGatewayProvider:
				uses = user.ResourceType == ResourceTypeAIGatewayModel && references(aiGatewayModelProviderNames(user.Fields))
			case ResourceTypeAIGatewayPolicy:
				uses = aiGatewayResourceUsesPolicies(user.ResourceType) &&
					references(stringSliceFromField(user.Fields,
						FieldPolicies))
			case ResourceTypeAIGatewayAuthStrategy:
				uses = aiGatewayResourceUsesAuthStrategies(user.ResourceType) && references(aiGatewayAuthStrategies(user.Fields))
			case ResourceTypeAIGatewayCertificate:
				value, _ := user.Fields[FieldCertificate].(string)
				uses = user.ResourceType == ResourceTypeAIGatewaySNI && references([]string{value})
			case ResourceTypeAIGatewayConsumer:
				uses = user.ResourceType == ResourceTypeAIGatewayConsumerGroup &&
					references(stringSliceFromField(user.Fields, FieldConsumers))
			case ResourceTypeAIGatewayMCPServer:
				uses = user.ResourceType == ResourceTypeAIGatewayMCPServer && references(aiGatewayMCPServerSources(user.Fields))
			}
			if uses && target.ID != user.ID {
				user.DependsOn = appendDependsOn(user.DependsOn, target.ID)
			}
		}
	}
	return nil
}
