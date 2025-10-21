package portal

import (
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/navigator"
)

// BuildListView returns the interactive portal view configuration used by the Konnect
// navigator when the user selects the "portals" resource.
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

	portals, err := runList(sdk.GetPortalAPI(), helper, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildPortalChildView(portals), nil
}

func init() {
	navigator.RegisterResource(
		"portals",
		[]string{"portals", "portal"},
		BuildListView,
	)
}
