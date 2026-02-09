package organization

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/navigator"
)

// BuildListView returns the interactive organization view configuration used by the Konnect
// navigator when the user selects the "organization" resource.
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

	org, err := runGetOrganization(sdk.GetMeAPI(), helper)
	if err != nil {
		return tableview.ChildView{}, err
	}
	if org == nil {
		return tableview.ChildView{}, fmt.Errorf("organization response was empty")
	}

	return buildOrganizationChildView(org), nil
}

func init() {
	navigator.RegisterResource(
		"organization",
		[]string{"organization", "org"},
		BuildListView,
	)
}
