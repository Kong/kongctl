package state

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/util/pagination"
)

// PageMeta contains pagination metadata from API responses.
type PageMeta = pagination.PageMeta

type PaginatedLister[T any] func(ctx context.Context, pageSize, pageNumber int64) ([]T, *PageMeta, error)

// PaginateAll counts raw rows so filtering and API page-size caps cannot hide pages.
func PaginateAll[T any](ctx context.Context, lister PaginatedLister[T]) ([]T, error) {
	return (pagination.NumberedReader[T]{Fetch: pagination.NumberedLister[T](lister)}).All(ctx, 100)
}

// FilteredPaginatedLister wraps a PaginatedLister with additional filtering logic
type FilteredPaginatedLister[T any] func(
	ctx context.Context, pageSize, pageNumber int64, filter func(T) bool,
) ([]T, *PageMeta, error)

// PaginateAllFiltered fetches all pages from a paginated API endpoint with filtering
func PaginateAllFiltered[T any](
	ctx context.Context, lister FilteredPaginatedLister[T], filter func(T) bool,
) ([]T, error) {
	return PaginateAll(ctx, func(ctx context.Context, size, page int64) ([]T, *PageMeta, error) {
		return lister(ctx, size, page, filter)
	})
}

// validatePaginationParams validates common pagination parameters
func validatePaginationParams(pageSize, pageNumber int64) error {
	if pageSize <= 0 {
		return fmt.Errorf("pageSize must be positive, got %d", pageSize)
	}
	if pageNumber <= 0 {
		return fmt.Errorf("pageNumber must be positive, got %d", pageNumber)
	}
	if pageSize > 1000 {
		return fmt.Errorf("pageSize too large, maximum is 1000, got %d", pageSize)
	}
	return nil
}
