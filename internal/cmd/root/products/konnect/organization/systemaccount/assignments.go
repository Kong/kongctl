package systemaccount

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/table"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	systemAccountRolesCommandName = "roles"
	systemAccountTeamsCommandName = "teams"
	systemAccountIDFlagName       = "system-account-id"
	systemAccountNameFlagName     = "system-account-name"
)

type systemAccountRoleRecord struct {
	ID              string `json:"id"                yaml:"id"`
	SystemAccountID string `json:"system_account_id" yaml:"system_account_id"`
	RoleName        string `json:"role_name"         yaml:"role_name"`
	EntityID        string `json:"entity_id"         yaml:"entity_id"`
	EntityTypeName  string `json:"entity_type_name"  yaml:"entity_type_name"`
	EntityRegion    string `json:"entity_region"     yaml:"entity_region"`
}

type systemAccountTeamRecord struct {
	ID              string `json:"id"                yaml:"id"`
	SystemAccountID string `json:"system_account_id" yaml:"system_account_id"`
	Name            string `json:"name"              yaml:"name"`
}

func newGetSystemAccountRolesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     systemAccountRolesCommandName,
		Short:   "List organization system account role assignments",
		Long:    "List direct role assignments for a Konnect organization system account.",
		PreRunE: parentPreRun,
		RunE: func(c *cobra.Command, args []string) error {
			return systemAccountAssignmentsHandler{cmd: c, kind: systemAccountRolesCommandName}.run(args)
		},
	}
	addSystemAccountAssignmentFlags(verb, addParentFlags, c)
	return c
}

func newGetSystemAccountTeamsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     systemAccountTeamsCommandName,
		Short:   "List organization system account team memberships",
		Long:    "List organization team memberships for a Konnect organization system account.",
		PreRunE: parentPreRun,
		RunE: func(c *cobra.Command, args []string) error {
			return systemAccountAssignmentsHandler{cmd: c, kind: systemAccountTeamsCommandName}.run(args)
		},
	}
	addSystemAccountAssignmentFlags(verb, addParentFlags, c)
	return c
}

func addSystemAccountAssignmentFlags(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	c *cobra.Command,
) {
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	c.Flags().String(systemAccountIDFlagName, "", "System account ID to list assignments for")
	c.Flags().String(systemAccountNameFlagName, "", "System account name to list assignments for")
}

type systemAccountAssignmentsHandler struct {
	cmd  *cobra.Command
	kind string
}

func (h systemAccountAssignmentsHandler) run(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf(
			"organization system account %s does not accept positional arguments; "+
				"use --system-account-id or --system-account-name",
			h.kind,
		)
	}

	accountID, _ := h.cmd.Flags().GetString(systemAccountIDFlagName)
	accountName, _ := h.cmd.Flags().GetString(systemAccountNameFlagName)
	if strings.TrimSpace(accountID) != "" && strings.TrimSpace(accountName) != "" {
		return fmt.Errorf("--system-account-id and --system-account-name cannot be used together")
	}
	if strings.TrimSpace(accountID) == "" && strings.TrimSpace(accountName) == "" {
		return fmt.Errorf("one of --system-account-id or --system-account-name is required")
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

	if strings.TrimSpace(accountName) != "" {
		account, err := resolveSystemAccountByNameStrict(accountName, sdk.GetSystemAccountAPI(), helper, cfg)
		if err != nil {
			return err
		}
		if account.ID == nil || *account.ID == "" {
			return fmt.Errorf("system account %q has no ID", accountName)
		}
		accountID = *account.ID
	}

	switch h.kind {
	case systemAccountRolesCommandName:
		roles, err := fetchSystemAccountRoles(helper, sdk.GetSystemAccountRolesAPI(), accountID)
		if err != nil {
			return err
		}
		return renderSystemAccountRoles(helper, outType, printer, accountID, roles)
	case systemAccountTeamsCommandName:
		teams, err := fetchSystemAccountTeams(helper, sdk.GetSystemAccountTeamMembershipAPI(), accountID, cfg)
		if err != nil {
			return err
		}
		return renderSystemAccountTeams(helper, outType, printer, accountID, teams)
	default:
		return fmt.Errorf("unsupported system account assignment type %q", h.kind)
	}
}

func resolveSystemAccountByNameStrict(
	name string,
	kkClient helpers.SystemAccountAPI,
	helper cmd.Helper,
	cfg config.Hook,
) (*kkComps.SystemAccount, error) {
	accounts, err := listSystemAccountsByName(name, kkClient, helper, cfg)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("system account with name %q not found; check --system-account-name", name)
	}
	if len(accounts) > 1 {
		return nil, fmt.Errorf("system account name %q matched multiple system accounts", name)
	}
	return &accounts[0], nil
}

