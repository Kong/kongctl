package export

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
	Verb = verbs.Export
)

var (
	exportUse = Verb.String()

	exportShort = i18n.T("root.verbs.export.exportShort",
		"Export current state as declarative configuration")

	exportLong = normalizers.LongDesc(i18n.T("root.verbs.export.exportLong",
		`Export the current state of resources as declarative configuration files.

This command retrieves the current configuration from the target environment
and generates declarative configuration files that can be version controlled,
modified, and applied to other environments.`))

	exportExamples = normalizers.Examples(i18n.T("root.verbs.export.exportExamples",
		fmt.Sprintf(`
		# Export all resources to directory
		%[1]s export --dir ./exported-config
		
		# Export specific resource types
		%[1]s export --dir ./exported-config --resources portals,services
		
		# Export resources for Konnect explicitly
		%[1]s export konnect --dir ./exported-config
		`, meta.CLIName)))
)

func NewExportCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     exportUse,
		Short:   exportShort,
		Long:    exportLong,
		Example: exportExamples,
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