package user

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const CommandName = "user"

var (
	userUse   = CommandName
	userShort = i18n.T("root.products.konnect.organization.user.userShort",
		"Manage Konnect organization user resources")
	userLong = normalizers.LongDesc(i18n.T("root.products.konnect.organization.user.userLong",
		`The user command allows you to view Konnect organization users and direct user role assignments.`))
	userExample = normalizers.Examples(i18n.T("root.products.konnect.organization.user.userExamples",
		fmt.Sprintf(`
# List all organization users
%[1]s get org users
# Get a specific organization user
%[1]s get org user <id|email>
# List direct roles assigned to an organization user
%[1]s get org user roles --user-email user@example.com
`, meta.CLIName)))
)

func NewUserCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     userUse,
		Short:   userShort,
		Long:    userLong,
		Example: userExample,
		Aliases: []string{"users", "User", "Users", "USER", "USERS"},
	}

	if verb == verbs.Get || verb == verbs.List {
		cmd := newGetUserCmd(verb, &baseCmd, addParentFlags, parentPreRun)
		if rolesCmd := newGetOrganizationUserRolesCmd(verb, addParentFlags, parentPreRun); rolesCmd != nil {
			cmd.AddCommand(rolesCmd)
		}
		return cmd.Command, nil
	}

	return &baseCmd, nil
}
