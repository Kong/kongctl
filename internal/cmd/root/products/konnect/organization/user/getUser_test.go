package user

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/build"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubOrganizationUsersAPI struct {
	listUsers func(context.Context, kkOps.ListUsersRequest, ...kkOps.Option) (*kkOps.ListUsersResponse, error)
	getUser   func(context.Context, string, ...kkOps.Option) (*kkOps.GetUserResponse, error)
}

func (s *stubOrganizationUsersAPI) ListUsers(
	ctx context.Context,
	request kkOps.ListUsersRequest,
	opts ...kkOps.Option,
) (*kkOps.ListUsersResponse, error) {
	if s.listUsers != nil {
		return s.listUsers(ctx, request, opts...)
	}
	return nil, nil
}

func (s *stubOrganizationUsersAPI) GetUser(
	ctx context.Context,
	userID string,
	opts ...kkOps.Option,
) (*kkOps.GetUserResponse, error) {
	if s.getUser != nil {
		return s.getUser(ctx, userID, opts...)
	}
	return nil, nil
}

type stubOrganizationTeamRolesAPI struct {
	listUserRoles func(context.Context, string, *kkOps.ListUserRolesQueryParamFilter, ...kkOps.Option) (
		*kkOps.ListUserRolesResponse,
		error,
	)
}

func (s *stubOrganizationTeamRolesAPI) ListTeamRoles(
	context.Context,
	string,
	*kkOps.ListTeamRolesQueryParamFilter,
	...kkOps.Option,
) (*kkOps.ListTeamRolesResponse, error) {
	panic("ListTeamRoles should not be called")
}

func (s *stubOrganizationTeamRolesAPI) ListUserRoles(
	ctx context.Context,
	userID string,
	filter *kkOps.ListUserRolesQueryParamFilter,
	opts ...kkOps.Option,
) (*kkOps.ListUserRolesResponse, error) {
	if s.listUserRoles != nil {
		return s.listUserRoles(ctx, userID, filter, opts...)
	}
	return nil, nil
}

func (s *stubOrganizationTeamRolesAPI) TeamsAssignRole(
	context.Context,
	string,
	*kkComps.AssignRole,
	...kkOps.Option,
) (*kkOps.TeamsAssignRoleResponse, error) {
	panic("TeamsAssignRole should not be called")
}

func (s *stubOrganizationTeamRolesAPI) TeamsRemoveRole(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.TeamsRemoveRoleResponse, error) {
	panic("TeamsRemoveRole should not be called")
}

func (s *stubOrganizationTeamRolesAPI) UsersAssignRole(
	context.Context,
	string,
	*kkComps.AssignRole,
	...kkOps.Option,
) (*kkOps.UsersAssignRoleResponse, error) {
	panic("UsersAssignRole should not be called")
}

func (s *stubOrganizationTeamRolesAPI) UsersRemoveRole(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.UsersRemoveRoleResponse, error) {
	panic("UsersRemoveRole should not be called")
}

type userTestHelper struct {
	cmd     *cobra.Command
	streams *iostreams.IOStreams
	cfg     config.Hook
	ctx     context.Context
}

func newUserTestHelper() *userTestHelper {
	cfg := config.BuildProfiledConfig("default", "", viper.New())
	cfg.Set(common.RequestPageSizeConfigPath, 2)
	cfg.Set(cmdCommon.OutputConfigPath, "json")
	streams := iostreams.NewTestIOStreamsOnly()
	return &userTestHelper{
		cmd:     &cobra.Command{Use: "users"},
		streams: streams,
		cfg:     cfg,
		ctx:     context.Background(),
	}
}

func (h *userTestHelper) GetCmd() *cobra.Command { return h.cmd }
func (h *userTestHelper) GetArgs() []string      { return nil }
func (h *userTestHelper) GetVerb() (verbs.VerbValue, error) {
	return verbs.Get, nil
}

func (h *userTestHelper) GetProduct() (products.ProductValue, error) {
	return products.ProductValue("konnect"), nil
}
func (h *userTestHelper) GetStreams() *iostreams.IOStreams { return h.streams }
func (h *userTestHelper) GetConfig() (config.Hook, error)  { return h.cfg, nil }
func (h *userTestHelper) GetOutputFormat() (cmdCommon.OutputFormat, error) {
	return cmdCommon.JSON, nil
}
func (h *userTestHelper) GetLogger() (*slog.Logger, error) { return slog.Default(), nil }
func (h *userTestHelper) GetBuildInfo() (*build.Info, error) {
	return &build.Info{}, nil
}
func (h *userTestHelper) GetContext() context.Context { return h.ctx }
func (h *userTestHelper) GetKonnectSDK(config.Hook, *slog.Logger) (helpers.SDKAPI, error) {
	return nil, nil
}

