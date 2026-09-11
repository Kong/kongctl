package planner

import (
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/state"
)

type planningResourceCache struct {
	managedControlPlanes             observationCache[state.ControlPlane]
	managedEventGatewayControlPlanes observationCache[state.EventGatewayControlPlane]
	managedPortals                   observationCache[state.Portal]
	managedAuthStrategies            observationCache[state.ApplicationAuthStrategy]
	managedDCRProviders              observationCache[state.DCRProvider]
	managedAPIs                      observationCache[state.API]
	managedCatalogServices           observationCache[state.CatalogService]
	managedAIGateways                observationCache[state.AIGateway]
	managedDashboards                observationCache[state.Dashboard]
	managedOrganizationTeams         observationCache[state.OrganizationTeam]

	portalTeamsByPortalID             map[string][]state.PortalTeam
	portalIdentityProvidersByPortalID map[string][]state.PortalIdentityProvider
	portalTeamGroupMappingsByPortalID map[string][]state.PortalTeamGroupMapping
}

func newPlanningResourceCache() *planningResourceCache {
	return &planningResourceCache{
		managedControlPlanes:              newObservationCache[state.ControlPlane](),
		managedEventGatewayControlPlanes:  newObservationCache[state.EventGatewayControlPlane](),
		managedPortals:                    newObservationCache[state.Portal](),
		managedAuthStrategies:             newObservationCache[state.ApplicationAuthStrategy](),
		managedDCRProviders:               newObservationCache[state.DCRProvider](),
		managedAPIs:                       newObservationCache[state.API](),
		managedCatalogServices:            newObservationCache[state.CatalogService](),
		managedAIGateways:                 newObservationCache[state.AIGateway](),
		managedDashboards:                 newObservationCache[state.Dashboard](),
		managedOrganizationTeams:          newObservationCache[state.OrganizationTeam](),
		portalTeamsByPortalID:             make(map[string][]state.PortalTeam),
		portalIdentityProvidersByPortalID: make(map[string][]state.PortalIdentityProvider),
		portalTeamGroupMappingsByPortalID: make(map[string][]state.PortalTeamGroupMapping),
	}
}

func (p *Planner) listManagedControlPlanes(ctx context.Context, namespaces []string) ([]state.ControlPlane, error) {
	var cache *observationCache[state.ControlPlane]
	if p.resourceCache != nil {
		cache = &p.resourceCache.managedControlPlanes
	}
	return cache.list(ctx, namespaces, p.namespaceFanout, p.client.ListManagedControlPlanes,
		func(controlPlane state.ControlPlane) string {
			return controlPlane.NormalizedLabels[labels.NamespaceKey]
		})
}

func (p *Planner) listManagedPortals(ctx context.Context, namespaces []string) ([]state.Portal, error) {
	var cache *observationCache[state.Portal]
	if p.resourceCache != nil {
		cache = &p.resourceCache.managedPortals
	}
	return cache.list(ctx, namespaces, p.namespaceFanout, p.client.ListManagedPortals,
		func(portal state.Portal) string { return portal.NormalizedLabels[labels.NamespaceKey] })
}

func (p *Planner) listManagedAuthStrategies(
	ctx context.Context,
	namespaces []string,
) ([]state.ApplicationAuthStrategy, error) {
	var cache *observationCache[state.ApplicationAuthStrategy]
	if p.resourceCache != nil {
		cache = &p.resourceCache.managedAuthStrategies
	}
	return cache.list(ctx, namespaces, p.namespaceFanout, p.client.ListManagedAuthStrategies,
		func(strategy state.ApplicationAuthStrategy) string {
			return strategy.NormalizedLabels[labels.NamespaceKey]
		})
}

func (p *Planner) listManagedDCRProviders(ctx context.Context, namespaces []string) ([]state.DCRProvider, error) {
	var cache *observationCache[state.DCRProvider]
	if p.resourceCache != nil {
		cache = &p.resourceCache.managedDCRProviders
	}
	return cache.list(ctx, namespaces, p.namespaceFanout, p.client.ListManagedDCRProviders,
		func(provider state.DCRProvider) string { return provider.NormalizedLabels[labels.NamespaceKey] })
}

func (p *Planner) listManagedAPIs(ctx context.Context, namespaces []string) ([]state.API, error) {
	var cache *observationCache[state.API]
	if p.resourceCache != nil {
		cache = &p.resourceCache.managedAPIs
	}
	return cache.list(ctx, namespaces, p.namespaceFanout, p.client.ListManagedAPIs,
		func(api state.API) string { return api.NormalizedLabels[labels.NamespaceKey] })
}

func (p *Planner) listManagedCatalogServices(
	ctx context.Context,
	namespaces []string,
) ([]state.CatalogService, error) {
	var cache *observationCache[state.CatalogService]
	if p.resourceCache != nil {
		cache = &p.resourceCache.managedCatalogServices
	}
	return cache.list(ctx, namespaces, p.namespaceFanout, p.client.ListManagedCatalogServices,
		func(service state.CatalogService) string { return service.NormalizedLabels[labels.NamespaceKey] })
}

