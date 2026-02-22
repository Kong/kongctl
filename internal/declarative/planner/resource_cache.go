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

	managedPortalsByKey  map[string][]state.Portal
	managedPortalsAll    []state.Portal
	managedPortalsLoaded bool

	managedAuthStrategiesByKey  map[string][]state.ApplicationAuthStrategy
	managedAuthStrategiesAll    []state.ApplicationAuthStrategy
	managedAuthStrategiesLoaded bool

	managedAPIsByKey  map[string][]state.API
	managedAPIsAll    []state.API
	managedAPIsLoaded bool

	portalTeamsByPortalID map[string][]state.PortalTeam
}

func newPlanningResourceCache() *planningResourceCache {
	return &planningResourceCache{
		managedControlPlanesByKey:  make(map[string][]state.ControlPlane),
		managedPortalsByKey:        make(map[string][]state.Portal),
		managedAuthStrategiesByKey: make(map[string][]state.ApplicationAuthStrategy),
		managedAPIsByKey:           make(map[string][]state.API),
		portalTeamsByPortalID:      make(map[string][]state.PortalTeam),
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

	controlPlanes, err := p.client.ListManagedControlPlanes(ctx, normalizedNamespaces)
	if err != nil {
		return nil, err
	}

	if cache != nil {
		cache.managedControlPlanesByKey[cacheKey] = controlPlanes
		if cacheKey == "*" {
			cache.managedControlPlanesAll = controlPlanes
			cache.managedControlPlanesLoaded = true
		}
	}

	return controlPlanes, nil
}

func (p *Planner) listManagedPortals(ctx context.Context, namespaces []string) ([]state.Portal, error) {
	normalizedNamespaces := normalizeNamespaces(namespaces)
	if len(normalizedNamespaces) == 0 {
		return []state.Portal{}, nil
	}

	cache := p.resourceCache
	cacheKey := namespaceCacheKey(normalizedNamespaces)
	if cache != nil {
		if cached, ok := cache.managedPortalsByKey[cacheKey]; ok {
			return cached, nil
		}

		if cacheKey != "*" && cache.managedPortalsLoaded {
			filtered := filterPortalsByNamespaces(cache.managedPortalsAll, normalizedNamespaces)
			cache.managedPortalsByKey[cacheKey] = filtered
			return filtered, nil
		}
	}

	portals, err := p.client.ListManagedPortals(ctx, normalizedNamespaces)
	if err != nil {
		return nil, err
	}

	if cache != nil {
		cache.managedPortalsByKey[cacheKey] = portals
		if cacheKey == "*" {
			cache.managedPortalsAll = portals
			cache.managedPortalsLoaded = true
		}
	}

	return portals, nil
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

	strategies, err := p.client.ListManagedAuthStrategies(ctx, normalizedNamespaces)
	if err != nil {
		return nil, err
	}

	if cache != nil {
		cache.managedAuthStrategiesByKey[cacheKey] = strategies
		if cacheKey == "*" {
			cache.managedAuthStrategiesAll = strategies
			cache.managedAuthStrategiesLoaded = true
		}
	}

	return strategies, nil
}

func (p *Planner) listManagedAPIs(ctx context.Context, namespaces []string) ([]state.API, error) {
	normalizedNamespaces := normalizeNamespaces(namespaces)
	if len(normalizedNamespaces) == 0 {
		return []state.API{}, nil
	}

	cache := p.resourceCache
	cacheKey := namespaceCacheKey(normalizedNamespaces)
	if cache != nil {
		if cached, ok := cache.managedAPIsByKey[cacheKey]; ok {
			return cached, nil
		}

		if cacheKey != "*" && cache.managedAPIsLoaded {
			filtered := filterAPIsByNamespaces(cache.managedAPIsAll, normalizedNamespaces)
			cache.managedAPIsByKey[cacheKey] = filtered
			return filtered, nil
		}
	}

	apis, err := p.client.ListManagedAPIs(ctx, normalizedNamespaces)
	if err != nil {
		return nil, err
	}

	if cache != nil {
		cache.managedAPIsByKey[cacheKey] = apis
		if cacheKey == "*" {
			cache.managedAPIsAll = apis
			cache.managedAPIsLoaded = true
		}
	}

	return apis, nil
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

func filterPortalsByNamespaces(portals []state.Portal, namespaces []string) []state.Portal {
	if len(namespaces) == 0 {
		return []state.Portal{}
	}
	if len(namespaces) == 1 && namespaces[0] == "*" {
		return portals
	}

	allowed := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		allowed[ns] = struct{}{}
	}

	filtered := make([]state.Portal, 0, len(portals))
	for _, portal := range portals {
		namespace := portal.NormalizedLabels[labels.NamespaceKey]
		if _, ok := allowed[namespace]; ok {
			filtered = append(filtered, portal)
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

func filterAPIsByNamespaces(apis []state.API, namespaces []string) []state.API {
	if len(namespaces) == 0 {
		return []state.API{}
	}
	if len(namespaces) == 1 && namespaces[0] == "*" {
		return apis
	}

	allowed := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		allowed[ns] = struct{}{}
	}

	filtered := make([]state.API, 0, len(apis))
	for _, api := range apis {
		namespace := api.NormalizedLabels[labels.NamespaceKey]
		if _, ok := allowed[namespace]; ok {
			filtered = append(filtered, api)
		}
	}
	return filtered
}
