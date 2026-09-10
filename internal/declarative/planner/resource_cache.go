package planner

import (
	"context"
	"sort"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/state"
)

type planningResourceCache struct {
	managedControlPlanesByKey  map[string][]state.ControlPlane
	managedControlPlanesAll    []state.ControlPlane
	managedControlPlanesLoaded bool

	managedEventGatewayControlPlanesByKey  map[string][]state.EventGatewayControlPlane
	managedEventGatewayControlPlanesAll    []state.EventGatewayControlPlane
	managedEventGatewayControlPlanesLoaded bool

	managedPortals observationCache[state.Portal]

	managedAuthStrategiesByKey  map[string][]state.ApplicationAuthStrategy
	managedAuthStrategiesAll    []state.ApplicationAuthStrategy
	managedAuthStrategiesLoaded bool
	managedDCRProvidersByKey    map[string][]state.DCRProvider
	managedDCRProvidersAll      []state.DCRProvider
	managedDCRProvidersLoaded   bool

	managedAPIs observationCache[state.API]

	managedCatalogServicesByKey  map[string][]state.CatalogService
	managedCatalogServicesAll    []state.CatalogService
	managedCatalogServicesLoaded bool

	managedAIGatewaysByKey  map[string][]state.AIGateway
	managedAIGatewaysAll    []state.AIGateway
	managedAIGatewaysLoaded bool

	managedDashboardsByKey  map[string][]state.Dashboard
	managedDashboardsAll    []state.Dashboard
	managedDashboardsLoaded bool

	managedOrganizationTeamsByKey  map[string][]state.OrganizationTeam
	managedOrganizationTeamsAll    []state.OrganizationTeam
	managedOrganizationTeamsLoaded bool

	portalTeamsByPortalID             map[string][]state.PortalTeam
	portalIdentityProvidersByPortalID map[string][]state.PortalIdentityProvider
	portalTeamGroupMappingsByPortalID map[string][]state.PortalTeamGroupMapping
}

func newPlanningResourceCache() *planningResourceCache {
	return &planningResourceCache{
		managedControlPlanesByKey:             make(map[string][]state.ControlPlane),
		managedEventGatewayControlPlanesByKey: make(map[string][]state.EventGatewayControlPlane),
		managedPortals:                        newObservationCache[state.Portal](),
		managedAuthStrategiesByKey:            make(map[string][]state.ApplicationAuthStrategy),
		managedDCRProvidersByKey:              make(map[string][]state.DCRProvider),
		managedAPIs:                           newObservationCache[state.API](),
		managedCatalogServicesByKey:           make(map[string][]state.CatalogService),
		managedAIGatewaysByKey:                make(map[string][]state.AIGateway),
		managedDashboardsByKey:                make(map[string][]state.Dashboard),
		managedOrganizationTeamsByKey:         make(map[string][]state.OrganizationTeam),
		portalTeamsByPortalID:                 make(map[string][]state.PortalTeam),
		portalIdentityProvidersByPortalID:     make(map[string][]state.PortalIdentityProvider),
		portalTeamGroupMappingsByPortalID:     make(map[string][]state.PortalTeamGroupMapping),
	}
}

