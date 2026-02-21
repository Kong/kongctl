package plan

import (
	"context"
	"fmt"
	"strings"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
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
		"Preview changes to Kong Konnect resources")

	planLong = normalizers.LongDesc(i18n.T("root.verbs.plan.planLong",
		`Generate an execution plan showing what changes will be made.`))

	planExamples = normalizers.Examples(i18n.T("root.verbs.plan.planExamples",
		fmt.Sprintf(`  %[1]s plan -f api.yaml
  %[1]s plan -f ./configs/ --recursive
  %[1]s plan -f config.yaml --output-file plan.json

Use "%[1]s help plan" for detailed documentation`, meta.CLIName)))
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

	// Intercept parse-time errors for -o/--output and replace with an actionable message.
	// (pflag rejects non-enum values before RunE runs, so this is the only way to catch
	// e.g. `kongctl plan -o plan.json`.)
	outputFlagMsg := fmt.Sprintf(
		"flags -o/--%s are not supported for the plan command; use --output-file to save the plan to a file",
		cmdcommon.OutputFlagName,
	)
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		if strings.Contains(err.Error(), fmt.Sprintf("--%s", cmdcommon.OutputFlagName)) ||
			strings.Contains(err.Error(), fmt.Sprintf("-%s", cmdcommon.OutputFlagShort)) {
			return fmt.Errorf("%s", outputFlagMsg)
		}
		return err
	})

	// Also add konnect as a subcommand for explicit usage
	cmd.AddCommand(konnectCmd)

	return cmd, nil
}
