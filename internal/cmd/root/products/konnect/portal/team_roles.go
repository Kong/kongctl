package portal

import (
	"fmt"
	"strings"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/charmbracelet/bubbles/table"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	teamRolesCommandName = "team-roles"

	teamIDFlagName   = "team-id"
	teamNameFlagName = "team-name"
)

type portalTeamRoleRecord struct {
	Team           string `json:"team"`
	TeamID         string `json:"team_id"`
	RoleName       string `json:"role_name"`
	EntityTypeName string `json:"entity_type_name"`
	EntityID       string `json:"entity_id"`
}

var (
	teamRolesUse = teamRolesCommandName

	teamRolesShort = i18n.T("root.products.konnect.portal.teamRolesShort",
		"List portal team role assignments for a Konnect portal")
	teamRolesLong = normalizers.LongDesc(i18n.T("root.products.konnect.portal.teamRolesLong",
		`Use the team-roles command to list role assignments for teams in a specific Konnect portal.`))
	teamRolesExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.teamRolesExamples",
			fmt.Sprintf(`
# List roles for all teams in a portal by ID
%[1]s get portal team roles --portal-id <portal-id>
# List roles for all teams in a portal by name
%[1]s get portal team roles --portal-name my-portal
# List roles for a specific team by name
%[1]s get portal team roles --portal-name my-portal --team-name backend-team
`, meta.CLIName)))
)

func newGetPortalTeamRolesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     teamRolesUse,
		Short:   teamRolesShort,
		Long:    teamRolesLong,
		Example: teamRolesExample,
		Aliases: []string{"team-role", "teamroles", "teamrole", "roles", "role"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			return bindPortalChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := portalTeamRolesHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addPortalChildFlags(cmd)
	bindTeamRoleFilterFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

func bindTeamRoleFilterFlags(cmd *cobra.Command) {
	cmd.Flags().String(teamIDFlagName, "", "Team ID to filter role assignments")
	cmd.Flags().String(teamNameFlagName, "", "Team name to filter role assignments")
}

type portalTeamRolesHandler struct {
	cmd *cobra.Command
}

func (h portalTeamRolesHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("portal team roles does not accept positional arguments; use flags to scope results"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	interactive, err := helper.IsInteractive()
	if err != nil {
		return err
	}

	var printer cli.PrintFlusher
	if !interactive {
		printer, err = cli.Format(outType.String(), helper.GetStreams().Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	portalID, portalName := getPortalIdentifiers(cfg)
	if portalID != "" && portalName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", portalIDFlagName, portalNameFlagName),
		}
	}

	if portalID == "" && portalName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"a portal identifier is required. Provide --%s or --%s",
				portalIDFlagName,
				portalNameFlagName,
			),
		}
	}

	if portalID == "" {
		portalID, err = resolvePortalIDByName(portalName, sdk.GetPortalAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}

	teamAPI := sdk.GetPortalTeamAPI()
	if teamAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Portal teams client is not available",
			Err: fmt.Errorf("portal teams client not configured"),
		}
	}

	roleAPI := sdk.GetPortalTeamRolesAPI()
	if roleAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Portal team roles client is not available",
			Err: fmt.Errorf("portal team roles client not configured"),
		}
	}

	teamIDFlag := strings.TrimSpace(h.cmd.Flag(teamIDFlagName).Value.String())
	teamNameFlag := strings.TrimSpace(h.cmd.Flag(teamNameFlagName).Value.String())

	if teamIDFlag != "" && teamNameFlag != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", teamIDFlagName, teamNameFlagName),
		}
	}

	var teams []kkComps.PortalTeamResponse
	if teamIDFlag == "" || teamNameFlag != "" {
		teams, err = fetchPortalTeams(helper, teamAPI, portalID, cfg)
		if err != nil {
			return err
		}
	}

	var teamID string
	var teamName string

	switch {
	case teamNameFlag != "":
		match := findTeamByName(teams, teamNameFlag)
		if match == nil || match.GetID() == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("team %q not found", teamNameFlag),
			}
		}
		teamID = *match.GetID()
		teamName = optionalPtr(match.GetName())
	case teamIDFlag != "":
		teamID = teamIDFlag
	default:
		// portal-only listing; iterate all teams
	}

	records := []portalTeamRoleRecord{}

	if teamID != "" {
		roles, err := fetchPortalTeamRoles(helper, roleAPI, portalID, teamID, cfg)
		if err != nil {
			return err
		}

		if teamName == "" {
			teamName = teamID
			if len(teams) == 0 {
				// try to enrich name from API if not already fetched
				teamName = teamID
			} else {
				for _, t := range teams {
					if t.GetID() != nil && *t.GetID() == teamID {
						teamName = optionalPtr(t.GetName())
						break
					}
				}
			}
		}

		records = append(records, roleResponsesToRecords(teamName, teamID, roles)...)
	} else {
		for _, team := range teams {
			teamIDValue := ""
			if team.GetID() != nil {
				teamIDValue = *team.GetID()
			}
			if teamIDValue == "" {
				continue
			}
			teamDisplayName := optionalPtr(team.GetName())
			roles, err := fetchPortalTeamRoles(helper, roleAPI, portalID, teamIDValue, cfg)
			if err != nil {
				return err
			}
			records = append(records, roleResponsesToRecords(teamDisplayName, teamIDValue, roles)...)
		}
	}

	return renderTeamRoles(helper, interactive, outType, printer, records)
}

