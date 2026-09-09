package planner

import "context"

// observationCache holds one managed resource kind's observations for a plan run.
// Namespace planners share it; a new run creates a fresh cache.
type observationCache[T any] struct {
	byNamespaces map[string][]T
	all          []T
	// A successful empty observation must be distinguishable from a cache miss.
	allLoaded bool
}

func newObservationCache[T any]() observationCache[T] {
	return observationCache[T]{byNamespaces: make(map[string][]T)}
}

// list preserves uncached reads when the receiver is nil. In that case, fanout
// still requests all namespaces and returns the fetched result without filtering.
func (c *observationCache[T]) list(
	ctx context.Context,
	namespaces []string,
	fanout bool,
	fetch func(context.Context, []string) ([]T, error),
	namespaceOf func(T) string,
) ([]T, error) {
	normalizedNamespaces := normalizeNamespaces(namespaces)
	if len(normalizedNamespaces) == 0 {
		return []T{}, nil
	}

	cacheKey := namespaceCacheKey(normalizedNamespaces)
	if c != nil {
		if cached, ok := c.byNamespaces[cacheKey]; ok {
			return cached, nil
		}

		if cacheKey != "*" && c.allLoaded {
			filtered := filterObservedNamespaces(c.all, normalizedNamespaces, namespaceOf)
			c.byNamespaces[cacheKey] = filtered
			return filtered, nil
		}
	}

	requestNamespaces := normalizedNamespaces
	useAllNamespaces := fanout && cacheKey != "*"
	if useAllNamespaces {
		requestNamespaces = []string{"*"}
	}

	observed, err := fetch(ctx, requestNamespaces)
	if err != nil {
		return nil, err
	}

	if c != nil {
		if useAllNamespaces {
			c.all = observed
			c.allLoaded = true
			filtered := filterObservedNamespaces(observed, normalizedNamespaces, namespaceOf)
			c.byNamespaces[cacheKey] = filtered
			return filtered, nil
		}

		c.byNamespaces[cacheKey] = observed
		if cacheKey == "*" {
			c.all = observed
			c.allLoaded = true
		}
	}

	return observed, nil
}

// filterObservedNamespaces receives normalized, non-empty, non-wildcard namespaces.
func filterObservedNamespaces[T any](observed []T, namespaces []string, namespaceOf func(T) string) []T {
	allowed := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		allowed[ns] = struct{}{}
	}

	filtered := make([]T, 0, len(observed))
	for _, resource := range observed {
		if _, ok := allowed[namespaceOf(resource)]; ok {
			filtered = append(filtered, resource)
		}
	}
	return filtered
}
