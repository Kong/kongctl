package organization

import (
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/organization/team"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/spf13/cobra"
)

type adoptOrganizationCommand struct {
	*cobra.Command
}

func newAdoptOrganizationCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *adoptOrganizationCommand {
	cmd := &adoptOrganizationCommand{
		Command: baseCmd,
	}

	cmd.Short = "Adopt organization resources into namespace management"
	cmd.Long = "Manage organization-level resources such as teams for adoption into namespace management"

	// Make org command require a subcommand - it should not run on its own
	cmd.RunE = func(c *cobra.Command, _ []string) error {
		return c.Help()
	}

	teamCmd, err := team.NewTeamCmd(verb, &cobra.Command{}, addParentFlags, parentPreRun)
	if err != nil {
		return nil
	}
	cmd.AddCommand(teamCmd)

	return cmd
}
