package route

import (
	"context"
	"fmt"
	"strings"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/charmbracelet/bubbles/table"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	kkCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	gatewaycommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/common"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

func init() {
	tableview.RegisterChildLoader("control-plane", "routes", loadControlPlaneRoutes)
	tableview.RegisterChildLoader("gateway-service", "routes", loadServiceRoutes)
}

func loadControlPlaneRoutes(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	controlPlaneID, err := controlPlaneIDFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	konnectSDK, ok := sdk.(*helpers.KonnectSDK)
	if !ok || konnectSDK.SDK == nil {
		return tableview.ChildView{}, fmt.Errorf("konnect SDK is not available")
	}

	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))
	routes, err := helpers.GetAllGatewayRoutes(helper.GetContext(), requestPageSize, controlPlaneID, konnectSDK.SDK)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return tableview.ChildView{}, cmd.PrepareExecutionError(
			"Failed to list Gateway Routes",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}

	rows := make([]table.Row, 0, len(routes))
	for i := range routes {
		record := textDisplayRecord{ID: "n/a", Name: "n/a"}
		if routes[i].RouteJSON != nil {
			record = jsonRouteToDisplayRecord(routes[i].RouteJSON)
		}
		rows = append(rows, table.Row{record.ID, record.Name})
	}

	detail := func(index int) string {
		if index < 0 || index >= len(routes) {
			return ""
		}
		return routeDetailView(routes[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Routes",
		ParentType:     "gateway-route",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(routes) {
				return nil
			}
			routeID := ""
			if routes[index].RouteJSON != nil && routes[index].RouteJSON.GetID() != nil {
				routeID = strings.TrimSpace(*routes[index].RouteJSON.GetID())
			}
			return &gatewaycommon.RouteContext{
				ControlPlaneID: controlPlaneID,
				RouteID:        routeID,
			}
		},
	}, nil
}

func loadServiceRoutes(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	ctx, err := gatewaycommon.ServiceContextFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	konnectSDK, ok := sdk.(*helpers.KonnectSDK)
	if !ok || konnectSDK.SDK == nil {
		return tableview.ChildView{}, fmt.Errorf("konnect SDK is not available")
	}

	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))
	routes, err := listGatewayRoutesForService(
		helper.GetContext(),
		requestPageSize,
		ctx.ControlPlaneID,
		ctx.ServiceID,
		konnectSDK.SDK,
	)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return tableview.ChildView{}, cmd.PrepareExecutionError(
			"Failed to list Gateway Routes",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}

	rows := make([]table.Row, 0, len(routes))
	for i := range routes {
		record := textDisplayRecord{ID: "n/a", Name: "n/a"}
		if routes[i].RouteJSON != nil {
			record = jsonRouteToDisplayRecord(routes[i].RouteJSON)
		}
		rows = append(rows, table.Row{record.ID, record.Name})
	}

	detail := func(index int) string {
		if index < 0 || index >= len(routes) {
			return ""
		}
		return routeDetailView(routes[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Routes",
		ParentType:     "gateway-route",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(routes) {
				return nil
			}
			return &routes[index]
		},
	}, nil
}

func controlPlaneIDFromParent(parent any) (string, error) {
	if parent == nil {
		return "", fmt.Errorf("control plane parent is nil")
	}

	switch cp := parent.(type) {
	case *kkComps.ControlPlane:
		id := strings.TrimSpace(cp.ID)
		if id == "" {
			return "", fmt.Errorf("control plane identifier is missing")
		}
		return id, nil
	case kkComps.ControlPlane:
		id := strings.TrimSpace(cp.ID)
		if id == "" {
			return "", fmt.Errorf("control plane identifier is missing")
		}
		return id, nil
	default:
		return "", fmt.Errorf("unexpected parent type %T", parent)
	}
}

func listGatewayRoutesForService(
	ctx context.Context,
	requestPageSize int64,
	controlPlaneID string,
	serviceID string,
	kkClient *kk.SDK,
) ([]kkComps.Route, error) {
	var allData []kkComps.Route
	offset := ""

	for {
		req := kkOps.ListRouteWithServiceRequest{
			ControlPlaneID: controlPlaneID,
			ServiceID:      serviceID,
			Size:           kk.Int64(requestPageSize),
		}
		if offset != "" {
			req.Offset = kk.String(offset)
		}

		res, err := kkClient.Routes.ListRouteWithService(ctx, req)
		if err != nil {
			return nil, err
		}
		if res.Object == nil {
			break
		}

		allData = append(allData, res.Object.GetData()...)

		if res.Object.Offset != nil && strings.TrimSpace(*res.Object.Offset) != "" {
			offset = strings.TrimSpace(*res.Object.Offset)
			continue
		}
		break
	}

	return allData, nil
}
