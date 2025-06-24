package diff

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
	Verb = verbs.Diff
)

var (
	diffUse = Verb.String()

	diffShort = i18n.T("root.verbs.diff.diffShort",
		"Display differences between current and desired state")

	diffLong = normalizers.LongDesc(i18n.T("root.verbs.diff.diffLong",
		`Compare the current state with the desired state defined in declarative
configuration files and display the differences.

The diff output shows what changes would be made without actually applying them,
useful for reviewing changes before synchronization.`))

	diffExamples = normalizers.Examples(i18n.T("root.verbs.diff.diffExamples",
		fmt.Sprintf(`
		# Show differences from configuration directory
		%[1]s diff --dir ./config
		
		# Show differences with detailed output
		%[1]s diff --dir ./config --detailed
		
		# Show differences for Konnect explicitly
		%[1]s diff konnect --dir ./config
		`, meta.CLIName)))
)

func NewDiffCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     diffUse,
		Short:   diffShort,
		Long:    diffLong,
		Example: diffExamples,
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