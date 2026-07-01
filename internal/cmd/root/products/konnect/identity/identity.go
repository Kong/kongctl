package identity

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const CommandName = "identity"

var (
	identityShort = i18n.T("root.products.konnect.identity.identityShort",
		"Manage Konnect Identity resources")
	identityLong = normalizers.LongDesc(i18n.T("root.products.konnect.identity.identityLong",
		`The identity command manages Kong Identity resources such as directories.`))
	identityExample = normalizers.Examples(
		i18n.T("root.products.konnect.identity.identityExamples",
			fmt.Sprintf(`
	# List all Kong Identity directories
	%[1]s get identity directories
	# Get a specific Kong Identity directory
	%[1]s get identity directory <id|name>
	`, meta.CLIName)),
	)
)

func NewIdentityCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := &cobra.Command{
		Use:     CommandName,
		Short:   identityShort,
		Long:    identityLong,
		Example: identityExample,
		Aliases: []string{"identities"},
	}

	switch verb {
	case verbs.Get, verbs.List:
		cmd.ConfigureRequiresSubcommand(baseCmd)
		directoryCmd := newDirectoryCmd(verb, addParentFlags, parentPreRun)
		baseCmd.AddCommand(directoryCmd)
	case verbs.Adopt:
		cmd.ConfigureRequiresSubcommand(baseCmd)
		directoryCmd := newAdoptDirectoryCmd(verb, addParentFlags, parentPreRun)
		baseCmd.AddCommand(directoryCmd)
	case verbs.Add,
		verbs.Listen,
		verbs.Apply,
		verbs.Lint,
		verbs.API,
		verbs.Create,
		verbs.Dump,
		verbs.Update,
		verbs.Delete,
		verbs.Help,
		verbs.Login,
		verbs.Install,
		verbs.Link,
		verbs.Logout,
		verbs.Plan,
		verbs.View,
		verbs.Sync,
		verbs.Diff,
		verbs.Export,
		verbs.Patch,
		verbs.Explain,
		verbs.Scaffold,
		verbs.Uninstall,
		verbs.Upgrade:
		if addParentFlags != nil {
			addParentFlags(verb, baseCmd)
		}
		if parentPreRun != nil {
			baseCmd.PreRunE = parentPreRun
		}
	}

	return baseCmd, nil
}
