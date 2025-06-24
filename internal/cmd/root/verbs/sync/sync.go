package sync

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Verb = verbs.Sync
)

var (
	syncUse = Verb.String()

	syncShort = i18n.T("root.verbs.sync.syncShort",
		"Synchronize declarative configuration to target environment")

	syncLong = normalizers.LongDesc(i18n.T("root.verbs.sync.syncLong",
		`Synchronize declarative configuration files to the target environment.

Sync analyzes the current state, compares it with the desired state defined
in the configuration files, and applies the necessary changes to achieve
the desired state.`))

	syncExamples = normalizers.Examples(i18n.T("root.verbs.sync.syncExamples",
		fmt.Sprintf(`
		# Sync configuration from directory
		%[1]s sync --dir ./config
		
		# Sync configuration with dry-run to preview changes
		%[1]s sync --dir ./config --dry-run
		
		# Sync configuration for Konnect explicitly
		%[1]s sync konnect --dir ./config
		`, meta.CLIName)))
)

func NewSyncCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     syncUse,
		Short:   syncShort,
		Long:    syncLong,
		Example: syncExamples,
		RunE: func(cmd *cobra.Command, args []string) error {
			// When called directly without subcommand, redirect to konnect
			if len(args) == 0 && cmd.Flags().NArg() == 0 {
				// Find the konnect subcommand
				for _, subcmd := range cmd.Commands() {
					if subcmd.Name() == "konnect" {
						// Copy parent flags to subcommand
						subcmd.Flags().AddFlagSet(cmd.Flags())
						// Execute konnect subcommand
						return subcmd.RunE(subcmd, args)
					}
				}
			}
			// If we get here, show help
			return cmd.Help()
		},
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			cmd.SetContext(context.WithValue(cmd.Context(), verbs.Verb, Verb))
		},
	}

	// Add konnect subcommand
	c, e := konnect.NewKonnectCmd(Verb)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(c)

	return cmd, nil
}