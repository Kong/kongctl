package service

import (
	"context"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/charmbracelet/bubbles/table"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	kkCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	gatewaycommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/common"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

func init() {
	tableview.RegisterChildLoader("control-plane", "services", loadControlPlaneServices)
}

func loadControlPlaneServices(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
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
	services, err := helpers.GetAllGatewayServices(helper.GetContext(), requestPageSize, controlPlaneID, konnectSDK.SDK)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return tableview.ChildView{}, cmd.PrepareExecutionError(
			"Failed to list Gateway Services",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}

	rows := make([]table.Row, 0, len(services))
	for i := range services {
		record := serviceToDisplayRecord(&services[i])
		rows = append(rows, table.Row{record.ID, record.Name})
	}

	detail := func(index int) string {
		if index < 0 || index >= len(services) {
			return ""
		}
		return serviceDetailView(&services[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Services",
		ParentType:     "gateway-service",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(services) {
				return nil
			}
			serviceID := ""
			if services[index].ID != nil {
				serviceID = strings.TrimSpace(*services[index].ID)
			}
			return &gatewaycommon.ServiceContext{
				ControlPlaneID: controlPlaneID,
				ServiceID:      serviceID,
			}
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
