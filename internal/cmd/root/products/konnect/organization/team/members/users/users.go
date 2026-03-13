package users

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
	CommandName = "user"
)

var (
	usersUse   = CommandName
	usersShort = i18n.T("root.products.konnect.organization.team.members.usersShort",
		"List users in a Konnect organization team")
	usersLong = normalizers.LongDesc(i18n.T("root.products.konnect.organization.team.members.usersLong",
		`Use the user command to list users that belong to a specific Konnect
organization team. An optional positional argument filters results: a UUID
matches users by ID, a value containing '@' is matched against email, and any
other value matches against the user's full name.`))
	usersExample = normalizers.Examples(i18n.T("root.products.konnect.organization.team.members.usersExamples",
		fmt.Sprintf(`
# List all users in a team by team ID
%[1]s get org team member users --team-id <team-id>
# List all users in a team by team name
%[1]s get org team member users --team-name my-team
# Filter users by email
%[1]s get org team member users --team-name my-team user@example.com
# Filter users by name
%[1]s get org team member users --team-name my-team "John Doe"
`, meta.CLIName)))
)

// NewUsersCmd creates the users sub-command.
func NewUsersCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	usersCmd := &cobra.Command{
		Use:     usersUse,
		Short:   usersShort,
		Long:    usersLong,
		Example: usersExample,
		Aliases: []string{"users", "User", "Users"},
		PreRunE: parentPreRun,
		RunE: func(c *cobra.Command, args []string) error {
			h := newGetUsersHandler(c)
			return h.run(args)
		},
	}

	members.AddTeamFlags(usersCmd)

	if addParentFlags != nil {
		addParentFlags(verb, usersCmd)
	}

	return usersCmd
}
