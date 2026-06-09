package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// SystemAccountRolesAPI defines organization system account role operations.
type SystemAccountRolesAPI interface {
	ListSystemAccountRoles(
		ctx context.Context,
		accountID string,
		filter *kkOps.GetSystemAccountsAccountIDAssignedRolesQueryParamFilter,
		opts ...kkOps.Option,
	) (*kkOps.GetSystemAccountsAccountIDAssignedRolesResponse, error)
	AssignSystemAccountRole(
		ctx context.Context,
		accountID string,
		assignRole *kkComps.AssignRole,
		opts ...kkOps.Option,
	) (*kkOps.PostSystemAccountsAccountIDAssignedRolesResponse, error)
	RemoveSystemAccountRole(
		ctx context.Context,
		accountID string,
		roleID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteSystemAccountsAccountIDAssignedRolesRoleIDResponse, error)
}

// SystemAccountRolesAPIImpl provides an SDK-backed implementation.
type SystemAccountRolesAPIImpl struct {
	SDK *kkSDK.SDK
}

func (s *SystemAccountRolesAPIImpl) ListSystemAccountRoles(
	ctx context.Context,
	accountID string,
	filter *kkOps.GetSystemAccountsAccountIDAssignedRolesQueryParamFilter,
	opts ...kkOps.Option,
) (*kkOps.GetSystemAccountsAccountIDAssignedRolesResponse, error) {
	return s.SDK.SystemAccountsRoles.GetSystemAccountsAccountIDAssignedRoles(ctx, accountID, filter, opts...)
}

func (s *SystemAccountRolesAPIImpl) AssignSystemAccountRole(
	ctx context.Context,
	accountID string,
	assignRole *kkComps.AssignRole,
	opts ...kkOps.Option,
) (*kkOps.PostSystemAccountsAccountIDAssignedRolesResponse, error) {
	return s.SDK.SystemAccountsRoles.PostSystemAccountsAccountIDAssignedRoles(ctx, accountID, assignRole, opts...)
}

func (s *SystemAccountRolesAPIImpl) RemoveSystemAccountRole(
	ctx context.Context,
	accountID string,
	roleID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteSystemAccountsAccountIDAssignedRolesRoleIDResponse, error) {
	return s.SDK.SystemAccountsRoles.DeleteSystemAccountsAccountIDAssignedRolesRoleID(ctx, accountID, roleID, opts...)
}

var _ SystemAccountRolesAPI = (*SystemAccountRolesAPIImpl)(nil)
