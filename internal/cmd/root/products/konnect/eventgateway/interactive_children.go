package eventgateway

import (
	"context"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
)

func init() {
	if !eventGatewayViewEnabled() {
		return
	}
	tableview.RegisterChildLoader("event-gateway", "backend-clusters", loadEventGatewayBackendClusters)
	tableview.RegisterChildLoader("event-gateway", "virtual-clusters", loadEventGatewayVirtualClusters)
}

func loadEventGatewayBackendClusters(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, err := eventGatewayIDFromParent(parent)
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

	clusterAPI := sdk.GetEventGatewayBackendClusterAPI()
	if clusterAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("event gateway backend clusters client is not available")
	}

	clusters, err := fetchBackendClusters(helper, clusterAPI, gatewayID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildBackendClusterChildView(clusters), nil
}

func loadEventGatewayVirtualClusters(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, err := eventGatewayIDFromParent(parent)
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

	clusterAPI := sdk.GetEventGatewayVirtualClusterAPI()
	if clusterAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("event gateway virtual clusters client is not available")
	}

	clusters, err := fetchVirtualClusters(helper, clusterAPI, gatewayID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildVirtualClusterChildView(clusters), nil
}

func eventGatewayIDFromParent(parent any) (string, error) {
	if parent == nil {
		return "", fmt.Errorf("event gateway parent is nil")
	}

	switch p := parent.(type) {
	case *kkComps.EventGatewayInfo:
		id := strings.TrimSpace(p.ID)
		if id == "" {
			return "", fmt.Errorf("event gateway identifier is missing")
		}
		return id, nil
	case kkComps.EventGatewayInfo:
		id := strings.TrimSpace(p.ID)
		if id == "" {
			return "", fmt.Errorf("event gateway identifier is missing")
		}
		return id, nil
	default:
		return "", fmt.Errorf("unexpected parent type %T", parent)
	}
}
