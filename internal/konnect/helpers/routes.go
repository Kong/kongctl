package helpers

import (
	"context"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

func GetAllGatewayRoutes(ctx context.Context, requestPageSize int64, cpID string, kkClient *kk.SDK,
) ([]kkComps.Route, error) {
	var allData []kkComps.Route

	offset := ""
	for {
		req := kkOps.ListRouteRequest{
			Size:           new(requestPageSize),
			ControlPlaneID: cpID,
			Offset:         new(offset),
		}

		res, err := kkClient.Routes.ListRoute(ctx, req)
		if err != nil {
			return nil, err
		}

		if res.Object == nil {
			break
		}

		allData = append(allData, res.Object.Data...)

		nextOffset, ok := nextOffsetToken(res.Object.Offset)
		if !ok {
			break
		}
		offset = nextOffset
	}

	return allData, nil
}
