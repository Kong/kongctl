package state

import (
	"context"
	"fmt"
)

// PageMeta contains pagination metadata from API responses
type PageMeta struct {
	Total float64
}

const maxPaginationPages int64 = 10000

// PaginatedLister is a function type that can fetch a page of results
type PaginatedLister[T any] func(ctx context.Context, pageSize, pageNumber int64) ([]T, *PageMeta, error)

// PaginateAll fetches all pages from a paginated API endpoint
func PaginateAll[T any](ctx context.Context, lister PaginatedLister[T]) ([]T, error) {
	var allResults []T
	var pageNumber int64 = 1
	pageSize := int64(100)

	for {
		if pageNumber > maxPaginationPages {
			return nil, fmt.Errorf("pagination exceeded safety limit of %d pages", maxPaginationPages)
		}

		// Fetch the current page
		pageResults, meta, err := lister(ctx, pageSize, pageNumber)
		if err != nil {
			return nil, err
		}

		// Add results to our collection
		allResults = append(allResults, pageResults...)

		// Without metadata, the helper cannot prove whether more raw pages exist.
		if meta == nil {
			break
		}

		// Completion must be based on raw-page progress, not on the filtered
		// result count returned by the caller.
		expectedTotalFetched := pageSize * pageNumber
		if meta.Total <= float64(expectedTotalFetched) {
			break
		}

		pageNumber++
	}

	return allResults, nil
}

// FilteredPaginatedLister wraps a PaginatedLister with additional filtering logic
type FilteredPaginatedLister[T any] func(
	ctx context.Context, pageSize, pageNumber int64, filter func(T) bool,
) ([]T, *PageMeta, error)

// PaginateAllFiltered fetches all pages from a paginated API endpoint with filtering
func PaginateAllFiltered[T any](
	ctx context.Context, lister FilteredPaginatedLister[T], filter func(T) bool,
) ([]T, error) {
	var allResults []T
	var pageNumber int64 = 1
	pageSize := int64(100)

	for {
		if pageNumber > maxPaginationPages {
			return nil, fmt.Errorf("pagination exceeded safety limit of %d pages", maxPaginationPages)
		}

		// Fetch the current page with filtering
		pageResults, meta, err := lister(ctx, pageSize, pageNumber, filter)
		if err != nil {
			return nil, err
		}

		// Add results to our collection
		allResults = append(allResults, pageResults...)

		// Without metadata, the helper cannot prove whether more raw pages exist.
		if meta == nil {
			break
		}

		// Filtering can make page-local result counts sparse or empty, so use the
		// raw total to determine whether all source pages have been traversed.
		expectedTotalFetched := pageSize * pageNumber
		if meta.Total <= float64(expectedTotalFetched) {
			break
		}

		pageNumber++
	}

	return allResults, nil
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
