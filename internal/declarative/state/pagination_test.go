package state

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestPaginateAll_Success(t *testing.T) {
	// Mock data: first page has full page size (100), second page has remainder (5)
	lister := func(_ context.Context, pageSize, pageNumber int64) ([]string, *PageMeta, error) {
		switch pageNumber {
		case 1:
			// Return a full page to trigger continuation
			items := make([]string, pageSize)
			for i := int64(0); i < pageSize; i++ {
				items[i] = fmt.Sprintf("item%d", i+1)
			}
			return items, &PageMeta{Total: float64(pageSize + 5)}, nil
		case 2:
			// Return partial page to signal end
			return []string{"item101", "item102", "item103", "item104", "item105"}, &PageMeta{Total: 105.0}, nil
		default:
			return []string{}, &PageMeta{Total: 105.0}, nil
		}
	}

	result, err := PaginateAll(context.Background(), lister)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have 100 + 5 = 105 items
	if len(result) != 105 {
		t.Fatalf("Expected 105 items, got %d", len(result))
	}

	// Check first and last few items
	if result[0] != "item1" {
		t.Errorf("Expected first item to be 'item1', got '%s'", result[0])
	}
	if result[99] != "item100" {
		t.Errorf("Expected 100th item to be 'item100', got '%s'", result[99])
	}
	if result[104] != "item105" {
		t.Errorf("Expected last item to be 'item105', got '%s'", result[104])
	}
}

func TestPaginateAll_SinglePage(t *testing.T) {
	lister := func(_ context.Context, _, pageNumber int64) ([]string, *PageMeta, error) {
		if pageNumber > 1 {
			return []string{}, &PageMeta{Total: 2.0}, nil
		}
		return []string{"item1", "item2"}, &PageMeta{Total: 2.0}, nil
	}

	result, err := PaginateAll(context.Background(), lister)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(result))
	}
}

func TestPaginateAll_EmptyResult(t *testing.T) {
	lister := func(_ context.Context, _, _ int64) ([]string, *PageMeta, error) {
		return []string{}, &PageMeta{Total: 0}, nil
	}

	result, err := PaginateAll(context.Background(), lister)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result) != 0 {
		t.Fatalf("Expected 0 items, got %d", len(result))
	}
}

func TestPaginateAll_APIError(t *testing.T) {
	expectedErr := fmt.Errorf("API error")
	lister := func(_ context.Context, _, _ int64) ([]string, *PageMeta, error) {
		return nil, nil, expectedErr
	}

	result, err := PaginateAll(context.Background(), lister)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("Expected API error, got: %v", err)
	}

	if result != nil {
		t.Fatalf("Expected nil result on error, got: %v", result)
	}
}

func TestPaginateAll_PartialError(t *testing.T) {
	lister := func(_ context.Context, pageSize, pageNumber int64) ([]string, *PageMeta, error) {
		if pageNumber == 1 {
			// Return full page to trigger second page request
			items := make([]string, pageSize)
			for i := int64(0); i < pageSize; i++ {
				items[i] = fmt.Sprintf("item%d", i+1)
			}
			return items, &PageMeta{Total: float64(pageSize + 5)}, nil
		}
		// Error on second page
		return nil, nil, fmt.Errorf("network error")
	}

	result, err := PaginateAll(context.Background(), lister)
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	if result != nil {
		t.Fatalf("Expected nil result on error, got: %v", result)
	}
}

func TestPaginateAll_NilMeta(t *testing.T) {
	lister := func(_ context.Context, _, pageNumber int64) ([]string, *PageMeta, error) {
		if pageNumber > 1 {
			return []string{}, nil, nil
		}
		return []string{"item1"}, nil, nil // Nil meta should break
	}

	result, err := PaginateAll(context.Background(), lister)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(result))
	}
}

func TestPaginateAllFiltered_Success(t *testing.T) {
	mockData := [][]string{
		{"apple", "banana", "cherry"},
		{"apricot", "blueberry"},
		{},
	}

	lister := func(_ context.Context, _, pageNumber int64, filter func(string) bool) ([]string, *PageMeta, error) {
		pageIndex := int(pageNumber - 1)
		if pageIndex >= len(mockData) {
			return []string{}, &PageMeta{Total: 5.0}, nil
		}

		page := mockData[pageIndex]

		// Apply filter
		var filtered []string
		for _, item := range page {
			if filter(item) {
				filtered = append(filtered, item)
			}
		}

		meta := &PageMeta{Total: 5.0}
		return filtered, meta, nil
	}

	// Filter for items starting with 'a'
	filter := func(item string) bool {
		return len(item) > 0 && item[0] == 'a'
	}

	result, err := PaginateAllFiltered(context.Background(), lister, filter)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expected := []string{"apple", "apricot"}
	if len(result) != len(expected) {
		t.Fatalf("Expected %d items, got %d", len(expected), len(result))
	}

	for i, item := range expected {
		if result[i] != item {
			t.Errorf("Expected item %d to be %s, got %s", i, item, result[i])
		}
	}
}

func TestValidatePaginationParams(t *testing.T) {
	tests := []struct {
		name       string
		pageSize   int64
		pageNumber int64
		expectErr  bool
	}{
		{"valid params", 100, 1, false},
		{"valid large page", 1000, 5, false},
		{"zero pageSize", 0, 1, true},
		{"negative pageSize", -1, 1, true},
		{"zero pageNumber", 100, 0, true},
		{"negative pageNumber", 100, -1, true},
		{"pageSize too large", 1001, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePaginationParams(tt.pageSize, tt.pageNumber)
			if tt.expectErr && err == nil {
				t.Errorf("Expected error for pageSize=%d, pageNumber=%d", tt.pageSize, tt.pageNumber)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error for pageSize=%d, pageNumber=%d: %v", tt.pageSize, tt.pageNumber, err)
			}
		})
	}
}

// Benchmark pagination with large datasets
func BenchmarkPaginateAll_LargeDataset(b *testing.B) {
	// Create mock data with 10,000 items across 100 pages
	totalItems := 10000

	lister := func(_ context.Context, pageSize, pageNumber int64) ([]string, *PageMeta, error) {
		start := int((pageNumber - 1) * pageSize)
		end := int(pageNumber * pageSize)

		if start >= totalItems {
			return []string{}, &PageMeta{Total: float64(totalItems)}, nil
		}

		if end > totalItems {
			end = totalItems
		}

		var page []string
		for i := start; i < end; i++ {
			page = append(page, fmt.Sprintf("item%d", i))
		}

		return page, &PageMeta{Total: float64(totalItems)}, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := PaginateAll(context.Background(), lister)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
		if len(result) != totalItems {
			b.Fatalf("Expected %d items, got %d", totalItems, len(result))
		}
	}
}
