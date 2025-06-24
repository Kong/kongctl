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
		# Generate a plan from configuration directory
		%[1]s plan --dir ./config
		
		# Generate a plan and save to file
		%[1]s plan --dir ./config --output-file plan.json
		
		# Generate a plan for Konnect explicitly
		%[1]s plan konnect --dir ./config
		`, meta.CLIName)))
)

func NewPlanCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     planUse,
		Short:   planShort,
		Long:    planLong,
		Example: planExamples,
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