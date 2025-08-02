package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkCOM "github.com/Kong/sdk-konnect-go/models/components"
	kkOPS "github.com/Kong/sdk-konnect-go/models/operations"
)

type ControlPlaneAPI interface {
	ListControlPlanes(ctx context.Context, request kkOPS.ListControlPlanesRequest,
		opts ...kkOPS.Option) (*kkOPS.ListControlPlanesResponse, error)
	CreateControlPlane(ctx context.Context, request kkCOM.CreateControlPlaneRequest,
		opts ...kkOPS.Option) (*kkOPS.CreateControlPlaneResponse, error)
	GetControlPlane(ctx context.Context, id string,
		opts ...kkOPS.Option) (*kkOPS.GetControlPlaneResponse, error)
	UpdateControlPlane(ctx context.Context, id string, updateControlPlaneRequest kkCOM.UpdateControlPlaneRequest,
		opts ...kkOPS.Option) (*kkOPS.UpdateControlPlaneResponse, error)
	DeleteControlPlane(ctx context.Context, id string,
		opts ...kkOPS.Option) (*kkOPS.DeleteControlPlaneResponse, error)
}

func GetControlPlaneID(ctx context.Context, kkClient ControlPlaneAPI, cpName string) (string, error) {
	var pageNumber, requestPageSize int64 = 1, 1

	req := kkOPS.ListControlPlanesRequest{
		PageSize:   kkSDK.Int64(requestPageSize),
		PageNumber: kkSDK.Int64(pageNumber),
		Filter: &kkCOM.ControlPlaneFilterParameters{
			Name: &kkCOM.Name{
				Eq: kkSDK.String(cpName),
			},
		},
	}

	res, err := kkClient.ListControlPlanes(ctx, req)
	if err != nil {
		return "", err
	}

	if len(res.ListControlPlanesResponse.Data) != 1 {
		return "", fmt.Errorf("a control plane with name %s not found", cpName)
	}

	return res.ListControlPlanesResponse.Data[0].ID, nil
}