func fetchSystemAccountRoles(
	helper cmd.Helper,
	roleAPI helpers.SystemAccountRolesAPI,
	accountID string,
) ([]kkComps.AssignedRole, error) {
	if roleAPI == nil {
		return nil, fmt.Errorf("system account roles client is not available")
	}

	res, err := roleAPI.ListSystemAccountRoles(helper.GetContext(), accountID, nil)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to list system account roles", err, helper.GetCmd(), attrs...)
	}
	if res == nil || res.AssignedRoleCollection == nil {
		return []kkComps.AssignedRole{}, nil
	}
	return res.AssignedRoleCollection.Data, nil
}

func fetchSystemAccountTeams(
	helper cmd.Helper,
	membershipAPI helpers.SystemAccountTeamMembershipAPI,
	accountID string,
	cfg config.Hook,
) ([]kkComps.Team, error) {
	if membershipAPI == nil {
		return nil, fmt.Errorf("system account team membership client is not available")
	}

	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var teams []kkComps.Team
	for {
		res, err := membershipAPI.ListSystemAccountTeams(
			helper.GetContext(),
			kkOps.GetSystemAccountsAccountIDTeamsRequest{
				AccountID:  accountID,
				PageSize:   &requestPageSize,
				PageNumber: &pageNumber,
			},
		)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list system account teams", err, helper.GetCmd(), attrs...)
		}
		if res == nil || res.TeamCollection == nil {
			return teams, nil
		}
		teams = append(teams, res.TeamCollection.Data...)
		totalItems := res.TeamCollection.GetMeta().GetPage().Total
		if len(teams) >= int(totalItems) {
			break
		}
		pageNumber++
	}
	return teams, nil
}

func systemAccountRoleToRecord(role kkComps.AssignedRole, accountID string) systemAccountRoleRecord {
	record := systemAccountRoleRecord{SystemAccountID: accountID}
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

func systemAccountTeamToRecord(team kkComps.Team, accountID string) systemAccountTeamRecord {
	record := systemAccountTeamRecord{SystemAccountID: accountID}
	if team.ID != nil {
		record.ID = *team.ID
	}
	if team.Name != nil {
		record.Name = *team.Name
	}
	return record
}

func renderSystemAccountRoles(
	helper cmd.Helper,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	accountID string,
	roles []kkComps.AssignedRole,
) error {
	records := make([]systemAccountRoleRecord, 0, len(roles))
	for _, role := range roles {
		records = append(records, systemAccountRoleToRecord(role, accountID))
	}

	return tableview.RenderForFormat(
		helper, false, outType, printer, helper.GetStreams(), records, records, "",
		tableview.WithRootLabel(common.ViewFieldUserRoles),
		tableview.WithDetailHelper(helper),
	)
}

func renderSystemAccountTeams(
	helper cmd.Helper,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	accountID string,
	teams []kkComps.Team,
) error {
	records := make([]systemAccountTeamRecord, 0, len(teams))
	for _, team := range teams {
		records = append(records, systemAccountTeamToRecord(team, accountID))
	}

	return tableview.RenderForFormat(
		helper, false, outType, printer, helper.GetStreams(), records, records, "",
		tableview.WithRootLabel(common.ViewFieldTeams),
		tableview.WithDetailHelper(helper),
	)
}

func buildSystemAccountRolesChildView(accountID string, roles []kkComps.AssignedRole) tableview.ChildView {
	records := make([]systemAccountRoleRecord, 0, len(roles))
	rows := make([]table.Row, 0, len(roles))
	for _, role := range roles {
		record := systemAccountRoleToRecord(role, accountID)
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
			fmt.Fprintf(&b, "system_account_id: %s\n", r.SystemAccountID)
			fmt.Fprintf(&b, "role_name: %s\n", r.RoleName)
			fmt.Fprintf(&b, "entity_id: %s\n", r.EntityID)
			fmt.Fprintf(&b, "entity_type_name: %s\n", r.EntityTypeName)
			fmt.Fprintf(&b, "entity_region: %s\n", r.EntityRegion)
			return strings.TrimRight(b.String(), "\n")
		},
		Title:      "Roles",
		ParentType: common.ViewParentSystemAccount,
	}
}

func buildSystemAccountTeamsChildView(accountID string, teams []kkComps.Team) tableview.ChildView {
	records := make([]systemAccountTeamRecord, 0, len(teams))
	rows := make([]table.Row, 0, len(teams))
	for _, team := range teams {
		record := systemAccountTeamToRecord(team, accountID)
		records = append(records, record)
		rows = append(rows, table.Row{record.ID, record.Name})
	}

	return tableview.ChildView{
		Headers: []string{"ID", "NAME"},
		Rows:    rows,
		DetailRenderer: func(index int) string {
			if index < 0 || index >= len(records) {
				return ""
			}
			r := records[index]
			var b strings.Builder
			fmt.Fprintf(&b, "id: %s\n", r.ID)
			fmt.Fprintf(&b, "system_account_id: %s\n", r.SystemAccountID)
			fmt.Fprintf(&b, "name: %s\n", r.Name)
			return strings.TrimRight(b.String(), "\n")
		},
		Title:      "Teams",
		ParentType: common.ViewParentSystemAccount,
	}
}
