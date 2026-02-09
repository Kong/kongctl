package systemaccount

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
)

func init() {
	tableview.RegisterChildLoader("organization", "system-accounts", loadOrganizationSystemAccounts)
}

func loadOrganizationSystemAccounts(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	if _, err := organizationFromParent(parent); err != nil {
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

	accounts, err := runList(sdk.GetSystemAccountAPI(), helper, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildSystemAccountChildView(accounts), nil
}

func organizationFromParent(parent any) (*kkComps.MeOrganization, error) {
	if parent == nil {
		return nil, fmt.Errorf("organization parent is nil")
	}

	switch org := parent.(type) {
	case *kkComps.MeOrganization:
		return org, nil
	case kkComps.MeOrganization:
		return &org, nil
	default:
		return nil, fmt.Errorf("unexpected parent type %T", parent)
	}
}
