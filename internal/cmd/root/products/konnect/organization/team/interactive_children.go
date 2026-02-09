package team

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
)

func init() {
	tableview.RegisterChildLoader("organization", "teams", loadOrganizationTeams)
}

func loadOrganizationTeams(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
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

	teams, err := runList(sdk.GetOrganizationTeamAPI(), helper, cfg, false)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildTeamChildView(teams), nil
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
