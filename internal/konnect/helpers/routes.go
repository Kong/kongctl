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
			Size:           kk.Int64(requestPageSize),
			ControlPlaneID: cpID,
			Offset:         kk.String(offset),
		}

		res, err := kkClient.Routes.ListRoute(ctx, req)
		if err != nil {
			return nil, err
		}

		allData = append(allData, res.Object.Data...)

		if res.Object.Offset != nil {
			offset = *res.Object.Offset
		} else {
			break
		}
	}

	return allData, nil
}
