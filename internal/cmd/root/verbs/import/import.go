package importcmd

import (
	"fmt"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/identity"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/spf13/cobra"
)

const Verb = verbs.Import

func NewImportCmd() (*cobra.Command, error) {
	konnectCmd, err := konnect.NewKonnectCmd(Verb)
	if err != nil {
		return nil, err
	}
	command := &cobra.Command{
		Use:   Verb.String(),
		Short: "Import data into Konnect",
		Long: "Import data using native Konnect import operations. " +
			"Organization-chart imports validate or execute a complete employee roster and reporting hierarchy.",
		Example: fmt.Sprintf(`  %[1]s import id org-chart --file roster.csv --dry-run --report-output-file report.json
  %[1]s import konnect identity org-chart --file roster.csv --execute --report-output-file run.json`, meta.CLIName),
		PersistentPreRunE: verbs.KonnectFirstPreRunE(Verb, konnectCmd),
	}
	cmdpkg.ConfigureRequiresSubcommand(command)
	bindings := []struct{ flag, path, description string }{
		{common.BaseURLFlagName, common.BaseURLConfigPath, "Base URL for Konnect API requests."},
		{common.RegionFlagName, common.RegionConfigPath, "Konnect region identifier (for example eu)."},
		{common.PATFlagName, common.PATConfigPath, "Konnect Personal Access Token (PAT)."},
	}
	for _, binding := range bindings {
		command.PersistentFlags().String(binding.flag, "",
			fmt.Sprintf("%s\n- Config path: [ %s ]", binding.description, binding.path))
	}
	identityCmd, err := identity.NewIdentityCmd(Verb, nil, konnectCmd.PersistentPreRunE)
	if err != nil {
		return nil, err
	}
	command.AddCommand(konnectCmd, identityCmd)
	return command, nil
}
