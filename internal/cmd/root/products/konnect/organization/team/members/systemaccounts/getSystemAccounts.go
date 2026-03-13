package systemaccounts

import (
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/charmbracelet/bubbles/table"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/organization/team/members"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

// systemAccountRecord is the structured record for displaying a team system account.
type systemAccountRecord struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type getSystemAccountsHandler struct {
	cmd *cobra.Command
}

func newGetSystemAccountsHandler(c *cobra.Command) getSystemAccountsHandler {
	return getSystemAccountsHandler{cmd: c}
}

func (h getSystemAccountsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return fmt.Errorf("too many arguments; expected at most one filter value")
	}

	filter := ""
	if len(args) == 1 {
		filter = strings.TrimSpace(args[0])
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

	teamIDFlag, teamNameFlag := members.GetTeamIdentifiers(h.cmd)

	membershipAPI := sdk.GetTeamMembershipAPI()
	if membershipAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Team membership client is not available",
			Err: fmt.Errorf("team membership client not configured"),
		}
	}

	teamID, teamName, err := members.ResolveOrgTeamID(
		helper.GetContext(),
		teamIDFlag,
		teamNameFlag,
		sdk.GetOrganizationTeamAPI(),
		cfg,
	)
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}

	// Build server-side filter when a search argument is provided.
	var reqFilter *kkOps.GetTeamsTeamIDSystemAccountsQueryParamFilter
	if filter != "" && !util.IsValidUUID(filter) {
		reqFilter = &kkOps.GetTeamsTeamIDSystemAccountsQueryParamFilter{
			Name: &kkComps.LegacyStringFieldFilter{Eq: &filter},
		}
	}

	allSAs, err := members.ListTeamSystemAccounts(helper, membershipAPI, teamID, cfg, reqFilter)
	if err != nil {
		return err
	}

	// Client-side filter by ID (UUID case) or additional name narrowing.
	if filter != "" {
		allSAs = clientFilterSAs(allSAs, filter)
	}

	records := make([]systemAccountRecord, 0, len(allSAs))
	for i := range allSAs {
		records = append(records, saToRecord(&allSAs[i]))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, r := range records {
		tableRows = append(tableRows, table.Row{r.ID, r.Name, r.Description})
	}

	rootLabel := helper.GetCmd().Name()
	if teamName != "" && teamName != teamID {
		rootLabel = fmt.Sprintf("%s (%s)", rootLabel, teamName)
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		allSAs,
		"",
		tableview.WithCustomTable([]string{"ID", "NAME", "DESCRIPTION"}, tableRows),
		tableview.WithRootLabel(rootLabel),
	)
}

// clientFilterSAs filters system accounts by ID (exact UUID) or name (contains).
func clientFilterSAs(sas []kkComps.SystemAccount, filter string) []kkComps.SystemAccount {
	var result []kkComps.SystemAccount
	for _, sa := range sas {
		if matchesSA(&sa, filter) {
			result = append(result, sa)
		}
	}
	return result
}

func matchesSA(sa *kkComps.SystemAccount, filter string) bool {
	if id := sa.GetID(); id != nil && strings.EqualFold(*id, filter) {
		return true
	}
	if name := sa.GetName(); name != nil && strings.EqualFold(*name, filter) {
		return true
	}
	return false
}

// saToRecord converts a SystemAccount to a displayable systemAccountRecord.
func saToRecord(sa *kkComps.SystemAccount) systemAccountRecord {
	const missing = "n/a"
	r := systemAccountRecord{ID: missing, Name: missing, Description: missing}
	if sa == nil {
		return r
	}
	if id := sa.GetID(); id != nil && *id != "" {
		r.ID = util.AbbreviateUUID(*id)
	}
	if name := sa.GetName(); name != nil {
		r.Name = *name
	}
	if desc := sa.GetDescription(); desc != nil {
		r.Description = *desc
	}
	return r
}
