package aigateway

import (
	"context"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
)

func init() {
	tableview.RegisterChildLoader(common.ViewParentAIGateway, common.ViewFieldProviders, loadAIGatewayProviders)
	tableview.RegisterChildLoader(common.ViewParentAIGateway, common.ViewFieldPolicies, loadAIGatewayPolicies)
	tableview.RegisterChildLoader(common.ViewParentAIGateway, common.ViewFieldConsumerGroups, loadAIGatewayConsumerGroups)
}

func loadAIGatewayProviders(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, err := aiGatewayIDFromParent(parent)
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

	providerAPI := sdk.GetAIGatewayProvidersAPI()
	if providerAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("AI Gateway Providers client is not available")
	}

	providers, err := fetchAIGatewayProviders(helper, providerAPI, gatewayID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}
	return buildAIGatewayProviderChildView(providers), nil
}

func loadAIGatewayPolicies(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, err := aiGatewayIDFromParent(parent)
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

	policyAPI := sdk.GetAIGatewayPoliciesAPI()
	if policyAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("AI Gateway Policies client is not available")
	}

	policies, err := fetchAIGatewayPolicies(helper, policyAPI, gatewayID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}
	return buildAIGatewayPolicyChildView(policies), nil
}

func loadAIGatewayConsumerGroups(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	gatewayID, err := aiGatewayIDFromParent(parent)
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

	groupAPI := sdk.GetAIGatewayConsumerGroupsAPI()
	if groupAPI == nil {
		return tableview.ChildView{}, fmt.Errorf("AI Gateway Consumer Groups client is not available")
	}

	groups, err := fetchAIGatewayConsumerGroups(helper, groupAPI, gatewayID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}
	return buildAIGatewayConsumerGroupChildView(groups), nil
}

func aiGatewayIDFromParent(parent any) (string, error) {
	if parent == nil {
		return "", fmt.Errorf("AI Gateway parent is nil")
	}

	switch p := parent.(type) {
	case *kkComps.AIGateway:
		id := strings.TrimSpace(p.ID)
		if id == "" {
			return "", fmt.Errorf("AI Gateway identifier is missing")
		}
		return id, nil
	case kkComps.AIGateway:
		id := strings.TrimSpace(p.ID)
		if id == "" {
			return "", fmt.Errorf("AI Gateway identifier is missing")
		}
		return id, nil
	default:
		return "", fmt.Errorf("unexpected parent type %T", parent)
	}
}