func (p *Planner) listManagedControlPlanes(ctx context.Context, namespaces []string) ([]state.ControlPlane, error) {
	normalizedNamespaces := normalizeNamespaces(namespaces)
	if len(normalizedNamespaces) == 0 {
		return []state.ControlPlane{}, nil
	}

	cache := p.resourceCache
	cacheKey := namespaceCacheKey(normalizedNamespaces)
	if cache != nil {
		if cached, ok := cache.managedControlPlanesByKey[cacheKey]; ok {
			return cached, nil
		}

		if cacheKey != "*" && cache.managedControlPlanesLoaded {
			filtered := filterControlPlanesByNamespaces(cache.managedControlPlanesAll, normalizedNamespaces)
			cache.managedControlPlanesByKey[cacheKey] = filtered
			return filtered, nil
		}
	}

	requestNamespaces := normalizedNamespaces
	useAllNamespaces := p.namespaceFanout && cacheKey != "*"
	if useAllNamespaces {
		requestNamespaces = []string{"*"}
	}

	controlPlanes, err := p.client.ListManagedControlPlanes(ctx, requestNamespaces)
	if err != nil {
		return nil, err
	}

	if cache != nil {
		if useAllNamespaces {
			cache.managedControlPlanesAll = controlPlanes
			cache.managedControlPlanesLoaded = true
			filtered := filterControlPlanesByNamespaces(controlPlanes, normalizedNamespaces)
			cache.managedControlPlanesByKey[cacheKey] = filtered
			return filtered, nil
		}

		cache.managedControlPlanesByKey[cacheKey] = controlPlanes
		if cacheKey == "*" {
			cache.managedControlPlanesAll = controlPlanes
			cache.managedControlPlanesLoaded = true
		}
	}

	return controlPlanes, nil
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
	normalizedNamespaces := normalizeNamespaces(namespaces)
	if len(normalizedNamespaces) == 0 {
		return []state.ApplicationAuthStrategy{}, nil
	}

	cache := p.resourceCache
	cacheKey := namespaceCacheKey(normalizedNamespaces)
	if cache != nil {
		if cached, ok := cache.managedAuthStrategiesByKey[cacheKey]; ok {
			return cached, nil
		}

		if cacheKey != "*" && cache.managedAuthStrategiesLoaded {
			filtered := filterAuthStrategiesByNamespaces(
				cache.managedAuthStrategiesAll,
				normalizedNamespaces,
			)
			cache.managedAuthStrategiesByKey[cacheKey] = filtered
			return filtered, nil
		}
	}

	requestNamespaces := normalizedNamespaces
	useAllNamespaces := p.namespaceFanout && cacheKey != "*"
	if useAllNamespaces {
		requestNamespaces = []string{"*"}
	}

	strategies, err := p.client.ListManagedAuthStrategies(ctx, requestNamespaces)
	if err != nil {
		return nil, err
	}

	if cache != nil {
		if useAllNamespaces {
			cache.managedAuthStrategiesAll = strategies
			cache.managedAuthStrategiesLoaded = true
			filtered := filterAuthStrategiesByNamespaces(strategies, normalizedNamespaces)
			cache.managedAuthStrategiesByKey[cacheKey] = filtered
			return filtered, nil
		}

		cache.managedAuthStrategiesByKey[cacheKey] = strategies
		if cacheKey == "*" {
			cache.managedAuthStrategiesAll = strategies
			cache.managedAuthStrategiesLoaded = true
		}
	}

	return strategies, nil
}

func (p *Planner) listManagedDCRProviders(ctx context.Context, namespaces []string) ([]state.DCRProvider, error) {
	normalizedNamespaces := normalizeNamespaces(namespaces)
	if len(normalizedNamespaces) == 0 {
		return []state.DCRProvider{}, nil
	}

	cache := p.resourceCache
	cacheKey := namespaceCacheKey(normalizedNamespaces)
	if cache != nil {
		if cached, ok := cache.managedDCRProvidersByKey[cacheKey]; ok {
			return cached, nil
		}

		if cacheKey != "*" && cache.managedDCRProvidersLoaded {
			filtered := filterDCRProvidersByNamespaces(cache.managedDCRProvidersAll, normalizedNamespaces)
			cache.managedDCRProvidersByKey[cacheKey] = filtered
			return filtered, nil
		}
	}

	requestNamespaces := normalizedNamespaces
	useAllNamespaces := p.namespaceFanout && cacheKey != "*"
	if useAllNamespaces {
		requestNamespaces = []string{"*"}
	}

	providers, err := p.client.ListManagedDCRProviders(ctx, requestNamespaces)
	if err != nil {
		return nil, err
	}

	if cache != nil {
		if useAllNamespaces {
			cache.managedDCRProvidersAll = providers
			cache.managedDCRProvidersLoaded = true
			filtered := filterDCRProvidersByNamespaces(providers, normalizedNamespaces)
			cache.managedDCRProvidersByKey[cacheKey] = filtered
			return filtered, nil
		}

		cache.managedDCRProvidersByKey[cacheKey] = providers
		if cacheKey == "*" {
			cache.managedDCRProvidersAll = providers
			cache.managedDCRProvidersLoaded = true
		}
	}

	return providers, nil
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
	normalizedNamespaces := normalizeNamespaces(namespaces)
	if len(normalizedNamespaces) == 0 {
		return []state.CatalogService{}, nil
	}

	cache := p.resourceCache
	cacheKey := namespaceCacheKey(normalizedNamespaces)
	if cache != nil {
		if cached, ok := cache.managedCatalogServicesByKey[cacheKey]; ok {
			return cached, nil
		}

		if cacheKey != "*" && cache.managedCatalogServicesLoaded {
			filtered := filterCatalogServicesByNamespaces(cache.managedCatalogServicesAll, normalizedNamespaces)
			cache.managedCatalogServicesByKey[cacheKey] = filtered
			return filtered, nil
		}
	}

	requestNamespaces := normalizedNamespaces
	useAllNamespaces := p.namespaceFanout && cacheKey != "*"
	if useAllNamespaces {
		requestNamespaces = []string{"*"}
	}

	services, err := p.client.ListManagedCatalogServices(ctx, requestNamespaces)
	if err != nil {
		return nil, err
	}

	if cache != nil {
		if useAllNamespaces {
			cache.managedCatalogServicesAll = services
			cache.managedCatalogServicesLoaded = true
			filtered := filterCatalogServicesByNamespaces(services, normalizedNamespaces)
			cache.managedCatalogServicesByKey[cacheKey] = filtered
			return filtered, nil
		}

		cache.managedCatalogServicesByKey[cacheKey] = services
		if cacheKey == "*" {
			cache.managedCatalogServicesAll = services
			cache.managedCatalogServicesLoaded = true
		}
	}

	return services, nil
}

