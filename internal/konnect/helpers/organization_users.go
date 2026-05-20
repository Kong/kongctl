package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// OrganizationUsersAPI defines the interface for organization user lookup operations.
type OrganizationUsersAPI interface {
	ListUsers(
		ctx context.Context,
		request kkOps.ListUsersRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListUsersResponse, error)
	GetUser(ctx context.Context, userID string, opts ...kkOps.Option) (*kkOps.GetUserResponse, error)
}

// OrganizationUsersAPIImpl provides an SDK-backed implementation of OrganizationUsersAPI.
type OrganizationUsersAPIImpl struct {
	SDK *kkSDK.SDK
}

func (o *OrganizationUsersAPIImpl) ListUsers(
	ctx context.Context,
	request kkOps.ListUsersRequest,
	opts ...kkOps.Option,
) (*kkOps.ListUsersResponse, error) {
	return o.SDK.Users.ListUsers(ctx, request, opts...)
}

func (o *OrganizationUsersAPIImpl) GetUser(
	ctx context.Context,
	userID string,
	opts ...kkOps.Option,
) (*kkOps.GetUserResponse, error) {
	return o.SDK.Users.GetUser(ctx, userID, opts...)
}

var _ OrganizationUsersAPI = (*OrganizationUsersAPIImpl)(nil)