func (p *Planner) listManagedAIGateways(
	ctx context.Context,
	namespaces []string,
) ([]state.AIGateway, error) {
	var cache *observationCache[state.AIGateway]
	if p.resourceCache != nil {
		cache = &p.resourceCache.managedAIGateways
	}
	return cache.list(ctx, namespaces, p.namespaceFanout, p.client.ListManagedAIGateways,
		func(gateway state.AIGateway) string { return gateway.NormalizedLabels[labels.NamespaceKey] })
}

func (p *Planner) listManagedDashboards(
	ctx context.Context,
	namespaces []string,
) ([]state.Dashboard, error) {
	var cache *observationCache[state.Dashboard]
	if p.resourceCache != nil {
		cache = &p.resourceCache.managedDashboards
	}
	return cache.list(ctx, namespaces, p.namespaceFanout, p.client.ListManagedDashboards,
		func(dashboard state.Dashboard) string { return dashboard.NormalizedLabels[labels.NamespaceKey] })
}

func (p *Planner) listManagedEventGatewayControlPlanes(
	ctx context.Context,
	namespaces []string,
) ([]state.EventGatewayControlPlane, error) {
	var cache *observationCache[state.EventGatewayControlPlane]
	if p.resourceCache != nil {
		cache = &p.resourceCache.managedEventGatewayControlPlanes
	}
	return cache.list(ctx, namespaces, p.namespaceFanout, p.client.ListManagedEventGatewayControlPlanes,
		func(gateway state.EventGatewayControlPlane) string {
			return gateway.NormalizedLabels[labels.NamespaceKey]
		})
}

func (p *Planner) listManagedOrganizationTeams(
	ctx context.Context,
	namespaces []string,
) ([]state.OrganizationTeam, error) {
	var cache *observationCache[state.OrganizationTeam]
	if p.resourceCache != nil {
		cache = &p.resourceCache.managedOrganizationTeams
	}
	return cache.list(ctx, namespaces, p.namespaceFanout, p.client.ListManagedOrganizationTeams,
		func(team state.OrganizationTeam) string { return team.NormalizedLabels[labels.NamespaceKey] })
}

func (p *Planner) listPortalTeams(ctx context.Context, portalID string) ([]state.PortalTeam, error) {
	if portalID == "" {
		return []state.PortalTeam{}, nil
	}

	cache := p.resourceCache
	if cache != nil {
		if cached, ok := cache.portalTeamsByPortalID[portalID]; ok {
			return cached, nil
		}
	}

	teams, err := p.client.ListPortalTeams(ctx, portalID)
	if err != nil {
		return nil, err
	}

	if cache != nil {
		cache.portalTeamsByPortalID[portalID] = teams
	}

	return teams, nil
}

func (p *Planner) listPortalIdentityProviders(
	ctx context.Context,
	portalID string,
) ([]state.PortalIdentityProvider, error) {
	if portalID == "" {
		return []state.PortalIdentityProvider{}, nil
	}

	cache := p.resourceCache
	if cache != nil {
		if cached, ok := cache.portalIdentityProvidersByPortalID[portalID]; ok {
			return cached, nil
		}
	}

	providers, err := p.client.ListPortalIdentityProviders(ctx, portalID)
	if err != nil {
		return nil, err
	}

	if cache != nil {
		cache.portalIdentityProvidersByPortalID[portalID] = providers
	}

	return providers, nil
}

func (p *Planner) listPortalTeamGroupMappings(
	ctx context.Context,
	portalID string,
) ([]state.PortalTeamGroupMapping, error) {
	if portalID == "" {
		return []state.PortalTeamGroupMapping{}, nil
	}

	cache := p.resourceCache
	if cache != nil {
		if cached, ok := cache.portalTeamGroupMappingsByPortalID[portalID]; ok {
			return cached, nil
		}
	}

	mappings, err := p.client.ListPortalTeamGroupMappings(ctx, portalID)
	if err != nil {
		return nil, err
	}

	if cache != nil {
		cache.portalTeamGroupMappingsByPortalID[portalID] = mappings
	}

	return mappings, nil
}

func normalizeNamespaces(namespaces []string) []string {
	if len(namespaces) == 0 {
		return nil
	}

	normalizedSet := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		trimmed := strings.TrimSpace(ns)
		if trimmed == "" {
			continue
		}
		if trimmed == "*" {
			return []string{"*"}
		}
		normalizedSet[trimmed] = struct{}{}
	}

	if len(normalizedSet) == 0 {
		return nil
	}

	return slices.Sorted(maps.Keys(normalizedSet))
}

func namespaceCacheKey(normalizedNamespaces []string) string {
	if len(normalizedNamespaces) == 0 {
		return ""
	}
	return strings.Join(normalizedNamespaces, ",")
}