func renderTeamRoles(
	helper cmd.Helper,
	interactive bool,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	records []portalTeamRoleRecord,
) error {
	tableRows := make([]table.Row, 0, len(records))
	for _, rec := range records {
		tableRows = append(tableRows, table.Row{
			rec.Team,
			rec.RoleName,
			rec.EntityTypeName,
			rec.EntityID,
		})
	}

	return tableview.RenderForFormat(
		interactive,
		outType,
		printer,
		helper.GetStreams(),
		records,
		records,
		"",
		tableview.WithCustomTable(
			[]string{"TEAM", "ROLE", "ENTITY TYPE", "ENTITY ID"},
			tableRows,
		),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func roleResponsesToRecords(
	teamName string,
	teamID string,
	roles []kkComps.PortalAssignedRoleResponse,
) []portalTeamRoleRecord {
	records := make([]portalTeamRoleRecord, 0, len(roles))
	for _, role := range roles {
		entityID := optionalPtr(role.GetEntityID())
		if util.IsValidUUID(entityID) {
			entityID = util.AbbreviateUUID(entityID)
		}
		records = append(records, portalTeamRoleRecord{
			Team:           teamName,
			TeamID:         teamID,
			RoleName:       optionalPtr(role.GetRoleName()),
			EntityTypeName: optionalPtr(role.GetEntityTypeName()),
			EntityID:       entityID,
		})
	}
	return records
}

func fetchPortalTeamRoles(
	helper cmd.Helper,
	roleAPI helpers.PortalTeamRolesAPI,
	portalID string,
	teamID string,
	cfg config.Hook,
) ([]kkComps.PortalAssignedRoleResponse, error) {
	var pageNumber int64 = 1
	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if pageSize < 1 {
		pageSize = int64(common.DefaultRequestPageSize)
	}

	var all []kkComps.PortalAssignedRoleResponse

	for {
		req := kkOps.ListPortalTeamRolesRequest{
			PortalID:   portalID,
			TeamID:     teamID,
			PageSize:   kk.Int64(pageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := roleAPI.ListPortalTeamRoles(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			attrs = append(attrs, "portal_id", portalID, "team_id", teamID)
			return nil, cmd.PrepareExecutionError("Failed to list portal team roles", err, helper.GetCmd(), attrs...)
		}

		if res.GetAssignedPortalRoleCollectionResponse() == nil {
			break
		}

		data := res.GetAssignedPortalRoleCollectionResponse().GetData()
		all = append(all, data...)

		total := int(res.GetAssignedPortalRoleCollectionResponse().GetMeta().Page.Total)
		if total == 0 || len(all) >= total || len(data) == 0 {
			break
		}

		pageNumber++
	}

	return all, nil
}
