package controlplane

import (
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/navigator"
)

// BuildListView returns the interactive control plane view configuration used by the Konnect
// navigator for the "control-planes" resource.
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

	cps, err := runList(sdk.GetControlPlaneAPI(), helper, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildControlPlaneChildView(cps), nil
}

func init() {
	navigator.RegisterResource(
		"control-planes",
		[]string{"control-planes", "gateway control-planes"},
		BuildListView,
	)
}
