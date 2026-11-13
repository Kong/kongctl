package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"

	"github.com/kong/kongctl/internal/konnect/apiutil"
	"github.com/kong/kongctl/internal/util/pagination"
)

// OrganizationTeamRolesAPI defines the interface for organization team role operations.
type OrganizationTeamRolesAPI interface {
	ListUserRolesPage(
		ctx context.Context,
		userID string,
		pageSize, pageNumber int64,
	) (*kkComps.AssignedRoleCollection, error)

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

func organizationTeamRolesBaseURL(baseURL string) string {
	if baseURL == "" {
		return globalAPIBaseURL
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Port() != "" || (u.Path != "" && u.Path != "/") {
		return baseURL
	}
	for _, domain := range []string{"konghq.com", "konghq.tech"} {
		if strings.HasSuffix(strings.ToLower(u.Hostname()), ".api."+domain) {
			u.Host = "global.api." + domain
			return u.String()
		}
	}
	return baseURL
}

func organizationAssignedRoleReader(
	fetch func(context.Context, int64, int64) (*kkComps.AssignedRoleCollection, error),
) pagination.NumberedReader[kkComps.AssignedRole] {
	return pagination.NumberedReader[kkComps.AssignedRole]{
		Fetch: func(ctx context.Context, size, page int64) ([]kkComps.AssignedRole, *pagination.PageMeta, error) {
			collection, err := fetch(ctx, size, page)
			if err != nil {
				return nil, nil, err
			}
			if collection == nil || collection.Meta == nil {
				return nil, nil, fmt.Errorf("missing assigned roles pagination metadata")
			}
			return collection.Data, &pagination.PageMeta{
				Total:    collection.Meta.Page.Total,
				RawCount: len(collection.Data),
			}, nil
		},
		Key: func(role kkComps.AssignedRole) string {
			if role.ID == nil {
				return ""
			}
			return *role.ID
		},
	}
}

func ListAllOrganizationTeamRoles(
	ctx context.Context,
	api OrganizationTeamRolesAPI,
	teamID string,
	pageSize int64,
) ([]kkComps.AssignedRole, error) {
	return organizationAssignedRoleReader(
		func(ctx context.Context, size, page int64) (*kkComps.AssignedRoleCollection, error) {
			return api.ListTeamRolesPage(ctx, teamID, size, page)
		},
	).All(ctx, pageSize)
}

func FindOrganizationTeamRole(
	ctx context.Context,
	api OrganizationTeamRolesAPI,
	teamID, roleID string,
) (*kkComps.AssignedRole, error) {
	return organizationAssignedRoleReader(
		func(ctx context.Context, size, page int64) (*kkComps.AssignedRoleCollection, error) {
			return api.ListTeamRolesPage(ctx, teamID, size, page)
		},
	).Find(ctx, 100, func(role kkComps.AssignedRole) bool {
		return role.ID != nil && *role.ID == roleID
	})
}

// ListTeamRolesPage supplies pagination parameters missing from the SDK operation.
func (o *OrganizationTeamRolesAPIImpl) ListTeamRolesPage(
	ctx context.Context, teamID string, pageSize, pageNumber int64,
) (*kkComps.AssignedRoleCollection, error) {
	return o.listAssignedRolesPage(
		ctx,
		"/v3/teams/"+url.PathEscape(teamID)+"/assigned-roles",
		pageSize,
		pageNumber,
	)
}

func (o *OrganizationTeamRolesAPIImpl) ListUserRolesPage(
	ctx context.Context,
	userID string,
	pageSize, pageNumber int64,
) (*kkComps.AssignedRoleCollection, error) {
	return o.listAssignedRolesPage(
		ctx,
		"/v3/users/"+url.PathEscape(userID)+"/assigned-roles",
		pageSize,
		pageNumber,
	)
}

func ListAllOrganizationUserRoles(
	ctx context.Context,
	api OrganizationTeamRolesAPI,
	userID string,
	pageSize int64,
) ([]kkComps.AssignedRole, error) {
	return organizationAssignedRoleReader(
		func(ctx context.Context, size, page int64) (*kkComps.AssignedRoleCollection, error) {
			return api.ListUserRolesPage(ctx, userID, size, page)
		},
	).All(ctx, pageSize)
}

func (o *OrganizationTeamRolesAPIImpl) listAssignedRolesPage(
	ctx context.Context,
	path string,
	pageSize, pageNumber int64,
) (*kkComps.AssignedRoleCollection, error) {
	path += fmt.Sprintf("?page[size]=%d&page[number]=%d", pageSize, pageNumber)
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
		message := strings.TrimSpace(string(result.Body))
		var problem struct {
			Detail  string `json:"detail"`
			Message string `json:"message"`
		}
		if json.Unmarshal(result.Body, &problem) == nil {
			if problem.Detail != "" {
				message = problem.Detail
			} else if problem.Message != "" {
				message = problem.Message
			}
		}
		if text := []rune(message); len(text) > 512 {
			message = string(text[:512]) + "..."
		}
		if message == "" {
			return nil, fmt.Errorf(
				"list organization assigned roles failed with status %d",
				result.StatusCode,
			)
		}
		return nil, fmt.Errorf(
			"list organization assigned roles failed with status %d: %s",
			result.StatusCode,
			message,
		)
	}
	var collection kkComps.AssignedRoleCollection
	if err := json.Unmarshal(result.Body, &collection); err != nil {
		return nil, fmt.Errorf("decode organization assigned roles: %w", err)
	}
	if collection.Meta == nil || collection.Meta.Page.Total < 0 ||
		math.IsNaN(collection.Meta.Page.Total) || math.IsInf(collection.Meta.Page.Total, 0) ||
		math.Trunc(collection.Meta.Page.Total) != collection.Meta.Page.Total {
		return nil, fmt.Errorf("missing or invalid organization assigned roles pagination metadata")
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
