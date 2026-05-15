package export

import (
	"context"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
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
)

func NewExportCmd() (*cobra.Command, error) {
	// Create the konnect subcommand first to get its implementation
	konnectCmd, err := konnect.NewKonnectCmd(Verb)
	if err != nil {
		return nil, err
	}

	cmd := &cobra.Command{
		Use:     exportUse,
		Short:   exportShort,
		Long:    exportLong,
		Example: konnectCmd.Example,
		// Use the konnect command's RunE directly for Konnect-first pattern
		RunE: konnectCmd.RunE,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			cmd.SetContext(context.WithValue(cmd.Context(), verbs.Verb, Verb))
		},
	}

	// Copy flags from konnect command to parent
	cmd.Flags().AddFlagSet(konnectCmd.Flags())

	// Also add konnect as a subcommand for explicit usage
	cmd.AddCommand(konnectCmd)

	return cmd, nil
}
