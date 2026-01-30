package organization

import (
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/organization/team"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/spf13/cobra"
)

func NewOrganizationCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	cmd := baseCmd
	if cmd == nil {
		cmd = &cobra.Command{}
	}

	cmd.Use = "organization"
	cmd.Short = "Adopt organization resources into namespace management"
	cmd.Long = "Manage organization-level resources such as teams for adoption into namespace management"

	// Make org command require a subcommand - it should not run on its own
	cmd.RunE = func(c *cobra.Command, _ []string) error {
		return c.Help()
	}

	// Add team subcommand
	teamCmd, err := team.NewTeamCmd(verb, &cobra.Command{}, addParentFlags, parentPreRun)
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(teamCmd)

	return cmd, nil
}
