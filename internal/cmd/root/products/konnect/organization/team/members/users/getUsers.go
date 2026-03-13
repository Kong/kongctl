package users

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

// userRecord is the structured record for displaying a team user.
type userRecord struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Active   string `json:"active"`
}

type getUsersHandler struct {
	cmd *cobra.Command
}

func newGetUsersHandler(c *cobra.Command) getUsersHandler {
	return getUsersHandler{cmd: c}
}

func (h getUsersHandler) run(args []string) error {
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
	var reqFilter *kkOps.ListTeamUsersQueryParamFilter
	if filter != "" {
		reqFilter = buildUserFilter(filter)
	}

	allUsers, err := members.ListTeamUsers(helper, membershipAPI, teamID, cfg, reqFilter)
	if err != nil {
		return err
	}

	// Client-side filter for cases where the argument can't map to a single
	// server-side predicate.
	if filter != "" {
		allUsers = clientFilterUsers(allUsers, filter)
	}

	records := make([]userRecord, 0, len(allUsers))
	for i := range allUsers {
		records = append(records, userToRecord(&allUsers[i]))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, r := range records {
		tableRows = append(tableRows, table.Row{r.ID, r.Email, r.FullName, r.Active})
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
		allUsers,
		"",
		tableview.WithCustomTable([]string{"ID", "EMAIL", "FULL NAME", "ACTIVE"}, tableRows),
		tableview.WithRootLabel(rootLabel),
	)
}

// buildUserFilter constructs a server-side filter that covers ID (exact),
// email (contains), and full_name (contains) based on the raw filter string.
// We send all three and let the server resolve whichever is relevant. When the
// argument looks like a UUID we use only the ID equals filter.
func buildUserFilter(filter string) *kkOps.ListTeamUsersQueryParamFilter {
	f := &kkOps.ListTeamUsersQueryParamFilter{}
	if util.IsValidUUID(filter) {
		f.ID = &kkComps.StringFieldEqualsFilter{Eq: &filter}
		return f
	}
	// Use email filter when the argument contains '@'.
	if strings.Contains(filter, "@") {
		f.Email = &kkComps.LegacyStringFieldFilter{Eq: &filter}
		return f
	}
	// Generic string: filter on full_name
	f.FullName = &kkComps.LegacyStringFieldFilter{Eq: &filter}
	return f
}

// clientFilterUsers performs an additional client-side pass over users in case
// the server didn't narrow results sufficiently (e.g. when the server applies
// OR semantics across multiple filter fields).
func clientFilterUsers(users []kkComps.User, filter string) []kkComps.User {
	var result []kkComps.User
	for _, u := range users {
		if matchesUser(&u, filter) {
			result = append(result, u)
		}
	}
	return result
}

func matchesUser(u *kkComps.User, filter string) bool {
	if id := u.GetID(); id != nil && strings.EqualFold(*id, filter) {
		return true
	}
	if email := u.GetEmail(); email != nil && strings.EqualFold(*email, filter) {
		return true
	}
	if name := u.GetFullName(); name != nil && strings.EqualFold(*name, filter) {
		return true
	}
	return false
}

// userToRecord converts a User to a displayable userRecord.
func userToRecord(u *kkComps.User) userRecord {
	const missing = "n/a"
	r := userRecord{ID: missing, Email: missing, FullName: missing, Active: missing}
	if u == nil {
		return r
	}
	if id := u.GetID(); id != nil && *id != "" {
		r.ID = util.AbbreviateUUID(*id)
	}
	if email := u.GetEmail(); email != nil {
		r.Email = *email
	}
	if name := u.GetFullName(); name != nil {
		r.FullName = *name
	}
	if active := u.GetActive(); active != nil {
		r.Active = fmt.Sprintf("%t", *active)
	}
	return r
}
