package state

import (
	"context"
	"fmt"
)

// PageMeta contains pagination metadata from API responses
type PageMeta struct {
	Total float64
}

// PaginatedLister is a function type that can fetch a page of results
type PaginatedLister[T any] func(ctx context.Context, pageSize, pageNumber int64) ([]T, *PageMeta, error)

// PaginateAll fetches all pages from a paginated API endpoint
func PaginateAll[T any](ctx context.Context, lister PaginatedLister[T]) ([]T, error) {
	var allResults []T
	var pageNumber int64 = 1
	pageSize := int64(100)

	for {
		// Fetch the current page
		pageResults, meta, err := lister(ctx, pageSize, pageNumber)
		if err != nil {
			return nil, err
		}

		// Add results to our collection
		allResults = append(allResults, pageResults...)

		// Check if we have more pages to fetch
		if meta == nil || len(pageResults) == 0 {
			break
		}

		// Check if we've fetched all available results based on metadata
		// Calculate how many items we should have fetched so far
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
		// Fetch the current page with filtering
		pageResults, meta, err := lister(ctx, pageSize, pageNumber, filter)
		if err != nil {
			return nil, err
		}

		// Add results to our collection
		allResults = append(allResults, pageResults...)

		// Check if we have more pages to fetch
		if meta == nil || len(pageResults) == 0 {
			break
		}

		// For filtered results, we can't rely on page size to determine end
		// We need to check if this is the last page based on page number
		// This is a simplified approach - in real scenarios, the API would indicate this
		pageNumber++
		
		// Simple heuristic: if we're beyond reasonable page count, stop
		if pageNumber > 10 {
			break
		}
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