package aigateway

import (
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/navigator"
)

// BuildListView returns the interactive AI Gateway list view configuration used by the Konnect navigator.
func BuildListView(helper cmd.Helper) (tableview.ChildView, error) {
	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	gateways, err := runList(sdk.GetAIGatewayAPI(), helper, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildAIGatewayChildView(gateways), nil
}

func init() {
	navigator.RegisterResource(
		common.ViewResourceAIGateways,
		[]string{common.ViewParentAIGateway, common.ViewAliasAIGateways, common.ViewAliasAIGatewayShort},
		BuildListView,
	)
}
