package helpers

import "fmt"

const maxPaginationPages int64 = 10000

func paginateAllPageNumber[T any](fetchPage func(pageSize, pageNumber int64) ([]T, float64, error)) ([]T, error) {
	const pageSize int64 = 100

	var (
		all        []T
		pageNumber int64 = 1
	)

	for {
		if pageNumber > maxPaginationPages {
			return nil, fmt.Errorf("pagination exceeded safety limit of %d pages", maxPaginationPages)
		}

		pageItems, total, err := fetchPage(pageSize, pageNumber)
		if err != nil {
			return nil, err
		}

		all = append(all, pageItems...)

		if total <= float64(pageSize*pageNumber) {
			break
		}

		pageNumber++
	}

	return all, nil
}
