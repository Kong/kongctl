package members

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/charmbracelet/bubbles/table"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

// teamMemberRecord is the generic JSON/YAML/table record for a single team member.
type teamMemberRecord struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
}

type getMembersHandler struct {
	cmd *cobra.Command
}

func newGetMembersHandler(c *cobra.Command) getMembersHandler {
	return getMembersHandler{cmd: c}
}

func (h getMembersHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"org team member does not accept positional arguments; " +
					"use 'members users' or 'members system-accounts' for filtered listing",
			),
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

	teamIDFlag, teamNameFlag := GetTeamIdentifiers(h.cmd)

	membershipAPI := sdk.GetTeamMembershipAPI()
	if membershipAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Team membership client is not available",
			Err: fmt.Errorf("team membership client not configured"),
		}
	}

	teamID, teamName, err := ResolveOrgTeamID(
		helper.GetContext(),
		teamIDFlag,
		teamNameFlag,
		sdk.GetOrganizationTeamAPI(),
		cfg,
	)
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}

	users, err := ListTeamUsers(helper, membershipAPI, teamID, cfg, nil)
	if err != nil {
		return err
	}

	systemAccounts, err := ListTeamSystemAccounts(helper, membershipAPI, teamID, cfg, nil)
	if err != nil {
		return err
	}

	records := make([]teamMemberRecord, 0, len(users)+len(systemAccounts))
	for i := range users {
		records = append(records, userToMemberRecord(&users[i]))
	}
	for i := range systemAccounts {
		records = append(records, systemAccountToMemberRecord(&systemAccounts[i]))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, r := range records {
		tableRows = append(tableRows, table.Row{r.Type, r.ID, r.Identifier})
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
		records,
		"",
		tableview.WithCustomTable([]string{"TYPE", "ID", "NAME/EMAIL"}, tableRows),
		tableview.WithRootLabel(rootLabel),
	)
}

// ListTeamUsers fetches all users belonging to the given team. The optional
// filter parameter constrains results server-side.
func ListTeamUsers(
	helper cmd.Helper,
	membershipAPI helpers.TeamMembershipAPI,
	teamID string,
	cfg config.Hook,
	filter *kkOps.ListTeamUsersQueryParamFilter,
) ([]kkComps.User, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allUsers []kkComps.User
	var totalFetched int

	for {
		req := kkOps.ListTeamUsersRequest{
			TeamID:     teamID,
			PageSize:   &requestPageSize,
			PageNumber: &pageNumber,
			Filter:     filter,
		}
		res, err := membershipAPI.ListTeamUsers(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list team users", err, helper.GetCmd(), attrs...)
		}

		col := res.GetUserCollection()
		data := col.Data
		allUsers = append(allUsers, data...)
		totalFetched += len(data)

		if col.Meta == nil || totalFetched >= int(col.Meta.Page.Total) {
			break
		}
		pageNumber++
	}

	return allUsers, nil
}

// ListTeamSystemAccounts fetches all system accounts belonging to the given team.
func ListTeamSystemAccounts(
	helper cmd.Helper,
	membershipAPI helpers.TeamMembershipAPI,
	teamID string,
	cfg config.Hook,
	filter *kkOps.GetTeamsTeamIDSystemAccountsQueryParamFilter,
) ([]kkComps.SystemAccount, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allSAs []kkComps.SystemAccount
	var totalFetched int

	for {
		req := kkOps.GetTeamsTeamIDSystemAccountsRequest{
			TeamID:     teamID,
			PageSize:   &requestPageSize,
			PageNumber: &pageNumber,
			Filter:     filter,
		}
		res, err := membershipAPI.ListTeamSystemAccounts(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list team system accounts", err, helper.GetCmd(), attrs...)
		}

		col := res.GetSystemAccountCollection()
		data := col.Data
		allSAs = append(allSAs, data...)
		totalFetched += len(data)

		if col.Meta == nil || totalFetched >= int(col.Meta.Page.Total) {
			break
		}
		pageNumber++
	}

	return allSAs, nil
}

// userToMemberRecord converts a User to a generic teamMemberRecord.
func userToMemberRecord(u *kkComps.User) teamMemberRecord {
	const missing = "n/a"
	r := teamMemberRecord{Type: "user", ID: missing, Identifier: missing}
	if u == nil {
		return r
	}
	if id := u.GetID(); id != nil && *id != "" {
		r.ID = util.AbbreviateUUID(*id)
	}
	if email := u.GetEmail(); email != nil && *email != "" {
		r.Identifier = *email
	}
	return r
}

// systemAccountToMemberRecord converts a SystemAccount to a generic teamMemberRecord.
func systemAccountToMemberRecord(sa *kkComps.SystemAccount) teamMemberRecord {
	const missing = "n/a"
	r := teamMemberRecord{Type: "system-account", ID: missing, Identifier: missing}
	if sa == nil {
		return r
	}
	if id := sa.GetID(); id != nil && *id != "" {
		r.ID = util.AbbreviateUUID(*id)
	}
	if name := sa.GetName(); name != nil && *name != "" {
		r.Identifier = *name
	}
	return r
}
