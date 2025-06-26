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
		# Show differences from configuration files
		%[1]s diff -f portal.yaml -f auth.yaml
		
		# Show differences from comma-separated files
		%[1]s diff -f portal.yaml,auth.yaml,api.yaml
		
		# Show differences from configuration directory
		%[1]s diff -f ./config
		
		# Show differences from directory recursively
		%[1]s diff -f ./config -R
		
		# Show differences with detailed output
		%[1]s diff -f ./config --detailed
		
		# Show differences from stdin
		cat portal.yaml | %[1]s diff -f -
		`, meta.CLIName)))
)

func NewDiffCmd() (*cobra.Command, error) {
	// Create the konnect subcommand first to get its implementation
	konnectCmd, err := konnect.NewKonnectCmd(Verb)
	if err != nil {
		return nil, err
	}

	cmd := &cobra.Command{
		Use:     diffUse,
		Short:   diffShort,
		Long:    diffLong,
		Example: diffExamples,
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