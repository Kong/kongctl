package scaffold

import (
	"context"
	"errors"
	"fmt"
	"strings"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Verb                     = verbs.Scaffold
	outputFlagUnsupportedMsg = "flags -o/--" + cmdcommon.OutputFlagName + " are not supported for the scaffold command"
)

var (
	scaffoldUse = Verb.String()

	scaffoldShort = i18n.T("root.verbs.scaffold.short", "Generate a YAML scaffold for a declarative resource")

	scaffoldLong = normalizers.LongDesc(i18n.T("root.verbs.scaffold.long",
		`Scaffold emits a commented YAML starter configuration for a supported
declarative resource path.

The output is intended to be edited and then used with declarative commands
such as apply or sync.`))

	scaffoldExamples = normalizers.Examples(i18n.T("root.verbs.scaffold.examples",
		fmt.Sprintf(`
		# Generate a starter API configuration
		%[1]s scaffold api
		# Generate a root-level child resource scaffold
		%[1]s scaffold api_version
		# Generate a nested child scaffold
		%[1]s scaffold api.versions
		# Generate an analytics dashboard scaffold with a starter tile
		%[1]s scaffold analytics.dashboards
		`, meta.CLIName)))
)

func NewScaffoldCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     scaffoldUse + " <resource-path>",
		Short:   scaffoldShort,
		Long:    scaffoldLong,
		Example: scaffoldExamples,
		Args:    cobra.MaximumNArgs(1),
		PersistentPreRunE: func(c *cobra.Command, _ []string) error {
			c.SetContext(context.WithValue(c.Context(), verbs.Verb, Verb))
			return nil
		},
		RunE: runScaffold,
	}

	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		if strings.Contains(err.Error(), "--"+cmdcommon.OutputFlagName) ||
			strings.Contains(err.Error(), "-"+cmdcommon.OutputFlagShort) {
			return &cmdpkg.UsageError{Err: errors.New(outputFlagUnsupportedMsg)}
		}
		return err
	})

	// scaffold rejects --output itself in RunE; opt out of root validation so
	// the actionable "not supported" message can surface instead of the
	// generic "invalid value" from the root validator.
	cmdcommon.SkipOutputFormatValidation(cmd)

	return cmd, nil
}

func runScaffold(command *cobra.Command, args []string) error {
	command.SilenceUsage = true

	if outputFlag := command.Flag(cmdcommon.OutputFlagName); outputFlag != nil && outputFlag.Changed {
		return &cmdpkg.UsageError{Err: errors.New(outputFlagUnsupportedMsg)}
	}

	if len(args) == 0 {
		return printAvailableResourcePaths(command)
	}

	subject, err := resources.ResolveExplainSubject(args[0])
	if err != nil {
		return err
	}

	scaffold, err := resources.RenderScaffoldYAML(subject)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(command.OutOrStdout(), scaffold)
	return err
}

// printAvailableResourcePaths lists the resource paths scaffold accepts, so a
// bare invocation is discoverable instead of an argument error.
func printAvailableResourcePaths(command *cobra.Command) error {
	out := command.OutOrStdout()
	if _, err := fmt.Fprintln(out, "Available resource paths:"); err != nil {
		return err
	}
	for _, path := range resources.ExplainResourcePaths() {
		if _, err := fmt.Fprintf(out, "  %s\n", path); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(
		out,
		"\nRun '%[1]s scaffold <resource-path>'. Child resources also accept nested paths,\n"+
			"for example '%[1]s scaffold api.versions'.\n",
		meta.CLIName,
	)
	return err
}
