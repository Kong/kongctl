package plan

import (
	"errors"
	"fmt"
	"strings"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
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
		"Generate a declarative configuration execution plan")

	planLong = normalizers.LongDesc(i18n.T("root.verbs.plan.planLong",
		`Generate an execution plan showing what changes will be made to a set of declarative configurations.`))
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
		Example: konnectCmd.Example,
		Args:    verbs.NoPositionalArgs,
		// Use the konnect command's RunE directly for Konnect-first pattern
		RunE:              konnectCmd.RunE,
		PersistentPreRunE: verbs.KonnectFirstPreRunE(Verb, konnectCmd),
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
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		if strings.Contains(err.Error(), fmt.Sprintf("--%s", cmdcommon.OutputFlagName)) ||
			strings.Contains(err.Error(), fmt.Sprintf("-%s", cmdcommon.OutputFlagShort)) {
			return &cmdpkg.UsageError{Err: errors.New(outputFlagMsg)}
		}
		return err
	})

	// plan rejects --output itself (in runPlan) with an actionable message;
	// opt out of root validation so that message can surface instead of the
	// generic "invalid value" from the root validator.
	cmdcommon.SkipOutputFormatValidation(cmd)

	// Also add konnect as a subcommand for explicit usage
	cmd.AddCommand(konnectCmd)

	return cmd, nil
}
