package helpers

import (
	"context"
	"fmt"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

func GetControlPlaneID(ctx context.Context, kkClient *kk.SDK, cpName string) (string, error) {
	var pageNumber, requestPageSize int64 = 1, 1

	req := kkOps.ListControlPlanesRequest{
		PageSize:     kk.Int64(requestPageSize),
		PageNumber:   kk.Int64(pageNumber),
		FilterNameEq: kk.String(cpName),
	}

	res, err := kkClient.ControlPlanes.ListControlPlanes(ctx, req)
	if err != nil {
		return "", err
	}

	if len(res.ListControlPlanesResponse.Data) != 1 {
		return "", fmt.Errorf("a control plane with name %s not found", cpName)
	}

	return res.ListControlPlanesResponse.Data[0].ID, nil
}
