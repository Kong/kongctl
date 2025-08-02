package common

import (
	"context"
	"fmt"
)

// PageMeta represents pagination metadata returned by APIs
type PageMeta struct {
	Total       float64 `json:"total"`
	Page        int64   `json:"page"`
	PageSize    int64   `json:"page_size"`
	HasNextPage bool    `json:"has_next_page"`
}

// PaginationHandler defines a function that performs paginated requests
// Returns: (items, pageInfo, error)
type PaginationHandler[T any] func(pageNumber int64) ([]T, *PageMeta, error)

// PaginatedList handles pagination logic for any paginated API
func PaginatedList[T any](ctx context.Context, handler PaginationHandler[T]) ([]T, error) {
	var allItems []T
	pageNumber := int64(1)

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		items, meta, err := handler(pageNumber)
		if err != nil {
			return nil, fmt.Errorf("pagination failed on page %d: %w", pageNumber, err)
		}

		// Add items to result
		allItems = append(allItems, items...)

		// Check if we have more pages
		if meta == nil || !hasMorePages(meta) {
			break
		}

		pageNumber++
	}

	return allItems, nil
}

// hasMorePages checks if there are more pages to fetch based on metadata
func hasMorePages(meta *PageMeta) bool {
	if meta.HasNextPage {
		return true
	}
	
	// Fallback calculation if HasNextPage is not set
	if meta.Total > 0 && meta.PageSize > 0 {
		return meta.Total > float64(meta.PageSize*meta.Page)
	}
	
	return false
}

// SimplePaginationHandler creates a handler for simple pagination scenarios
// where you just need to call a function with page number and size
type SimplePaginationFunc[T any] func(pageNumber, pageSize int64) ([]T, *PageMeta, error)

func NewSimplePaginationHandler[T any](pageSize int64, fn SimplePaginationFunc[T]) PaginationHandler[T] {
	return func(pageNumber int64) ([]T, *PageMeta, error) {
		return fn(pageNumber, pageSize)
	}
}

// BatchProcessor processes items in batches with a given batch size
type BatchProcessor[T any] func(batch []T) error

// ProcessInBatches processes a slice of items in batches
func ProcessInBatches[T any](items []T, batchSize int, processor BatchProcessor[T]) error {
	if batchSize <= 0 {
		return fmt.Errorf("batch size must be positive, got %d", batchSize)
	}

	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}

		batch := items[i:end]
		if err := processor(batch); err != nil {
			return fmt.Errorf("batch processing failed at batch starting at index %d: %w", i, err)
		}
	}

	return nil
}