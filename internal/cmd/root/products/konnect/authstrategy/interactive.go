package authstrategy

import (
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/navigator"
)

// BuildListView returns the interactive auth strategy view configuration used by the Konnect
// navigator when the "auth-strategies" resource is selected.
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

	strategies, err := runList("", sdk.GetAppAuthStrategiesAPI(), helper, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildAuthStrategyChildView(strategies), nil
}

func init() {
	navigator.RegisterResource(
		"auth-strategies",
		[]string{"auth-strategies", "application-auth-strategies", "auth-strategy"},
		BuildListView,
	)
}