func (p *Planner) listManagedAIGateways(
	ctx context.Context,
	namespaces []string,
) ([]state.AIGateway, error) {
	normalizedNamespaces := normalizeNamespaces(namespaces)
	if len(normalizedNamespaces) == 0 {
		return []state.AIGateway{}, nil
	}

	cache := p.resourceCache
	cacheKey := namespaceCacheKey(normalizedNamespaces)
	if cache != nil {
		if cached, ok := cache.managedAIGatewaysByKey[cacheKey]; ok {
			return cached, nil
		}

		if cacheKey != "*" && cache.managedAIGatewaysLoaded {
			filtered := filterAIGatewaysByNamespaces(cache.managedAIGatewaysAll, normalizedNamespaces)
			cache.managedAIGatewaysByKey[cacheKey] = filtered
			return filtered, nil
		}
	}

	requestNamespaces := normalizedNamespaces
	useAllNamespaces := p.namespaceFanout && cacheKey != "*"
	if useAllNamespaces {
		requestNamespaces = []string{"*"}
	}

	gateways, err := p.client.ListManagedAIGateways(ctx, requestNamespaces)
	if err != nil {
		return nil, err
	}

	if cache != nil {
		if useAllNamespaces {
			cache.managedAIGatewaysAll = gateways
			cache.managedAIGatewaysLoaded = true
			filtered := filterAIGatewaysByNamespaces(gateways, normalizedNamespaces)
			cache.managedAIGatewaysByKey[cacheKey] = filtered
			return filtered, nil
		}

		cache.managedAIGatewaysByKey[cacheKey] = gateways
		if cacheKey == "*" {
			cache.managedAIGatewaysAll = gateways
			cache.managedAIGatewaysLoaded = true
		}
	}

	return gateways, nil
}

func (p *Planner) listManagedDashboards(
	ctx context.Context,
	namespaces []string,
) ([]state.Dashboard, error) {
	normalizedNamespaces := normalizeNamespaces(namespaces)
	if len(normalizedNamespaces) == 0 {
		return []state.Dashboard{}, nil
	}

	cache := p.resourceCache
	cacheKey := namespaceCacheKey(normalizedNamespaces)
	if cache != nil {
		if cached, ok := cache.managedDashboardsByKey[cacheKey]; ok {
			return cached, nil
		}

		if cacheKey != "*" && cache.managedDashboardsLoaded {
			filtered := filterDashboardsByNamespaces(cache.managedDashboardsAll, normalizedNamespaces)
			cache.managedDashboardsByKey[cacheKey] = filtered
			return filtered, nil
		}
	}

	requestNamespaces := normalizedNamespaces
	useAllNamespaces := p.namespaceFanout && cacheKey != "*"
	if useAllNamespaces {
		requestNamespaces = []string{"*"}
	}

	dashboards, err := p.client.ListManagedDashboards(ctx, requestNamespaces)
	if err != nil {
		return nil, err
	}

	if cache != nil {
		if useAllNamespaces {
			cache.managedDashboardsAll = dashboards
			cache.managedDashboardsLoaded = true
			filtered := filterDashboardsByNamespaces(dashboards, normalizedNamespaces)
			cache.managedDashboardsByKey[cacheKey] = filtered
			return filtered, nil
		}

		cache.managedDashboardsByKey[cacheKey] = dashboards
		if cacheKey == "*" {
			cache.managedDashboardsAll = dashboards
			cache.managedDashboardsLoaded = true
		}
	}

	return dashboards, nil
}

