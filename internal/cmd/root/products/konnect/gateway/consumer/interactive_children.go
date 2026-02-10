package consumer

import (
	"context"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/charmbracelet/bubbles/table"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	gatewaycommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/common"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

func init() {
	tableview.RegisterChildLoader("control-plane", "consumers", loadControlPlaneConsumers)
}

func loadControlPlaneConsumers(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
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

	consumers, err := fetchAllConsumers(helper, cfg, konnectSDK.SDK, controlPlaneID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return tableview.ChildView{}, cmd.PrepareExecutionError(
			"Failed to list Gateway Consumers",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}

	rows := make([]table.Row, 0, len(consumers))
	for i := range consumers {
		record := consumerToDisplayRecord(&consumers[i])
		rows = append(rows, table.Row{record.ID, record.Username})
	}

	detail := func(index int) string {
		if index < 0 || index >= len(consumers) {
			return ""
		}
		return ConsumerDetailView(&consumers[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "USERNAME"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Consumers",
		ParentType:     "gateway-consumer",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(consumers) {
				return nil
			}
			consumerID := ""
			if consumers[index].GetID() != nil {
				consumerID = strings.TrimSpace(*consumers[index].GetID())
			}
			return &gatewaycommon.ConsumerContext{
				ControlPlaneID: controlPlaneID,
				ConsumerID:     consumerID,
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
