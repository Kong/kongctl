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
		# Show differences from an existing plan file
		%[1]s diff --plan plan.json
		
		# Show differences from a plan on stdin
		cat plan.json | %[1]s diff --plan -
		
		# Generate plan and pipe to diff for immediate review
		%[1]s plan -f portal.yaml | %[1]s diff --plan -
		
		# Show differences in JSON format
		%[1]s diff --plan plan.json -o json
		
		# Show differences in YAML format
		%[1]s diff --plan plan.json -o yaml
		
		# Generate and review plan from configuration files
		%[1]s diff -f portal.yaml -f auth.yaml
		
		# Generate and review plan from directory
		%[1]s diff -f ./config -R
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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cmd.SetContext(context.WithValue(cmd.Context(), verbs.Verb, Verb))
			// Also run the konnect command's PersistentPreRunE to set up SDKAPIFactory
			if konnectCmd.PersistentPreRunE != nil {
				return konnectCmd.PersistentPreRunE(cmd, args)
			}
			return nil
		},
	}

	// Copy flags from konnect command to parent
	cmd.Flags().AddFlagSet(konnectCmd.Flags())

	// Also add konnect as a subcommand for explicit usage
	cmd.AddCommand(konnectCmd)

	return cmd, nil
}