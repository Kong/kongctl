package team

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName             = "team"
	skipSystemTeamsFlagName = "skip-system-teams"
)

var (
	teamUse   = CommandName
	teamShort = i18n.T("root.products.konnect.team.teamShort",
		"Manage Konnect team resources")
	teamLong = normalizers.LongDesc(i18n.T("root.products.konnect.team.teamLong",
		`The team command allows you to work with Konnect team resources.`))
	teamExample = normalizers.Examples(i18n.T("root.products.konnect.team.teamExamples",
		fmt.Sprintf(`
# List all teams
%[1]s get org teams
# Get a specific team
%[1]s get org team <id|name>
`, meta.CLIName)))
)

func NewTeamCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     teamUse,
		Short:   teamShort,
		Long:    teamLong,
		Example: teamExample,
		Aliases: []string{"teams", "Team", "Teams", "TEAM", "TEAMS"},
	}

	// Handle supported verbs
	if verb == verbs.Get || verb == verbs.List {
		return newGetTeamCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	}

	// Return base command for unsupported verbs
	return &baseCmd, nil
}
