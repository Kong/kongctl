package scaffold

import (
	"context"
	"fmt"
	"strings"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Verb = verbs.Scaffold
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
		`, meta.CLIName)))
)

func NewScaffoldCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     scaffoldUse + " <resource-path>",
		Short:   scaffoldShort,
		Long:    scaffoldLong,
		Example: scaffoldExamples,
		Args:    cobra.ExactArgs(1),
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			c.SetContext(context.WithValue(c.Context(), verbs.Verb, Verb))
			return nil
		},
		RunE: runScaffold,
	}

	outputFlagMsg := fmt.Sprintf(
		"flags -o/--%s are not supported for the scaffold command",
		cmdcommon.OutputFlagName,
	)
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		if strings.Contains(err.Error(), fmt.Sprintf("--%s", cmdcommon.OutputFlagName)) ||
			strings.Contains(err.Error(), fmt.Sprintf("-%s", cmdcommon.OutputFlagShort)) {
			return fmt.Errorf("%s", outputFlagMsg)
		}
		return err
	})

	return cmd, nil
}

func runScaffold(command *cobra.Command, args []string) error {
	command.SilenceUsage = true

	if outputFlag := command.Flag(cmdcommon.OutputFlagName); outputFlag != nil && outputFlag.Changed {
		return fmt.Errorf("flags -o/--%s are not supported for the scaffold command", cmdcommon.OutputFlagName)
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
