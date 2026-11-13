package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"

	"github.com/kong/kongctl/internal/konnect/apiutil"
)

// OrganizationTeamRolesAPI defines the interface for organization team role operations.
type OrganizationTeamRolesAPI interface {
	ListTeamRolesPage(
		ctx context.Context, teamID string, pageSize, pageNumber int64,
	) (*kkComps.AssignedRoleCollection, error)
	ListTeamRoles(
		ctx context.Context,
		teamID string,
		filter *kkOps.ListTeamRolesQueryParamFilter,
		opts ...kkOps.Option,
	) (*kkOps.ListTeamRolesResponse, error)
	ListUserRoles(
		ctx context.Context,
		userID string,
		filter *kkOps.ListUserRolesQueryParamFilter,
		opts ...kkOps.Option,
	) (*kkOps.ListUserRolesResponse, error)
	TeamsAssignRole(
		ctx context.Context,
		teamID string,
		assignRole *kkComps.AssignRole,
		opts ...kkOps.Option,
	) (*kkOps.TeamsAssignRoleResponse, error)
	TeamsRemoveRole(
		ctx context.Context,
		teamID string,
		roleID string,
		opts ...kkOps.Option,
	) (*kkOps.TeamsRemoveRoleResponse, error)
	UsersAssignRole(
		ctx context.Context,
		userID string,
		assignRole *kkComps.AssignRole,
		opts ...kkOps.Option,
	) (*kkOps.UsersAssignRoleResponse, error)
	UsersRemoveRole(
		ctx context.Context,
		userID string,
		roleID string,
		opts ...kkOps.Option,
	) (*kkOps.UsersRemoveRoleResponse, error)
}

// OrganizationTeamRolesAPIImpl provides an SDK-backed implementation of OrganizationTeamRolesAPI.
type OrganizationTeamRolesAPIImpl struct {
	SDK         *kkSDK.SDK
	BaseURL     string
	Token       string
	TokenSource apiutil.TokenSource
	HTTPClient  kkSDK.HTTPClient
}

// ListTeamRolesPage supplies pagination parameters missing from the SDK operation.
func (o *OrganizationTeamRolesAPIImpl) ListTeamRolesPage(
	ctx context.Context, teamID string, pageSize, pageNumber int64,
) (*kkComps.AssignedRoleCollection, error) {
	path := fmt.Sprintf("/v3/teams/%s/assigned-roles?page[size]=%d&page[number]=%d",
		url.PathEscape(teamID), pageSize, pageNumber)
	var result *apiutil.Result
	var err error
	if o.TokenSource != nil {
		result, err = apiutil.RequestWithTokenSource(ctx, o.HTTPClient, http.MethodGet,
			o.BaseURL, path, o.TokenSource, nil, nil)
	} else {
		result, err = apiutil.Request(ctx, o.HTTPClient, http.MethodGet, o.BaseURL, path, o.Token, nil, nil)
	}
	if err != nil {
		return nil, err
	}
	if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("list organization team roles failed with status %d: %s", result.StatusCode, result.Body)
	}
	var collection kkComps.AssignedRoleCollection
	if err := json.Unmarshal(result.Body, &collection); err != nil {
		return nil, fmt.Errorf("decode organization team roles: %w", err)
	}
	if collection.Meta == nil || collection.Meta.Page.Total < 0 ||
		math.IsNaN(collection.Meta.Page.Total) || math.IsInf(collection.Meta.Page.Total, 0) ||
		math.Trunc(collection.Meta.Page.Total) != collection.Meta.Page.Total {
		return nil, fmt.Errorf("missing or invalid organization team roles pagination metadata")
	}
	return &collection, nil
}

func (o *OrganizationTeamRolesAPIImpl) ListTeamRoles(
	ctx context.Context,
	teamID string,
	filter *kkOps.ListTeamRolesQueryParamFilter,
	opts ...kkOps.Option,
) (*kkOps.ListTeamRolesResponse, error) {
	return o.SDK.Roles.ListTeamRoles(ctx, teamID, filter, opts...)
}

func (o *OrganizationTeamRolesAPIImpl) TeamsAssignRole(
	ctx context.Context,
	teamID string,
	assignRole *kkComps.AssignRole,
	opts ...kkOps.Option,
) (*kkOps.TeamsAssignRoleResponse, error) {
	return o.SDK.Roles.TeamsAssignRole(ctx, teamID, assignRole, opts...)
}

func (o *OrganizationTeamRolesAPIImpl) ListUserRoles(
	ctx context.Context,
	userID string,
	filter *kkOps.ListUserRolesQueryParamFilter,
	opts ...kkOps.Option,
) (*kkOps.ListUserRolesResponse, error) {
	return o.SDK.Roles.ListUserRoles(ctx, userID, filter, opts...)
}

func (o *OrganizationTeamRolesAPIImpl) TeamsRemoveRole(
	ctx context.Context,
	teamID string,
	roleID string,
	opts ...kkOps.Option,
) (*kkOps.TeamsRemoveRoleResponse, error) {
	return o.SDK.Roles.TeamsRemoveRole(ctx, teamID, roleID, opts...)
}

func (o *OrganizationTeamRolesAPIImpl) UsersAssignRole(
	ctx context.Context,
	userID string,
	assignRole *kkComps.AssignRole,
	opts ...kkOps.Option,
) (*kkOps.UsersAssignRoleResponse, error) {
	return o.SDK.Roles.UsersAssignRole(ctx, userID, assignRole, opts...)
}

func (o *OrganizationTeamRolesAPIImpl) UsersRemoveRole(
	ctx context.Context,
	userID string,
	roleID string,
	opts ...kkOps.Option,
) (*kkOps.UsersRemoveRoleResponse, error) {
	return o.SDK.Roles.UsersRemoveRole(ctx, userID, roleID, opts...)
}

var _ OrganizationTeamRolesAPI = (*OrganizationTeamRolesAPIImpl)(nil)
