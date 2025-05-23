package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkOPS "github.com/Kong/sdk-konnect-go/models/operations"
)

type AppAuthStrategiesAPI interface {
	ListAppAuthStrategies(ctx context.Context, request kkOPS.ListAppAuthStrategiesRequest,
		opts ...kkOPS.Option) (*kkOPS.ListAppAuthStrategiesResponse, error)
}

// GetAllAppAuthStrategies fetches all app auth strategies with pagination
func GetAllAppAuthStrategies(ctx context.Context, kkClient AppAuthStrategiesAPI) ([]interface{}, error) {
	var allStrategies []interface{}
	var pageNumber int64 = 1
	requestPageSize := int64(100) // Use a reasonable page size

	for {
		req := kkOPS.ListAppAuthStrategiesRequest{
			PageSize:   kkSDK.Int64(requestPageSize),
			PageNumber: kkSDK.Int64(pageNumber),
		}

		res, err := kkClient.ListAppAuthStrategies(ctx, req)
		if err != nil {
			return nil, err
		}

		if res.ListAppAuthStrategiesResponse == nil ||
			len(res.ListAppAuthStrategiesResponse.Data) == 0 {
			break
		}

		// Add the strategies to our collection
		for _, strategy := range res.ListAppAuthStrategiesResponse.Data {
			allStrategies = append(allStrategies, strategy)
		}

		// Check if we have more pages
		if res.ListAppAuthStrategiesResponse.Meta.Page.Total <=
			float64(requestPageSize*(pageNumber)) {
			break
		}

		pageNumber++
	}

	return allStrategies, nil
}
