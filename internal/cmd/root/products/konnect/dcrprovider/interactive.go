package dcrprovider

import (
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/navigator"
)

// BuildListView returns the interactive DCR provider list view configuration used by the Konnect
// navigator when the "dcr-providers" resource is selected.
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

	providers, err := runList(sdk.GetDCRProvidersAPI(), helper, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildDCRProviderChildView(providers), nil
}

func init() {
	navigator.RegisterResource(
		common.ViewResourceDCRProviders,
		[]string{common.ViewAliasDCRProviders, common.ViewParentDCRProvider},
		BuildListView,
	)
}
