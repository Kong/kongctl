package explain

import (
	"context"
	"encoding/json"
	"fmt"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	jqoutput "github.com/kong/kongctl/internal/cmd/output/jq"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

const (
	Verb             = verbs.Explain
	extendedFlagName = "extended"
)

var (
	explainUse = Verb.String()

	explainShort = i18n.T("root.verbs.explain.short", "Explain declarative resource types")

	explainLong = normalizers.LongDesc(i18n.T("root.verbs.explain.long",
		`Explain shows the declarative schema for a supported resource type or field path.

Use text output for human-readable field summaries. Use json or yaml output to
retrieve the same machine-readable schema document in different serializations.`))

	explainExamples = normalizers.Examples(i18n.T("root.verbs.explain.examples",
		fmt.Sprintf(`
		# Explain the declarative API resource
		%[1]s explain api
		# Explain a child resource in nested form
		%[1]s explain api.versions
		# Explain a specific field
		%[1]s explain api.publications.portal_id
		# Retrieve the machine-readable schema as JSON Schema
		%[1]s explain api --output json
		# Retrieve the same schema serialized as YAML
		%[1]s explain api --output yaml
		`, meta.CLIName)))
)

func NewExplainCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     explainUse + " <resource-path>",
		Short:   explainShort,
		Long:    explainLong,
		Example: explainExamples,
		Args:    cobra.ExactArgs(1),
		PersistentPreRunE: func(c *cobra.Command, _ []string) error {
			c.SetContext(context.WithValue(c.Context(), verbs.Verb, Verb))
			return bindExplainFlags(c)
		},
		RunE: runExplain,
	}

	jqoutput.AddFlags(cmd.PersistentFlags())
	cmd.Flags().Bool(extendedFlagName, false, "Include extended field details in text output")

	return cmd, nil
}

func runExplain(command *cobra.Command, args []string) error {
	helper := cmdpkg.BuildHelper(command, args)
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	extended, err := command.Flags().GetBool(extendedFlagName)
	if err != nil {
		return err
	}
	if extended && outType != cmdcommon.TEXT {
		return &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("--%s is only supported with --output text", extendedFlagName),
		}
	}

	jqSettings, err := jqoutput.ResolveSettings(command, cfg)
	if err != nil {
		return err
	}
	if !shouldResolveExplainJQ(command, outType) {
		jqSettings = jqoutput.Settings{}
	}
	if err := jqoutput.ValidateOutputFormat(outType, jqSettings); err != nil {
		return err
	}

	subject, err := resources.ResolveExplainSubject(args[0])
	if err != nil {
		return err
	}

	switch outType {
	case cmdcommon.TEXT:
		_, err = fmt.Fprintln(command.OutOrStdout(), resources.RenderExplainText(subject, extended))
		return err
	case cmdcommon.JSON:
		schema := resources.RenderExplainSchema(subject)
		payload, handled, err := jqoutput.ApplyToRaw(
			schema,
			outType,
			jqSettings,
			command.OutOrStdout(),
		)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
		encoder := json.NewEncoder(command.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	case cmdcommon.YAML:
		schema := resources.RenderExplainSchema(subject)
		payload, handled, err := jqoutput.ApplyToRaw(
			schema,
			outType,
			jqSettings,
			command.OutOrStdout(),
		)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
		data, err := yaml.Marshal(payload)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(command.OutOrStdout(), string(data))
		return err
	default:
		return fmt.Errorf("unsupported output format: %s", outType.String())
	}
}

func bindExplainFlags(command *cobra.Command) error {
	helper := cmdpkg.BuildHelper(command, nil)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	return jqoutput.BindFlags(cfg, command.Flags())
}

func shouldResolveExplainJQ(command *cobra.Command, outType cmdcommon.OutputFormat) bool {
	if command == nil {
		return false
	}
	flags := command.Flags()
	if flags == nil {
		return false
	}
	return flags.Changed(jqoutput.FlagName) || flags.Changed(jqoutput.RawOutputFlagName) ||
		outType == cmdcommon.JSON || outType == cmdcommon.YAML
}
