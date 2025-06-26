package apply

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
	Verb = verbs.Apply
)

var (
	applyUse = Verb.String()

	applyShort = i18n.T("root.verbs.apply.applyShort", "Apply declarative configuration")

	applyLong = normalizers.LongDesc(i18n.T("root.verbs.apply.applyLong",
		`Apply declarative configuration files to target environment.

Apply reads the configuration files and makes the necessary API calls to create,
update, or delete resources to match the desired state.`))

	applyExamples = normalizers.Examples(i18n.T("root.verbs.apply.applyExamples",
		fmt.Sprintf(`
		# Apply configuration from files
		%[1]s apply -f portal.yaml -f auth.yaml
		
		# Apply configuration from comma-separated files
		%[1]s apply -f portal.yaml,auth.yaml,api.yaml
		
		# Apply configuration from directory
		%[1]s apply -f ./config
		
		# Apply configuration from directory recursively
		%[1]s apply -f ./config -R
		
		# Apply configuration with force flag
		%[1]s apply -f ./config --force
		
		# Apply configuration from stdin
		cat portal.yaml | %[1]s apply -f -
		`, meta.CLIName)))
)

func NewApplyCmd() (*cobra.Command, error) {
	// Create the konnect subcommand first to get its implementation
	konnectCmd, err := konnect.NewKonnectCmd(Verb)
	if err != nil {
		return nil, err
	}

	cmd := &cobra.Command{
		Use:     applyUse,
		Short:   applyShort,
		Long:    applyLong,
		Example: applyExamples,
		Aliases: []string{"a", "A"},
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
