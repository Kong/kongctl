package user

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/table"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	userRolesCommandName = "roles"
	userIDFlagName       = "user-id"
	userEmailFlagName    = "user-email"
)

type organizationUserRoleRecord struct {
	ID             string `json:"id"               yaml:"id"`
	UserID         string `json:"user_id"          yaml:"user_id"`
	RoleName       string `json:"role_name"        yaml:"role_name"`
	EntityID       string `json:"entity_id"        yaml:"entity_id"`
	EntityTypeName string `json:"entity_type_name" yaml:"entity_type_name"`
	EntityRegion   string `json:"entity_region"    yaml:"entity_region"`
}

func newGetOrganizationUserRolesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     userRolesCommandName,
		Short:   "List organization user role assignments",
		Long:    "List direct role assignments for a Konnect organization user.",
		PreRunE: parentPreRun,
		RunE: func(c *cobra.Command, args []string) error {
			handler := organizationUserRolesHandler{cmd: c}
			return handler.run(args)
		},
	}

	if addParentFlags != nil {
		addParentFlags(verb, c)
	}

	c.Flags().String(userIDFlagName, "", "User ID to list roles for")
	c.Flags().String(userEmailFlagName, "", "User email to list roles for")

	return c
}

type organizationUserRolesHandler struct {
	cmd *cobra.Command
}

func (h organizationUserRolesHandler) run(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("organization user roles does not accept positional arguments; use --user-id or --user-email")
	}

	userID, _ := h.cmd.Flags().GetString(userIDFlagName)
	userEmail, _ := h.cmd.Flags().GetString(userEmailFlagName)
	if strings.TrimSpace(userID) != "" && strings.TrimSpace(userEmail) != "" {
		return fmt.Errorf("--user-id and --user-email cannot be used together")
	}
	if strings.TrimSpace(userID) == "" && strings.TrimSpace(userEmail) == "" {
		return fmt.Errorf("one of --user-id or --user-email is required")
	}

	helper := cmd.BuildHelper(h.cmd, args)
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	if strings.TrimSpace(userEmail) != "" {
		orgUser, err := resolveOrganizationUserByEmail(userEmail, sdk.GetOrganizationUsersAPI(), helper, cfg)
		if err != nil {
			return err
		}
		if orgUser.ID == nil || *orgUser.ID == "" {
			return fmt.Errorf("organization user %q has no ID", userEmail)
		}
		userID = *orgUser.ID
	}

	roles, err := fetchOrganizationUserRoles(helper, sdk.GetOrganizationTeamRolesAPI(), userID)
	if err != nil {
		return err
	}

	return renderOrganizationUserRoles(helper, outType, printer, userID, roles)
}

func fetchOrganizationUserRoles(
	helper cmd.Helper,
	roleAPI helpers.OrganizationTeamRolesAPI,
	userID string,
) ([]kkComps.AssignedRole, error) {
	if roleAPI == nil {
		return nil, fmt.Errorf("organization user roles client is not available")
	}

	res, err := roleAPI.ListUserRoles(helper.GetContext(), userID, nil)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to list organization user roles", err, helper.GetCmd(), attrs...)
	}
	if res == nil || res.AssignedRoleCollection == nil {
		return []kkComps.AssignedRole{}, nil
	}

	return res.AssignedRoleCollection.Data, nil
}

func organizationUserRoleToRecord(role kkComps.AssignedRole, userID string) organizationUserRoleRecord {
	record := organizationUserRoleRecord{UserID: userID}
	if role.ID != nil {
		record.ID = *role.ID
	}
	if role.RoleName != nil {
		record.RoleName = *role.RoleName
	}
	if role.EntityID != nil {
		record.EntityID = *role.EntityID
	}
	if role.EntityTypeName != nil {
		record.EntityTypeName = *role.EntityTypeName
	}
	if role.EntityRegion != nil {
		record.EntityRegion = string(*role.EntityRegion)
	}
	return record
}

func renderOrganizationUserRoles(
	helper cmd.Helper,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	userID string,
	roles []kkComps.AssignedRole,
) error {
	records := make([]organizationUserRoleRecord, 0, len(roles))
	for _, role := range roles {
		records = append(records, organizationUserRoleToRecord(role, userID))
	}

	return tableview.RenderForFormat(helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		records,
		"",
		tableview.WithRootLabel("roles"),
		tableview.WithDetailHelper(helper),
	)
}

func buildOrganizationUserRolesChildView(userID string, roles []kkComps.AssignedRole) tableview.ChildView {
	records := make([]organizationUserRoleRecord, 0, len(roles))
	rows := make([]table.Row, 0, len(roles))
	for _, role := range roles {
		record := organizationUserRoleToRecord(role, userID)
		records = append(records, record)
		rows = append(rows, table.Row{record.RoleName, record.EntityTypeName, record.EntityRegion})
	}

	return tableview.ChildView{
		Headers: []string{"ROLE", "ENTITY TYPE", "REGION"},
		Rows:    rows,
		DetailRenderer: func(index int) string {
			if index < 0 || index >= len(records) {
				return ""
			}
			r := records[index]
			var b strings.Builder
			fmt.Fprintf(&b, "id: %s\n", r.ID)
			fmt.Fprintf(&b, "user_id: %s\n", r.UserID)
			fmt.Fprintf(&b, "role_name: %s\n", r.RoleName)
			fmt.Fprintf(&b, "entity_id: %s\n", r.EntityID)
			fmt.Fprintf(&b, "entity_type_name: %s\n", r.EntityTypeName)
			fmt.Fprintf(&b, "entity_region: %s\n", r.EntityRegion)
			return strings.TrimRight(b.String(), "\n")
		},
		Title:      "Roles",
		ParentType: common.ViewParentOrganizationUser,
	}
}
