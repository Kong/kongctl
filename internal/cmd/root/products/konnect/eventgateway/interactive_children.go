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
	tableview.RegisterChildLoader("event-gateway", "data-plane-certificates", loadEventGatewayDataPlaneCertificates)
	tableview.RegisterChildLoader("event-gateway", "listeners", loadEventGatewayListeners)
	tableview.RegisterChildLoader("listener", "policies", loadEventGatewayListenerPolicies)
	tableview.RegisterChildLoader("virtual-cluster", "cluster-policies", loadEventGatewayVirtualClusterClusterPolicies)
	tableview.RegisterChildLoader("virtual-cluster", "produce-policies", loadEventGatewayVirtualClusterProducePolicies)
	tableview.RegisterChildLoader("virtual-cluster", "consume-policies", loadEventGatewayVirtualClusterConsumePolicies)
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

	return buildVirtualClusterChildView(clusters, gatewayID), nil
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

func loadEventGatewayListeners(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
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

	listenerAPI := sdk.GetEventGatewayListenerAPI()
	if listenerAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("event gateway listener client is not available")
	}

	listeners, err := fetchListeners(helper, listenerAPI, gatewayID, cfg, "")
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildListenerChildView(listeners, gatewayID), nil
}

func loadEventGatewayListenerPolicies(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, listenerID, err := listenerIDsFromParent(parent)
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

	policyAPI := sdk.GetEventGatewayListenerPolicyAPI()
	if policyAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("event gateway listener policy client is not available")
	}

	_, rawPolicies, err := fetchListenerPolicies(helper, policyAPI, gatewayID, listenerID, cfg, "")
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildListenerPolicyChildView(rawPolicies), nil
}

func listenerIDsFromParent(parent any) (string, string, error) {
	if parent == nil {
		return "", "", fmt.Errorf("listener parent is nil")
	}

	switch p := parent.(type) {
	case *ListenerWithGateway:
		if p.EventGatewayListener == nil {
			return "", "", fmt.Errorf("listener is nil")
		}
		gatewayID := strings.TrimSpace(p.EventGatewayID)
		if gatewayID == "" {
			return "", "", fmt.Errorf("event gateway identifier is missing from listener")
		}
		listenerID := strings.TrimSpace(p.ID)
		if listenerID == "" {
			return "", "", fmt.Errorf("listener identifier is missing")
		}
		return gatewayID, listenerID, nil
	case ListenerWithGateway:
		if p.EventGatewayListener == nil {
			return "", "", fmt.Errorf("listener is nil")
		}
		gatewayID := strings.TrimSpace(p.EventGatewayID)
		if gatewayID == "" {
			return "", "", fmt.Errorf("event gateway identifier is missing from listener")
		}
		listenerID := strings.TrimSpace(p.ID)
		if listenerID == "" {
			return "", "", fmt.Errorf("listener identifier is missing")
		}
		return gatewayID, listenerID, nil
	default:
		return "", "", fmt.Errorf("unexpected parent type %T", parent)
	}
}

func loadEventGatewayVirtualClusterClusterPolicies(
	_ context.Context,
	helper cmd.Helper,
	parent any,
) (tableview.ChildView, error) {
	gatewayID, virtualClusterID, err := virtualClusterIDsFromParent(parent)
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

	policyAPI := sdk.GetEventGatewayClusterPolicyAPI()
	if policyAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("event gateway cluster policy client is not available")
	}

	_, rawPolicies, err := fetchClusterPolicies(helper, policyAPI, gatewayID, virtualClusterID, cfg, "")
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildClusterPolicyChildView(rawPolicies), nil
}

func loadEventGatewayVirtualClusterProducePolicies(
	_ context.Context,
	helper cmd.Helper,
	parent any,
) (tableview.ChildView, error) {
	gatewayID, virtualClusterID, err := virtualClusterIDsFromParent(parent)
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

	policyAPI := sdk.GetEventGatewayProducePolicyAPI()
	if policyAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("event gateway produce policy client is not available")
	}

	_, rawPolicies, err := fetchProducePolicies(helper, policyAPI, gatewayID, virtualClusterID, cfg, "")
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildProducePolicyChildView(rawPolicies), nil
}

func loadEventGatewayVirtualClusterConsumePolicies(
	_ context.Context,
	helper cmd.Helper,
	parent any,
) (tableview.ChildView, error) {
	gatewayID, virtualClusterID, err := virtualClusterIDsFromParent(parent)
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

	policyAPI := sdk.GetEventGatewayConsumePolicyAPI()
	if policyAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("event gateway consume policy client is not available")
	}

	_, rawPolicies, err := fetchConsumePolicies(helper, policyAPI, gatewayID, virtualClusterID)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildConsumePolicyChildView(rawPolicies), nil
}

func virtualClusterIDsFromParent(parent any) (string, string, error) {
	if parent == nil {
		return "", "", fmt.Errorf("virtual cluster parent is nil")
	}

	switch p := parent.(type) {
	case *VirtualClusterWithGateway:
		if p.VirtualCluster == nil {
			return "", "", fmt.Errorf("virtual cluster is nil")
		}
		gatewayID := strings.TrimSpace(p.EventGatewayID)
		if gatewayID == "" {
			return "", "", fmt.Errorf("event gateway identifier is missing from virtual cluster")
		}
		virtualClusterID := strings.TrimSpace(p.ID)
		if virtualClusterID == "" {
			return "", "", fmt.Errorf("virtual cluster identifier is missing")
		}
		return gatewayID, virtualClusterID, nil
	case VirtualClusterWithGateway:
		if p.VirtualCluster == nil {
			return "", "", fmt.Errorf("virtual cluster is nil")
		}
		gatewayID := strings.TrimSpace(p.EventGatewayID)
		if gatewayID == "" {
			return "", "", fmt.Errorf("event gateway identifier is missing from virtual cluster")
		}
		virtualClusterID := strings.TrimSpace(p.ID)
		if virtualClusterID == "" {
			return "", "", fmt.Errorf("virtual cluster identifier is missing")
		}
		return gatewayID, virtualClusterID, nil
	default:
		return "", "", fmt.Errorf("unexpected parent type %T", parent)
	}
}

func loadEventGatewayDataPlaneCertificates(
	_ context.Context,
	helper cmd.Helper,
	parent any,
) (tableview.ChildView, error) {
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

	certAPI := sdk.GetEventGatewayDataPlaneCertificateAPI()
	if certAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("event gateway data plane certificates client is not available")
	}

	certs, err := fetchDataPlaneCertificates(helper, certAPI, gatewayID, cfg, "")
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildDataPlaneCertChildView(certs), nil
}
