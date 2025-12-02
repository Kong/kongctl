package portal

import (
	"fmt"
	"strings"
	"time"

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
	teamsCommandName = "teams"
)

type portalTeamSummaryRecord struct {
	ID               string
	Name             string
	Description      string
	LocalCreatedTime string
	LocalUpdatedTime string
}

var (
	teamsUse = teamsCommandName

	teamsShort = i18n.T("root.products.konnect.portal.teamsShort",
		"Manage portal teams for a Konnect portal")
	teamsLong = normalizers.LongDesc(i18n.T("root.products.konnect.portal.teamsLong",
		`Use the teams command to list or retrieve developer teams for a specific Konnect portal.`))
	teamsExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.teamsExamples",
			fmt.Sprintf(`
# List teams for a portal by ID
%[1]s get portal teams --portal-id <portal-id>
# List teams for a portal by name
%[1]s get portal teams --portal-name my-portal
# Get a specific team by ID
%[1]s get portal teams --portal-id <portal-id> <team-id>
# Get a specific team by name
%[1]s get portal teams --portal-id <portal-id> developers
`, meta.CLIName)))
)

func newGetPortalTeamsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     teamsUse,
		Short:   teamsShort,
		Long:    teamsLong,
		Example: teamsExample,
		Aliases: []string{"team"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			return bindPortalChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := portalTeamsHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addPortalChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	if teamRolesCmd := newGetPortalTeamRolesCmd(verb, addParentFlags, parentPreRun); teamRolesCmd != nil {
		cmd.AddCommand(teamRolesCmd)
	}

	return cmd
}

type portalTeamsHandler struct {
	cmd *cobra.Command
}

func (h portalTeamsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing portal teams requires 0 or 1 arguments (ID or name)"),
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

	if len(args) == 1 {
		return h.getSingleTeam(
			helper,
			teamAPI,
			portalID,
			strings.TrimSpace(args[0]),
			interactive,
			outType,
			printer,
			cfg,
		)
	}

	return h.listTeams(helper, teamAPI, portalID, interactive, outType, printer, cfg)
}

func (h portalTeamsHandler) listTeams(
	helper cmd.Helper,
	teamAPI helpers.PortalTeamAPI,
	portalID string,
	interactive bool,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	teams, err := fetchPortalTeams(helper, teamAPI, portalID, cfg)
	if err != nil {
		return err
	}

	records := make([]portalTeamSummaryRecord, 0, len(teams))
	for _, team := range teams {
		records = append(records, portalTeamSummaryToRecord(team))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Name})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(teams) {
			return ""
		}
		return portalTeamDetailView(teams[index])
	}

	return tableview.RenderForFormat(
		interactive,
		outType,
		printer,
		helper.GetStreams(),
		records,
		teams,
		"",
		tableview.WithCustomTable([]string{"ID", "NAME"}, tableRows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (h portalTeamsHandler) getSingleTeam(
	helper cmd.Helper,
	teamAPI helpers.PortalTeamAPI,
	portalID string,
	identifier string,
	interactive bool,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	teamID := identifier
	if !util.IsValidUUID(identifier) {
		teams, err := fetchPortalTeams(helper, teamAPI, portalID, cfg)
		if err != nil {
			return err
		}
		match := findTeamByName(teams, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("team %q not found", identifier),
			}
		}
		if match.GetID() != nil {
			teamID = *match.GetID()
		} else {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("team %q does not have an ID", identifier),
			}
		}
	}

	res, err := teamAPI.GetPortalTeam(helper.GetContext(), teamID, portalID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get portal team", err, helper.GetCmd(), attrs...)
	}

	team := res.GetPortalTeamResponse()
	if team == nil {
		return &cmd.ExecutionError{
			Msg: "Portal team response was empty",
			Err: fmt.Errorf("no team returned for id %s", teamID),
		}
	}

	return tableview.RenderForFormat(
		interactive,
		outType,
		printer,
		helper.GetStreams(),
		portalTeamSummaryToRecord(*team),
		team,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func fetchPortalTeams(
	helper cmd.Helper,
	teamAPI helpers.PortalTeamAPI,
	portalID string,
	cfg config.Hook,
) ([]kkComps.PortalTeamResponse, error) {
	var pageNumber int64 = 1
	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if pageSize < 1 {
		pageSize = int64(common.DefaultRequestPageSize)
	}

	var all []kkComps.PortalTeamResponse

	for {
		req := kkOps.ListPortalTeamsRequest{
			PortalID:   portalID,
			PageSize:   kk.Int64(pageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := teamAPI.ListPortalTeams(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list portal teams", err, helper.GetCmd(), attrs...)
		}

		if res.GetListPortalTeamsResponse() == nil {
			break
		}

		data := res.GetListPortalTeamsResponse().GetData()
		all = append(all, data...)

		total := int(res.GetListPortalTeamsResponse().GetMeta().Page.Total)
		if total == 0 || len(all) >= total || len(data) == 0 {
			break
		}

		pageNumber++
	}

	return all, nil
}

func findTeamByName(teams []kkComps.PortalTeamResponse, identifier string) *kkComps.PortalTeamResponse {
	lowered := strings.ToLower(identifier)
	for _, team := range teams {
		if team.GetID() != nil && strings.ToLower(*team.GetID()) == lowered {
			teamCopy := team
			return &teamCopy
		}
		if team.GetName() != nil && strings.ToLower(*team.GetName()) == lowered {
			teamCopy := team
			return &teamCopy
		}
	}
	return nil
}

func portalTeamSummaryToRecord(team kkComps.PortalTeamResponse) portalTeamSummaryRecord {
	id := optionalPtr(team.GetID())
	if id != valueNA {
		id = util.AbbreviateUUID(id)
	}
	return portalTeamSummaryRecord{
		ID:               id,
		Name:             optionalPtr(team.GetName()),
		Description:      optionalPtr(team.GetDescription()),
		LocalCreatedTime: formatTimePtr(team.GetCreatedAt()),
		LocalUpdatedTime: formatTimePtr(team.GetUpdatedAt()),
	}
}

func optionalPtr(value *string) string {
	if value == nil || *value == "" {
		return valueNA
	}
	return *value
}

func formatTimePtr(value *time.Time) string {
	if value == nil || value.IsZero() {
		return valueNA
	}
	return value.In(time.Local).Format("2006-01-02 15:04:05")
}

func portalTeamDetailView(team kkComps.PortalTeamResponse) string {
	var b strings.Builder
	id := optionalPtr(team.GetID())
	if id != valueNA {
		id = util.AbbreviateUUID(id)
	}
	fmt.Fprintf(&b, "Name: %s\n", optionalPtr(team.GetName()))
	fmt.Fprintf(&b, "ID: %s\n", id)
	fmt.Fprintf(&b, "Description: %s\n", optionalPtr(team.GetDescription()))
	fmt.Fprintf(&b, "Created: %s\n", formatTimePtr(team.GetCreatedAt()))
	fmt.Fprintf(&b, "Updated: %s\n", formatTimePtr(team.GetUpdatedAt()))

	return b.String()
}
