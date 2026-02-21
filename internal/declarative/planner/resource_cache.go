package planner

import (
	"context"
	"sort"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/state"
)

type planningResourceCache struct {
	managedAPIsByKey  map[string][]state.API
	managedAPIsAll    []state.API
	managedAPIsLoaded bool
}

func newPlanningResourceCache() *planningResourceCache {
	return &planningResourceCache{
		managedAPIsByKey: make(map[string][]state.API),
	}
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
