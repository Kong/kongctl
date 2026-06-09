package user

import (
	"bytes"
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	kkCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchOrganizationUserRoles(t *testing.T) {
	roleRegion := kkComps.AssignedRoleEntityRegionUs
	api := &stubOrganizationTeamRolesAPI{
		listUserRoles: func(
			_ context.Context,
			userID string,
			filter *kkOps.ListUserRolesQueryParamFilter,
			_ ...kkOps.Option,
		) (*kkOps.ListUserRolesResponse, error) {
			assert.Equal(t, "user-1", userID)
			assert.Nil(t, filter)
			return &kkOps.ListUserRolesResponse{
				AssignedRoleCollection: &kkComps.AssignedRoleCollection{
					Data: []kkComps.AssignedRole{{
						ID:             stringPtr("role-1"),
						RoleName:       stringPtr("Viewer"),
						EntityID:       stringPtr("entity-1"),
						EntityTypeName: stringPtr("API"),
						EntityRegion:   &roleRegion,
					}},
				},
			}, nil
		},
	}

	roles, err := fetchOrganizationUserRoles(newUserTestHelper(), api, "user-1")
	require.NoError(t, err)
	require.Len(t, roles, 1)
	assert.Equal(t, "role-1", *roles[0].ID)
}

func TestOrganizationUserRolesHandlerValidatesSelectorFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		userID  string
		email   string
		wantErr string
	}{
		{
			name:    "rejects positional args",
			args:    []string{"extra"},
			wantErr: "organization user roles does not accept positional arguments; use --user-id or --user-email",
		},
		{
			name:    "requires selector",
			wantErr: "one of --user-id or --user-email is required",
		},
		{
			name:    "rejects both selectors",
			userID:  "user-1",
			email:   "one@example.com",
			wantErr: "--user-id and --user-email cannot be used together",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &cobra.Command{Use: "roles"}
			c.Flags().String(userIDFlagName, "", "")
			c.Flags().String(userEmailFlagName, "", "")
			if tt.userID != "" {
				require.NoError(t, c.Flags().Set(userIDFlagName, tt.userID))
			}
			if tt.email != "" {
				require.NoError(t, c.Flags().Set(userEmailFlagName, tt.email))
			}

			err := organizationUserRolesHandler{cmd: c}.run(tt.args)
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestRenderOrganizationUserRolesAppliesJQToRecords(t *testing.T) {
	helper := newUserTestHelper()
	helper.cmd.Flags().String("jq", "", "")
	require.NoError(t, helper.cmd.Flags().Set("jq", `.[] | select(.user_id == "user-1") | .role_name`))

	printer := testPrinter{out: helper.streams.Out.(*bytes.Buffer)}
	err := renderOrganizationUserRoles(helper, cmdCommon.JSON, printer, "user-1", []kkComps.AssignedRole{
		{ID: stringPtr("role-1"), RoleName: stringPtr("Viewer")},
	})
	require.NoError(t, err)
	assert.Contains(t, helper.streams.Out.(*bytes.Buffer).String(), "Viewer")
}

func TestBuildOrganizationUserRolesChildView(t *testing.T) {
	view := buildOrganizationUserRolesChildView("user-1", []kkComps.AssignedRole{{
		ID:             stringPtr("role-1"),
		RoleName:       stringPtr("Viewer"),
		EntityTypeName: stringPtr("API"),
	}})

	require.Len(t, view.Rows, 1)
	assert.Equal(t, kkCommon.ViewParentOrganizationUser, view.ParentType)
	assert.Contains(t, view.DetailRenderer(0), "user_id: user-1")
}
