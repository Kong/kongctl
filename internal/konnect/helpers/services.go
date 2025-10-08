package helpers

import (
	"context"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// GatewayServiceAPI defines the interface for gateway service operations needed by the CLI.
type GatewayServiceAPI interface {
	ListService(ctx context.Context, request kkOps.ListServiceRequest,
		opts ...kkOps.Option) (*kkOps.ListServiceResponse, error)
}

func GetAllGatewayServices(ctx context.Context, requestPageSize int64, cpID string, kkClient *kk.SDK,
) ([]kkComps.ServiceOutput, error) {
	var allData []kkComps.ServiceOutput

	offset := ""
	for {
		req := kkOps.ListServiceRequest{
			Size:           kk.Int64(requestPageSize),
			ControlPlaneID: cpID,
			Offset:         kk.String(offset),
		}

		res, err := kkClient.Services.ListService(ctx, req)
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
