package eventgateway

import (
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/navigator"
)

// BuildListView returns the interactive Event Gateway list view configuration used by the Konnect
// navigator when the user selects the "event-gateways" resource.
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

	gateways, err := runList(sdk.GetEventGatewayControlPlaneAPI(), helper, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildEventGatewayChildView(gateways), nil
}

func init() {
	if !eventGatewayViewEnabled() {
		return
	}
	navigator.RegisterResource(
		"event-gateways",
		[]string{"event-gateway", "event-gateways", "egw"},
		BuildListView,
	)
}