func (p *Planner) listManagedEventGatewayControlPlanes(
	ctx context.Context,
	namespaces []string,
) ([]state.EventGatewayControlPlane, error) {
	normalizedNamespaces := normalizeNamespaces(namespaces)
	if len(normalizedNamespaces) == 0 {
		return []state.EventGatewayControlPlane{}, nil
	}

	cache := p.resourceCache
	cacheKey := namespaceCacheKey(normalizedNamespaces)
	if cache != nil {
		if cached, ok := cache.managedEventGatewayControlPlanesByKey[cacheKey]; ok {
			return cached, nil
		}

		if cacheKey != "*" && cache.managedEventGatewayControlPlanesLoaded {
			filtered := filterEventGatewayControlPlanesByNamespaces(
				cache.managedEventGatewayControlPlanesAll,
				normalizedNamespaces,
			)
			cache.managedEventGatewayControlPlanesByKey[cacheKey] = filtered
			return filtered, nil
		}
	}

	requestNamespaces := normalizedNamespaces
	useAllNamespaces := p.namespaceFanout && cacheKey != "*"
	if useAllNamespaces {
		requestNamespaces = []string{"*"}
	}

	gateways, err := p.client.ListManagedEventGatewayControlPlanes(ctx, requestNamespaces)
	if err != nil {
		return nil, err
	}

	if cache != nil {
		if useAllNamespaces {
			cache.managedEventGatewayControlPlanesAll = gateways
			cache.managedEventGatewayControlPlanesLoaded = true
			filtered := filterEventGatewayControlPlanesByNamespaces(gateways, normalizedNamespaces)
			cache.managedEventGatewayControlPlanesByKey[cacheKey] = filtered
			return filtered, nil
		}

		cache.managedEventGatewayControlPlanesByKey[cacheKey] = gateways
		if cacheKey == "*" {
			cache.managedEventGatewayControlPlanesAll = gateways
			cache.managedEventGatewayControlPlanesLoaded = true
		}
	}

	return gateways, nil
}

func (p *Planner) listManagedOrganizationTeams(
	ctx context.Context,
	namespaces []string,
) ([]state.OrganizationTeam, error) {
	normalizedNamespaces := normalizeNamespaces(namespaces)
	if len(normalizedNamespaces) == 0 {
		return []state.OrganizationTeam{}, nil
	}

	cache := p.resourceCache
	cacheKey := namespaceCacheKey(normalizedNamespaces)
	if cache != nil {
		if cached, ok := cache.managedOrganizationTeamsByKey[cacheKey]; ok {
			return cached, nil
		}

		if cacheKey != "*" && cache.managedOrganizationTeamsLoaded {
			filtered := filterOrganizationTeamsByNamespaces(cache.managedOrganizationTeamsAll, normalizedNamespaces)
			cache.managedOrganizationTeamsByKey[cacheKey] = filtered
			return filtered, nil
		}
	}

	requestNamespaces := normalizedNamespaces
	useAllNamespaces := p.namespaceFanout && cacheKey != "*"
	if useAllNamespaces {
		requestNamespaces = []string{"*"}
	}

	teams, err := p.client.ListManagedOrganizationTeams(ctx, requestNamespaces)
	if err != nil {
		return nil, err
	}

	if cache != nil {
		if useAllNamespaces {
			cache.managedOrganizationTeamsAll = teams
			cache.managedOrganizationTeamsLoaded = true
			filtered := filterOrganizationTeamsByNamespaces(teams, normalizedNamespaces)
			cache.managedOrganizationTeamsByKey[cacheKey] = filtered
			return filtered, nil
		}

		cache.managedOrganizationTeamsByKey[cacheKey] = teams
		if cacheKey == "*" {
			cache.managedOrganizationTeamsAll = teams
			cache.managedOrganizationTeamsLoaded = true
		}
	}

	return teams, nil
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

	normalized := make([]string, 0, len(normalizedSet))
	for ns := range normalizedSet {
		normalized = append(normalized, ns)
	}
	sort.Strings(normalized)
	return normalized
}

func namespaceCacheKey(normalizedNamespaces []string) string {
	if len(normalizedNamespaces) == 0 {
		return ""
	}
	return strings.Join(normalizedNamespaces, ",")
}

func filterControlPlanesByNamespaces(controlPlanes []state.ControlPlane, namespaces []string) []state.ControlPlane {
	if len(namespaces) == 0 {
		return []state.ControlPlane{}
	}
	if len(namespaces) == 1 && namespaces[0] == "*" {
		return controlPlanes
	}

	allowed := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		allowed[ns] = struct{}{}
	}

	filtered := make([]state.ControlPlane, 0, len(controlPlanes))
	for _, controlPlane := range controlPlanes {
		namespace := controlPlane.NormalizedLabels[labels.NamespaceKey]
		if _, ok := allowed[namespace]; ok {
			filtered = append(filtered, controlPlane)
		}
	}

	return filtered
}

