package helpers

import (
	"context"

	kkInternal "github.com/Kong/sdk-konnect-go-internal"
	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
	kkInternalOps "github.com/Kong/sdk-konnect-go-internal/models/operations"
)

func GetAllPortals(ctx context.Context, requestPageSize int64, kkClient *kkInternal.SDK,
) ([]kkInternalComps.PortalV3, error) {
	var allData []kkInternalComps.PortalV3

	var pageNumber int64 = 1
	for {
		req := kkInternalOps.ListPortalsRequest{
			PageSize:   kkInternal.Int64(requestPageSize),
			PageNumber: kkInternal.Int64(pageNumber),
		}

		res, err := kkClient.V3Portals.ListPortals(ctx, req)
		if err != nil {
			return nil, err
		}

		if res.ListPortalsResponseV3 != nil && len(res.ListPortalsResponseV3.Data) > 0 {
			allData = append(allData, res.ListPortalsResponseV3.Data...)
			pageNumber++
		} else {
			break
		}
	}

	return allData, nil
}
