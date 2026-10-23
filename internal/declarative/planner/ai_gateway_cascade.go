package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

type aiGatewayDescendant struct {
	name, namespace string
	protected       bool
}

func cascadePublicMetadata[T any](values []T, err error) ([]aiGatewayDescendant, error) {
	if err != nil {
		return nil, err
	}
	result := make([]aiGatewayDescendant, 0, len(values))
	for _, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		var public struct {
			Name           string            `json:"name"`
			ManagedBy      map[string]string `json:"managed_by"`
			StateManagedBy map[string]string `json:"ManagedBy"`
			Labels         map[string]string `json:"labels"`
		}
		if err := json.Unmarshal(data, &public); err != nil {
			return nil, err
		}
		metadata := public.ManagedBy
		if metadata == nil {
			metadata = public.StateManagedBy
		}
		result = append(result,
			aiGatewayDescendant{
				public.Name,
				metadata[resources.ChildNamespaceKey],
				metadata[resources.ChildProtectedKey] == "true" || labels.IsProtectedResource(public.Labels),
			})
	}
	return result, nil
}

func addAIGatewayCascadeWarnings(plan *Plan, gateway PlannedChange, kind string, children []aiGatewayDescendant) {
	byNamespace := make(map[string][]string)
	protected := make(map[string][]string)
	for _, child := range children {
		namespace := child.namespace
		if namespace == "" {
			namespace = "unowned"
		}
		byNamespace[namespace] = append(byNamespace[namespace], child.name)
		if child.protected {
			protected[namespace] = append(protected[namespace], child.name)
		}
	}
	for _, namespace := range slices.Sorted(maps.Keys(byNamespace)) {
		names := byNamespace[namespace]
		slices.Sort(names)
		slices.Sort(protected[namespace])
		plan.AddWarning(gateway.ResourceRef,
			fmt.Sprintf("Deleting AI Gateway %q cascades to %s: namespace %q, %d children %q; "+
				"protected descendants %q (warn only)",
				gateway.ResourceRef,
				kind,
				namespace,
				len(names),
				names,
				protected[namespace]))
	}
}

func (p *Planner) planAIGatewayCascadeWarnings(ctx context.Context, plan *Plan) error {
	for _, gateway := range plan.Changes {
		if gateway.ResourceType != ResourceTypeAIGateway || gateway.Action != ActionDelete {
			continue
		}
		for _, family := range p.aiGatewayCascadeFamilies() {
			children, err := family.observe(ctx, gateway.ResourceID)
			if err != nil {
				if state.IsAPIClientError(err) {
					plan.AddWarning(gateway.ResourceRef,
						fmt.Sprintf("Deleting AI Gateway %q also cascades to %s; "+
							"descendant ownership/protection could not be observed: %s",
							gateway.ResourceRef,
							family.kind,
							err))
					continue
				}
				return fmt.Errorf("inspect %s before AI Gateway cascade deletion: %w", family.kind, err)
			}
			addAIGatewayCascadeWarnings(plan, gateway, family.kind, children)
		}
	}
	return nil
}

type aiGatewayCascadeFamily struct {
	kind    string
	observe func(context.Context, string) ([]aiGatewayDescendant, error)
}

func (p *Planner) aiGatewayCascadeFamilies() []aiGatewayCascadeFamily {
	return []aiGatewayCascadeFamily{
		{ResourceTypeAIGatewayProvider, func(ctx context.Context, id string) ([]aiGatewayDescendant, error) {
			return cascadePublicMetadata(p.listAIGatewayProviders(ctx, id))
		}},
		{ResourceTypeAIGatewayAuthStrategy, func(ctx context.Context, id string) ([]aiGatewayDescendant, error) {
			return cascadePublicMetadata(p.listAIGatewayAuthStrategies(ctx, id))
		}},
		{ResourceTypeAIGatewayPolicy, func(ctx context.Context, id string) ([]aiGatewayDescendant, error) {
			return cascadePublicMetadata(p.listAIGatewayPolicies(ctx, id))
		}},
		{ResourceTypeAIGatewayCustomPolicy, func(ctx context.Context, id string) ([]aiGatewayDescendant, error) {
			return cascadePublicMetadata(p.listAIGatewayCustomPolicies(ctx, id))
		}},
		{ResourceTypeAIGatewayAgent, func(ctx context.Context, id string) ([]aiGatewayDescendant, error) {
			return cascadePublicMetadata(p.listAIGatewayAgents(ctx, id))
		}},
		{ResourceTypeAIGatewayMCPServer, func(ctx context.Context, id string) ([]aiGatewayDescendant, error) {
			return cascadePublicMetadata(p.listAIGatewayMCPServers(ctx, id))
		}},
		{ResourceTypeAIGatewayModel, func(ctx context.Context, id string) ([]aiGatewayDescendant, error) {
			return cascadePublicMetadata(p.listAIGatewayModels(ctx, id))
		}},
		{ResourceTypeAIGatewayConsumer, func(ctx context.Context, id string) ([]aiGatewayDescendant, error) {
			return cascadePublicMetadata(p.listAIGatewayConsumers(ctx, id))
		}},
		{ResourceTypeAIGatewayConsumerGroup, func(ctx context.Context, id string) ([]aiGatewayDescendant, error) {
			return cascadePublicMetadata(p.listAIGatewayConsumerGroups(ctx, id))
		}},
		{ResourceTypeAIGatewayVault, func(ctx context.Context, id string) ([]aiGatewayDescendant, error) {
			return cascadePublicMetadata(p.listAIGatewayVaults(ctx, id))
		}},
		{ResourceTypeAIGatewayCertificate, func(ctx context.Context, id string) ([]aiGatewayDescendant, error) {
			return cascadePublicMetadata(p.listAIGatewayCertificates(ctx, id))
		}},
		{ResourceTypeAIGatewayCACertificate, func(ctx context.Context, id string) ([]aiGatewayDescendant, error) {
			return cascadePublicMetadata(p.listAIGatewayCACertificates(ctx, id))
		}},
		{ResourceTypeAIGatewaySNI, func(ctx context.Context, id string) ([]aiGatewayDescendant, error) {
			return cascadePublicMetadata(p.listAIGatewaySNIs(ctx, id))
		}},
	}
}
