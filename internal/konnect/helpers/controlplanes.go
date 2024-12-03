package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkCOM "github.com/Kong/sdk-konnect-go/models/components"
	kkOPS "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/err"
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
		PageSize:     kkSDK.Int64(requestPageSize),
		PageNumber:   kkSDK.Int64(pageNumber),
		FilterNameEq: kkSDK.String(cpName),
	}

	res, e := kkClient.ListControlPlanes(ctx, req)
	if e != nil {
		ee := &err.ExecutionError{
			Err:   e,
			Attrs: err.TryConvertErrorToAttrs(e),
		}
		return "", ee
	}

	if len(res.ListControlPlanesResponse.Data) != 1 {
		return "", &err.ExecutionError{
			Err: fmt.Errorf("a control plane with name %s not found", cpName),
		}
	}

	return res.ListControlPlanesResponse.Data[0].ID, nil
}

func GetControlPlaneIDByNameIfNecessary(ctx context.Context, cpAPI ControlPlaneAPI,
	cpID string, cpName string,
) (string, error) {
	if cpID != "" {
		return cpID, nil
	}

	if cpName == "" {
		return "", &err.ConfigurationError{
			Err: fmt.Errorf("control plane ID or name is required"),
		}
	}

	return GetControlPlaneID(ctx, cpAPI, cpName)
}

//	if cpName == "" {
//		return &err.ConfigurationError{
//			Err: fmt.Errorf("control plane ID or name is required"),
//		}
//	}
//	var err error
//	cpID, err = helpers.GetControlPlaneID(helper.GetContext(), kkClient.GetControlPlaneAPI(), cpName)
//	if err != nil {
//		attrs := cmd.TryConvertErrorToAttrs(err)
//		return cmd.PrepareExecutionError("Failed to get Control Plane ID", err, helper.GetCmd(), attrs...)
//	}
//}