func TestRunListUsersPagesAllUsers(t *testing.T) {
	helper := newUserTestHelper()
	var seenPages []int64
	users := []kkComps.User{
		{ID: new("user-1"), Email: new("one@example.com")},
		{ID: new("user-2"), Email: new("two@example.com")},
		{ID: new("user-3"), Email: new("three@example.com")},
	}

	api := &stubOrganizationUsersAPI{
		listUsers: func(
			_ context.Context,
			request kkOps.ListUsersRequest,
			_ ...kkOps.Option,
		) (*kkOps.ListUsersResponse, error) {
			seenPages = append(seenPages, *request.PageNumber)
			start := int((*request.PageNumber - 1) * *request.PageSize)
			end := min(start+int(*request.PageSize), len(users))
			return usersResponse(users[start:end], len(users)), nil
		},
	}

	got, err := runListUsers(api, helper, helper.cfg)
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2}, seenPages)
	assert.Equal(t, users, got)
}

func TestResolveOrganizationUserByEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		users   []kkComps.User
		wantID  string
		wantErr string
	}{
		{
			name:   "case insensitive exact match",
			email:  "ONE@example.com",
			users:  []kkComps.User{{ID: new("user-1"), Email: new("one@example.com")}},
			wantID: "user-1",
		},
		{
			name:    "missing",
			email:   "missing@example.com",
			users:   []kkComps.User{{ID: new("user-1"), Email: new("one@example.com")}},
			wantErr: `organization user with email "missing@example.com" not found`,
		},
		{
			name:  "duplicate",
			email: "dupe@example.com",
			users: []kkComps.User{
				{ID: new("user-1"), Email: new("dupe@example.com")},
				{ID: new("user-2"), Email: new("DUPE@example.com")},
			},
			wantErr: `organization user email "dupe@example.com" matched 2 users; use user ID`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helper := newUserTestHelper()
			api := &stubOrganizationUsersAPI{
				listUsers: func(
					context.Context,
					kkOps.ListUsersRequest,
					...kkOps.Option,
				) (*kkOps.ListUsersResponse, error) {
					return usersResponse(tt.users, len(tt.users)), nil
				},
			}

			got, err := resolveOrganizationUserByEmail(tt.email, api, helper, helper.cfg)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantID, *got.ID)
		})
	}
}

func TestRenderUsersListAppliesJQToRecords(t *testing.T) {
	helper := newUserTestHelper()
	helper.cmd.Flags().String("jq", "", "")
	require.NoError(t, helper.cmd.Flags().Set("jq", `.[] | select(.email == "one@example.com") | .id`))

	printer := testPrinter{out: helper.streams.Out.(*bytes.Buffer)}
	err := renderUsersList(helper, "users", cmdCommon.JSON, printer, []kkComps.User{
		{ID: new("user-1"), Email: new("one@example.com")},
		{ID: new("user-2"), Email: new("two@example.com")},
	})
	require.NoError(t, err)
	assert.Contains(t, helper.streams.Out.(*bytes.Buffer).String(), "user-1")
	assert.NotContains(t, helper.streams.Out.(*bytes.Buffer).String(), "user-2")
}

func TestBuildUserChildView(t *testing.T) {
	view := buildUserChildView([]kkComps.User{{
		ID:       new("4d9b3f3e-7b1b-4b6b-8b1b-4b6b7b1b4b6b"),
		Email:    new("one@example.com"),
		FullName: new("One User"),
	}})

	require.Len(t, view.Rows, 1)
	assert.Equal(t, common.ViewParentOrganizationUser, view.ParentType)
	assert.Contains(t, view.DetailRenderer(0), "email: one@example.com")
	require.NotNil(t, view.DetailContext)
	assert.IsType(t, &kkComps.User{}, view.DetailContext(0))
}

type testPrinter struct {
	out *bytes.Buffer
}

func (p testPrinter) Print(v any) {
	fmt.Fprint(p.out, v)
}

func (p testPrinter) Flush() {}

func usersResponse(users []kkComps.User, total int) *kkOps.ListUsersResponse {
	return &kkOps.ListUsersResponse{
		UserCollection: &kkComps.UserCollection{
			Data: users,
			Meta: &kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: float64(total)},
			},
		},
	}
}
