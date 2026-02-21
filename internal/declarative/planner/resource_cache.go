package planner

import (
	"context"
	"sort"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/state"
)

type planningResourceCache struct {
	managedPortalsByKey  map[string][]state.Portal
	managedPortalsAll    []state.Portal
	managedPortalsLoaded bool

	managedAPIsByKey  map[string][]state.API
	managedAPIsAll    []state.API
	managedAPIsLoaded bool
}

func newPlanningResourceCache() *planningResourceCache {
	return &planningResourceCache{
		managedPortalsByKey: make(map[string][]state.Portal),
		managedAPIsByKey:    make(map[string][]state.API),
	}
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
