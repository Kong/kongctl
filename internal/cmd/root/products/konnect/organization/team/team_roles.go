package team

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
	teamRolesCommandName = "roles"
	teamIDFlagName       = "team-id"
	teamNameFlagName     = "team-name"
)

type organizationTeamRoleRecord struct {
	ID             string `json:"id"               yaml:"id"`
	TeamID         string `json:"team_id"          yaml:"team_id"`
	RoleName       string `json:"role_name"        yaml:"role_name"`
	EntityID       string `json:"entity_id"        yaml:"entity_id"`
	EntityTypeName string `json:"entity_type_name" yaml:"entity_type_name"`
	EntityRegion   string `json:"entity_region"    yaml:"entity_region"`
}

func newGetOrganizationTeamRolesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     teamRolesCommandName,
		Short:   "List organization team role assignments",
		Long:    "List role assignments for a Konnect organization team.",
		PreRunE: parentPreRun,
		RunE: func(c *cobra.Command, args []string) error {
			handler := organizationTeamRolesHandler{cmd: c}
			return handler.run(args)
		},
	}

	if addParentFlags != nil {
		addParentFlags(verb, c)
	}

	c.Flags().String(teamIDFlagName, "", "Team ID to list roles for")
	c.Flags().String(teamNameFlagName, "", "Team name to list roles for")

	return c
}

type organizationTeamRolesHandler struct {
	cmd *cobra.Command
}

func (h organizationTeamRolesHandler) run(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("organization team roles does not accept positional arguments; use --team-id or --team-name")
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

	teamID, _ := h.cmd.Flags().GetString(teamIDFlagName)
	teamName, _ := h.cmd.Flags().GetString(teamNameFlagName)
	if strings.TrimSpace(teamID) != "" && strings.TrimSpace(teamName) != "" {
		return fmt.Errorf("--team-id and --team-name cannot be used together")
	}
	if strings.TrimSpace(teamID) == "" && strings.TrimSpace(teamName) == "" {
		return fmt.Errorf("one of --team-id or --team-name is required")
	}

	if strings.TrimSpace(teamName) != "" {
		teams, err := runListByName(teamName, sdk.GetOrganizationTeamAPI(), helper, cfg, false)
		if err != nil {
			return err
		}
		if len(teams) != 1 {
			return fmt.Errorf("organization team name %q matched %d teams; use --team-id", teamName, len(teams))
		}
		if teams[0].ID == nil || *teams[0].ID == "" {
			return fmt.Errorf("organization team %q has no ID", teamName)
		}
		teamID = *teams[0].ID
	}

	roles, err := fetchOrganizationTeamRoles(helper, sdk.GetOrganizationTeamRolesAPI(), teamID)
	if err != nil {
		return err
	}

	return renderOrganizationTeamRoles(helper, outType, printer, teamID, roles)
}

func fetchOrganizationTeamRoles(
	helper cmd.Helper,
	roleAPI helpers.OrganizationTeamRolesAPI,
	teamID string,
) ([]kkComps.AssignedRole, error) {
	if roleAPI == nil {
		return nil, fmt.Errorf("organization team roles client is not available")
	}

	res, err := roleAPI.ListTeamRoles(helper.GetContext(), teamID, nil)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to list organization team roles", err, helper.GetCmd(), attrs...)
	}
	if res == nil || res.AssignedRoleCollection == nil {
		return []kkComps.AssignedRole{}, nil
	}

	return res.AssignedRoleCollection.Data, nil
}

func organizationTeamRoleToRecord(role kkComps.AssignedRole, teamID string) organizationTeamRoleRecord {
	record := organizationTeamRoleRecord{
		TeamID: teamID,
	}
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

func renderOrganizationTeamRoles(
	helper cmd.Helper,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	teamID string,
	roles []kkComps.AssignedRole,
) error {
	records := make([]organizationTeamRoleRecord, 0, len(roles))
	for _, role := range roles {
		records = append(records, organizationTeamRoleToRecord(role, teamID))
	}

	return tableview.RenderForFormat(helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		roles,
		"",
		tableview.WithRootLabel("roles"),
		tableview.WithDetailHelper(helper),
	)
}

func buildOrganizationTeamRolesChildView(teamID string, roles []kkComps.AssignedRole) tableview.ChildView {
	records := make([]organizationTeamRoleRecord, 0, len(roles))
	rows := make([]table.Row, 0, len(roles))
	for _, role := range roles {
		record := organizationTeamRoleToRecord(role, teamID)
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
			fmt.Fprintf(&b, "team_id: %s\n", r.TeamID)
			fmt.Fprintf(&b, "role_name: %s\n", r.RoleName)
			fmt.Fprintf(&b, "entity_id: %s\n", r.EntityID)
			fmt.Fprintf(&b, "entity_type_name: %s\n", r.EntityTypeName)
			fmt.Fprintf(&b, "entity_region: %s\n", r.EntityRegion)
			return strings.TrimRight(b.String(), "\n")
		},
		Title:      "Roles",
		ParentType: common.ViewParentTeam,
	}
}
