package systemaccounts

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect/organization/team/members"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "system-account"
)

var (
	systemAccountsUse   = CommandName
	systemAccountsShort = i18n.T(
		"root.products.konnect.organization.team.members.systemAccountsShort",
		"List system accounts in a Konnect organization team")
	systemAccountsLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.organization.team.members.systemAccountsLong",
		`Use the system-account command to list system accounts that belong to a
specific Konnect organization team. An optional positional argument filters
results: a UUID matches by ID, any other value is matched against the system
account name.`))
	systemAccountsExample = normalizers.Examples(i18n.T(
		"root.products.konnect.organization.team.members.systemAccountsExamples",
		fmt.Sprintf(`
# List all system accounts in a team by team ID
%[1]s get org team member system-accounts --team-id <team-id>
# List all system accounts in a team by team name
%[1]s get org team member system-accounts --team-name my-team
# Filter by name
%[1]s get org team member system-accounts --team-name my-team gateway-sa
`, meta.CLIName)))
)

// NewSystemAccountsCmd creates the system-accounts sub-command.
func NewSystemAccountsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	systemAccountsCmd := &cobra.Command{
		Use:     systemAccountsUse,
		Short:   systemAccountsShort,
		Long:    systemAccountsLong,
		Example: systemAccountsExample,
		Aliases: []string{"system-accounts", "systemaccount", "systemaccounts",
			"system_account", "system_accounts", "sa", "sas", "SA", "SAS"},
		PreRunE: parentPreRun,
		RunE: func(c *cobra.Command, args []string) error {
			h := newGetSystemAccountsHandler(c)
			return h.run(args)
		},
	}

	members.AddTeamFlags(systemAccountsCmd)

	if addParentFlags != nil {
		addParentFlags(verb, systemAccountsCmd)
	}

	return systemAccountsCmd
}
