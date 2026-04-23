package helpers

func paginateAllPageNumber[T any](fetchPage func(pageSize, pageNumber int64) ([]T, float64, error)) ([]T, error) {
	const pageSize int64 = 100

	var (
		all        []T
		pageNumber int64 = 1
	)

	for {
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
