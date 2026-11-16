package pagination

import (
	"context"
	"fmt"
)

const MaxNumberedPages int64 = 10000

// PageMeta describes raw API progress before any caller-side filtering.
type PageMeta struct {
	Total float64
	// RawCount must be set when the returned items have been filtered.
	// Unfiltered readers may leave it zero to use the returned item count.
	RawCount int
}

type NumberedLister[T any] func(context.Context, int64, int64) ([]T, *PageMeta, error)

// NumberedReader validates page progress and optionally checks resource identities.
type NumberedReader[T any] struct {
	Fetch NumberedLister[T]
	Key   func(T) string
}

func (r NumberedReader[T]) All(ctx context.Context, pageSize int64) ([]T, error) {
	var result []T
	err := r.walk(ctx, pageSize, func(value T) bool {
		result = append(result, value)
		return false
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Find stops as soon as the requested resource is observed.
func (r NumberedReader[T]) Find(
	ctx context.Context,
	pageSize int64,
	match func(T) bool,
) (*T, error) {
	var result *T
	err := r.walk(ctx, pageSize, func(value T) bool {
		if match(value) {
			result = &value
			return true
		}
		return false
	})
	return result, err
}

func (r NumberedReader[T]) walk(ctx context.Context, pageSize int64, visit func(T) bool) error {
	if pageSize <= 0 {
		pageSize = 100
	}
	seen := make(map[string]bool)
	var fetched int64
	for page := int64(1); page <= MaxNumberedPages; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		items, meta, err := r.Fetch(ctx, pageSize, page)
		if err != nil {
			return err
		}
		if meta == nil {
			return fmt.Errorf("missing pagination metadata")
		}
		rawCount := meta.RawCount
		if rawCount == 0 {
			rawCount = len(items)
		}
		if rawCount < len(items) || rawCount < 0 {
			return fmt.Errorf("invalid raw pagination count")
		}
		for _, value := range items {
			if r.Key != nil {
				id := r.Key(value)
				if id == "" || seen[id] {
					return fmt.Errorf("pagination response contains a missing or repeated ID")
				}
				seen[id] = true
			}
			if visit(value) {
				return nil
			}
		}
		fetched += int64(rawCount)
		if float64(fetched) >= meta.Total {
			return nil
		}
		if rawCount == 0 {
			return fmt.Errorf("incomplete pagination: empty page before reported total")
		}
	}
	return fmt.Errorf("pagination exceeded safety limit of %d pages", MaxNumberedPages)
}
