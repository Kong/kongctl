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
