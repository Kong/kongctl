package helpers

import (
	"context"

	kkInternal "github.com/Kong/sdk-konnect-go-internal"
	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
	kkInternalOps "github.com/Kong/sdk-konnect-go-internal/models/operations"
)

// PortalAPI defines the interface for operations on Developer Portals
type PortalAPI interface {
	ListPortals(ctx context.Context, request kkInternalOps.ListPortalsRequest) (*kkInternalOps.ListPortalsResponse, error)
	GetPortal(ctx context.Context, id string) (*kkInternalOps.GetPortalResponse, error)
}

// InternalPortalAPI provides an implementation of the PortalAPI interface using the internal SDK
type InternalPortalAPI struct {
	SDK *kkInternal.SDK
}

// ListPortals implements the PortalAPI interface
func (p *InternalPortalAPI) ListPortals(ctx context.Context, request kkInternalOps.ListPortalsRequest) (*kkInternalOps.ListPortalsResponse, error) {
	return p.SDK.V3Portals.ListPortals(ctx, request)
}

// GetPortal implements the PortalAPI interface
func (p *InternalPortalAPI) GetPortal(ctx context.Context, id string) (*kkInternalOps.GetPortalResponse, error) {
	return p.SDK.V3Portals.GetPortal(ctx, id)
}

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