func filterAuthStrategiesByNamespaces(
	strategies []state.ApplicationAuthStrategy,
	namespaces []string,
) []state.ApplicationAuthStrategy {
	if len(namespaces) == 0 {
		return []state.ApplicationAuthStrategy{}
	}
	if len(namespaces) == 1 && namespaces[0] == "*" {
		return strategies
	}

	allowed := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		allowed[ns] = struct{}{}
	}

	filtered := make([]state.ApplicationAuthStrategy, 0, len(strategies))
	for _, strategy := range strategies {
		namespace := strategy.NormalizedLabels[labels.NamespaceKey]
		if _, ok := allowed[namespace]; ok {
			filtered = append(filtered, strategy)
		}
	}

	return filtered
}

func filterDCRProvidersByNamespaces(providers []state.DCRProvider, namespaces []string) []state.DCRProvider {
	if len(namespaces) == 0 {
		return []state.DCRProvider{}
	}
	if len(namespaces) == 1 && namespaces[0] == "*" {
		return providers
	}

	allowed := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		allowed[ns] = struct{}{}
	}

	filtered := make([]state.DCRProvider, 0, len(providers))
	for _, provider := range providers {
		namespace := provider.NormalizedLabels[labels.NamespaceKey]
		if _, ok := allowed[namespace]; ok {
			filtered = append(filtered, provider)
		}
	}

	return filtered
}

func filterCatalogServicesByNamespaces(services []state.CatalogService, namespaces []string) []state.CatalogService {
	if len(namespaces) == 0 {
		return []state.CatalogService{}
	}
	if len(namespaces) == 1 && namespaces[0] == "*" {
		return services
	}

	allowed := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		allowed[ns] = struct{}{}
	}

	filtered := make([]state.CatalogService, 0, len(services))
	for _, service := range services {
		namespace := service.NormalizedLabels[labels.NamespaceKey]
		if _, ok := allowed[namespace]; ok {
			filtered = append(filtered, service)
		}
	}

	return filtered
}

func filterAIGatewaysByNamespaces(gateways []state.AIGateway, namespaces []string) []state.AIGateway {
	if len(namespaces) == 0 {
		return []state.AIGateway{}
	}
	if len(namespaces) == 1 && namespaces[0] == "*" {
		return gateways
	}

	allowed := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		allowed[ns] = struct{}{}
	}

	filtered := make([]state.AIGateway, 0, len(gateways))
	for _, gateway := range gateways {
		namespace := gateway.NormalizedLabels[labels.NamespaceKey]
		if _, ok := allowed[namespace]; ok {
			filtered = append(filtered, gateway)
		}
	}

	return filtered
}

func filterDashboardsByNamespaces(dashboards []state.Dashboard, namespaces []string) []state.Dashboard {
	if len(namespaces) == 0 {
		return []state.Dashboard{}
	}
	if len(namespaces) == 1 && namespaces[0] == "*" {
		return dashboards
	}

	allowed := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		allowed[ns] = struct{}{}
	}

	filtered := make([]state.Dashboard, 0, len(dashboards))
	for _, dashboard := range dashboards {
		namespace := dashboard.NormalizedLabels[labels.NamespaceKey]
		if _, ok := allowed[namespace]; ok {
			filtered = append(filtered, dashboard)
		}
	}

	return filtered
}

func filterEventGatewayControlPlanesByNamespaces(
	gateways []state.EventGatewayControlPlane,
	namespaces []string,
) []state.EventGatewayControlPlane {
	if len(namespaces) == 0 {
		return []state.EventGatewayControlPlane{}
	}
	if len(namespaces) == 1 && namespaces[0] == "*" {
		return gateways
	}

	allowed := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		allowed[ns] = struct{}{}
	}

	filtered := make([]state.EventGatewayControlPlane, 0, len(gateways))
	for _, gateway := range gateways {
		namespace := gateway.NormalizedLabels[labels.NamespaceKey]
		if _, ok := allowed[namespace]; ok {
			filtered = append(filtered, gateway)
		}
	}

	return filtered
}

func filterOrganizationTeamsByNamespaces(
	teams []state.OrganizationTeam,
	namespaces []string,
) []state.OrganizationTeam {
	if len(namespaces) == 0 {
		return []state.OrganizationTeam{}
	}
	if len(namespaces) == 1 && namespaces[0] == "*" {
		return teams
	}

	allowed := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		allowed[ns] = struct{}{}
	}

	filtered := make([]state.OrganizationTeam, 0, len(teams))
	for _, team := range teams {
		namespace := team.NormalizedLabels[labels.NamespaceKey]
		if _, ok := allowed[namespace]; ok {
			filtered = append(filtered, team)
		}
	}

	return filtered
}
