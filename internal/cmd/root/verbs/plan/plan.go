package plan

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
	Verb = verbs.Plan
)

var (
	planUse = Verb.String()

	planShort = i18n.T("root.verbs.plan.planShort",
		"Generate a declarative configuration plan artifact")

	planLong = normalizers.LongDesc(i18n.T("root.verbs.plan.planLong",
		`Generate a plan artifact from declarative configuration files.

The plan artifact represents the desired state and can be used for review,
approval workflows, or as input to sync operations.`))

	planExamples = normalizers.Examples(i18n.T("root.verbs.plan.planExamples",
		fmt.Sprintf(`
		# Generate a plan from configuration files
		%[1]s plan -f portal.yaml -f auth.yaml
		
		# Generate a plan from comma-separated files
		%[1]s plan -f portal.yaml,auth.yaml,api.yaml
		
		# Generate a plan from a directory
		%[1]s plan -f ./config
		
		# Generate a plan from directory recursively
		%[1]s plan -f ./config -R
		
		# Generate a plan and save to file
		%[1]s plan -f ./config --output-file plan.json
		
		# Generate a plan from stdin
		cat portal.yaml | %[1]s plan -f -
		`, meta.CLIName)))
)

func NewPlanCmd() (*cobra.Command, error) {
	// Create the konnect subcommand first to get its implementation
	konnectCmd, err := konnect.NewKonnectCmd(Verb)
	if err != nil {
		return nil, err
	}

	cmd := &cobra.Command{
		Use:     planUse,
		Short:   planShort,
		Long:    planLong,
		Example: planExamples,
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