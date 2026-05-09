package user

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
)

func init() {
	tableview.RegisterChildLoader(common.ViewParentOrganization, common.ViewFieldUsers, loadOrganizationUsers)
	tableview.RegisterChildLoader(
		common.ViewParentOrganizationUser,
		common.ViewFieldUserRoles,
		loadOrganizationUserRolesForUser,
	)
}

func loadOrganizationUsers(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
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

	users, err := runListUsers(sdk.GetOrganizationUsersAPI(), helper, cfg)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildUserChildView(users), nil
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

func loadOrganizationUserRolesForUser(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	orgUser, err := userFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}
	if orgUser.ID == nil || *orgUser.ID == "" {
		return tableview.ChildView{}, fmt.Errorf("user ID is required to load roles")
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

	roles, err := fetchOrganizationUserRoles(helper, sdk.GetOrganizationTeamRolesAPI(), *orgUser.ID)
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildOrganizationUserRolesChildView(*orgUser.ID, roles), nil
}

func userFromParent(parent any) (*kkComps.User, error) {
	if parent == nil {
		return nil, fmt.Errorf("user parent is nil")
	}

	switch orgUser := parent.(type) {
	case *kkComps.User:
		return orgUser, nil
	case kkComps.User:
		return &orgUser, nil
	default:
		return nil, fmt.Errorf("unexpected parent type %T", parent)
	}
}
