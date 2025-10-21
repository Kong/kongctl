package api

import (
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/navigator"
)

// BuildListView returns the interactive API list view configuration used by the Konnect
// navigator when the user selects the "apis" resource.
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

	apis, err := runList(sdk.GetAPIAPI(), helper, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildAPIChildView(apis), nil
}

func init() {
	navigator.RegisterResource(
		"apis",
		[]string{"apis", "api"},
		BuildListView,
	)
}
