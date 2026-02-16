package portal

import (
	"fmt"
	"strings"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/charmbracelet/bubbles/table"
	"github.com/kong/kongctl/internal/cmd"
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
	developerTeamsCommandName = "teams"
)

type portalTeamDeveloperRecord struct {
	ID               string `json:"id"`
	Email            string `json:"email"`
	FullName         string `json:"full_name"`
	Active           string `json:"active"`
	LocalCreatedTime string `json:"created_at"`
	LocalUpdatedTime string `json:"updated_at"`
}

var (
	developerTeamsUse = developerTeamsCommandName

	developerTeamsShort = i18n.T("root.products.konnect.portal.developerTeamsShort",
		"List developers assigned to a portal team")
	developerTeamsLong = normalizers.LongDesc(i18n.T("root.products.konnect.portal.developerTeamsLong",
		`Use the teams subcommand to list developers that belong to a specific portal team.`))
	developerTeamsExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.developerTeamsExamples",
			fmt.Sprintf(`
# List developers for a team by ID
%[1]s get portal developers teams --portal-id <portal-id> --team-id <team-id>
# List developers for a team by name
%[1]s get portal developers teams --portal-name my-portal --team-name backend-team
`, meta.CLIName)))
)

func newGetPortalDeveloperTeamsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     developerTeamsUse,
		Short:   developerTeamsShort,
		Long:    developerTeamsLong,
		Example: developerTeamsExample,
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
			handler := portalDeveloperTeamsHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addPortalChildFlags(cmd)
	bindDeveloperTeamFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

func bindDeveloperTeamFlags(cmd *cobra.Command) {
	cmd.Flags().String(teamIDFlagName, "", "Team ID to list developers for")
	cmd.Flags().String(teamNameFlagName, "", "Team name to list developers for")
}

type portalDeveloperTeamsHandler struct {
	cmd *cobra.Command
}

func (h portalDeveloperTeamsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("portal developer teams does not accept positional arguments; use flags to scope results"),
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

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

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

	membershipAPI := sdk.GetPortalTeamMembershipAPI()
	if membershipAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Portal team membership client is not available",
			Err: fmt.Errorf("portal team membership client not configured"),
		}
	}

	teamIDFlag := strings.TrimSpace(h.cmd.Flag(teamIDFlagName).Value.String())
	teamNameFlag := strings.TrimSpace(h.cmd.Flag(teamNameFlagName).Value.String())

	if teamIDFlag != "" && teamNameFlag != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", teamIDFlagName, teamNameFlagName),
		}
	}

	var teamID string
	var teamName string

	switch {
	case teamNameFlag != "":
		teamAPI := sdk.GetPortalTeamAPI()
		if teamAPI == nil {
			return &cmd.ExecutionError{
				Msg: "Portal teams client is not available",
				Err: fmt.Errorf("portal teams client not configured"),
			}
		}

		teams, err := fetchPortalTeams(helper, teamAPI, portalID, cfg)
		if err != nil {
			return err
		}
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
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"a team identifier is required. Provide --%s or --%s",
				teamIDFlagName,
				teamNameFlagName,
			),
		}
	}

	developers, err := fetchPortalTeamDevelopers(helper, membershipAPI, portalID, teamID, cfg)
	if err != nil {
		return err
	}

	records := make([]portalTeamDeveloperRecord, 0, len(developers))
	for _, dev := range developers {
		records = append(records, portalTeamDeveloperSummaryToRecord(dev))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Email, record.Active})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(developers) {
			return ""
		}
		return portalTeamDeveloperDetailView(developers[index])
	}

	rootLabel := helper.GetCmd().Name()
	if teamName != "" {
		rootLabel = fmt.Sprintf("%s (%s)", rootLabel, teamName)
	}

	return tableview.RenderForFormat(
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		developers,
		"",
		tableview.WithCustomTable([]string{"ID", "EMAIL", "ACTIVE"}, tableRows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(rootLabel),
	)
}

func fetchPortalTeamDevelopers(
	helper cmd.Helper,
	membershipAPI helpers.PortalTeamMembershipAPI,
	portalID string,
	teamID string,
	cfg config.Hook,
) ([]kkComps.BasicDeveloper, error) {
	var pageNumber int64 = 1
	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if pageSize < 1 {
		pageSize = int64(common.DefaultRequestPageSize)
	}

	var all []kkComps.BasicDeveloper

	for {
		req := kkOps.ListPortalTeamDevelopersRequest{
			PortalID:   portalID,
			TeamID:     teamID,
			PageSize:   kk.Int64(pageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := membershipAPI.ListPortalTeamDevelopers(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError(
				"Failed to list portal team developers",
				err,
				helper.GetCmd(),
				attrs...)
		}

		if res.GetListBasicDevelopersResponse() == nil {
			break
		}

		data := res.GetListBasicDevelopersResponse().GetData()
		all = append(all, data...)

		meta := res.GetListBasicDevelopersResponse().GetMeta()
		total := 0
		if meta != nil {
			total = int(meta.GetPage().Total)
		}

		if total == 0 || len(all) >= total || len(data) == 0 {
			break
		}

		pageNumber++
	}

	return all, nil
}

func portalTeamDeveloperSummaryToRecord(dev kkComps.BasicDeveloper) portalTeamDeveloperRecord {
	id := valueNA
	if dev.GetID() != nil && *dev.GetID() != "" {
		id = util.AbbreviateUUID(*dev.GetID())
	}

	return portalTeamDeveloperRecord{
		ID:               id,
		Email:            optionalPtr(dev.GetEmail()),
		FullName:         optionalPtr(dev.GetFullName()),
		Active:           optionalBool(dev.GetActive()),
		LocalCreatedTime: formatTimePtr(dev.GetCreatedAt()),
		LocalUpdatedTime: formatTimePtr(dev.GetUpdatedAt()),
	}
}

func portalTeamDeveloperDetailView(dev kkComps.BasicDeveloper) string {
	var b strings.Builder
	id := valueNA
	if dev.GetID() != nil && *dev.GetID() != "" {
		id = util.AbbreviateUUID(*dev.GetID())
	}
	fmt.Fprintf(&b, "Email: %s\n", optionalPtr(dev.GetEmail()))
	fmt.Fprintf(&b, "ID: %s\n", id)
	fmt.Fprintf(&b, "Full Name: %s\n", optionalPtr(dev.GetFullName()))
	fmt.Fprintf(&b, "Active: %s\n", optionalBool(dev.GetActive()))
	fmt.Fprintf(&b, "Created: %s\n", formatTimePtr(dev.GetCreatedAt()))
	fmt.Fprintf(&b, "Updated: %s\n", formatTimePtr(dev.GetUpdatedAt()))

	return b.String()
}

func optionalBool(value *bool) string {
	if value == nil {
		return valueNA
	}

	if *value {
		return "true"
	}

	return "false"
}
