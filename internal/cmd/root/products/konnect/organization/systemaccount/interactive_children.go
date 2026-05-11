package systemaccount

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
)

func init() {
	tableview.RegisterChildLoader(
		common.ViewParentOrganization,
		common.ViewFieldSystemAccounts,
		loadOrganizationSystemAccounts,
	)
	tableview.RegisterChildLoader(
		common.ViewParentSystemAccount,
		common.ViewFieldUserRoles,
		loadSystemAccountRolesForSystemAccount,
	)
	tableview.RegisterChildLoader(
		common.ViewParentSystemAccount,
		common.ViewFieldTeams,
		loadSystemAccountTeamsForSystemAccount,
	)
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

func loadSystemAccountRolesForSystemAccount(
	_ context.Context,
	helper cmd.Helper,
	parent any,
) (tableview.ChildView, error) {
	account, err := systemAccountFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}
	if account.ID == nil || *account.ID == "" {
		return tableview.ChildView{}, fmt.Errorf("system account ID is required to load roles")
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

	roles, err := fetchSystemAccountRoles(helper, sdk.GetSystemAccountRolesAPI(), *account.ID)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildSystemAccountRolesChildView(*account.ID, roles), nil
}

func loadSystemAccountTeamsForSystemAccount(
	_ context.Context,
	helper cmd.Helper,
	parent any,
) (tableview.ChildView, error) {
	account, err := systemAccountFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}
	if account.ID == nil || *account.ID == "" {
		return tableview.ChildView{}, fmt.Errorf("system account ID is required to load teams")
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

	teams, err := fetchSystemAccountTeams(helper, sdk.GetSystemAccountTeamMembershipAPI(), *account.ID, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildSystemAccountTeamsChildView(*account.ID, teams), nil
}

func systemAccountFromParent(parent any) (*kkComps.SystemAccount, error) {
	if parent == nil {
		return nil, fmt.Errorf("system account parent is nil")
	}

	switch account := parent.(type) {
	case *kkComps.SystemAccount:
		return account, nil
	case kkComps.SystemAccount:
		return &account, nil
	default:
		return nil, fmt.Errorf("unexpected parent type %T", parent)
	}
}
