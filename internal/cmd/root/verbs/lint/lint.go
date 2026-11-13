package lint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/lint"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Verb = verbs.Lint

	rulesetFlagName             = "ruleset"
	rulesetFlagShort            = "r"
	filenameFlagName            = "filename"
	filenameFlagShort           = "f"
	recursiveFlagName           = "recursive"
	recursiveFlagShort          = "R"
	failSeverityFlagName        = "fail-severity"
	failSeverityFlagShort       = "F"
	displayOnlyFailuresFlagName = "display-only-failures"
	displayOnlyFailuresShort    = "D"
	expandFlagName              = "expand"
)

var (
	lintUse = Verb.String()

	lintShort = i18n.T("root.verbs.lint.lintShort",
		"Lint configuration files against a ruleset")

	lintLong = normalizers.LongDesc(i18n.T("root.verbs.lint.lintLong",
		`Validate configuration files against a linting ruleset.

Linting uses vacuum with Spectral-compatible YAML or JSON rulesets.
Compatibility is partial:
  - Custom functions (functions/functionsDir) and --functions are not supported.
  - By default, declarative tags (!file, !ref, !env) and templates
    (_templates, _extends) are not expanded before linting. Use --expand to
    lint declarative configuration after expansion. OpenAPI $ref resolution
    is separate and may access referenced content or the network.
  - --ruleset must name a local file. Local paths in extends resolve from
    the working directory, not the ruleset directory. URLs can be used
    inside extends.

With --expand, the inputs are read as one declarative configuration, as plan
and apply read them: local !file content is included and _templates/_extends
are expanded across all inputs. !env, !ref, !secret and lookup tags stay
unresolved, so no environment values or secrets are read. Defaults are not
applied and plan validation does not run. Each non-empty input is linted
separately, and line numbers refer to the expanded document.

For more information on Spectral rulesets, see:
  https://docs.stoplight.io/docs/spectral/

Input files are specified with -f/--filename and can be individual
files or directories. Use -R/--recursive to process directories
recursively.`))

	lintExamples = normalizers.Examples(i18n.T("root.verbs.lint.lintExamples",
		fmt.Sprintf(`  # Lint a single file
  %[1]s lint -f config.yaml -r ruleset.yaml

  # Lint all YAML files in a directory
  %[1]s lint -f ./configs/ -r ruleset.yaml

  # Lint recursively with JSON output
  %[1]s lint -f ./configs/ -R -r ruleset.yaml --output json

  # Only show errors (not warnings/info/hints)
  %[1]s lint -f config.yaml -r ruleset.yaml --fail-severity error -D

  # Fail on warnings and above
  %[1]s lint -f config.yaml -r ruleset.yaml --fail-severity warn

  # Lint declarative configuration after expanding templates and !file
  %[1]s lint -f ./templates/ -f ./team/ -r ruleset.yaml --expand

  # Read from stdin
  cat config.yaml | %[1]s lint -f - -r ruleset.yaml`, meta.CLIName)))
)

// NewLintCmd creates and returns the lint cobra command.
func NewLintCmd() (*cobra.Command, error) {
	lintCmd := &cobra.Command{
		Use:     lintUse,
		Short:   lintShort,
		Long:    lintLong,
		Example: lintExamples,
		Aliases: []string{"li"},
		PersistentPreRun: func(c *cobra.Command, _ []string) {
			c.SetContext(context.WithValue(c.Context(), verbs.Verb, Verb))
		},
		RunE: runLint,
	}

	lintCmd.Flags().StringSliceP(filenameFlagName, filenameFlagShort, []string{},
		"Input file(s) or directory to lint. Use '-' to read from stdin.")
	lintCmd.Flags().BoolP(recursiveFlagName, recursiveFlagShort, false,
		"Process the directory used in -f, --filename recursively")
	lintCmd.Flags().StringP(rulesetFlagName, rulesetFlagShort, "",
		"Local YAML or JSON ruleset for vacuum (partial Spectral compatibility; no custom functions). "+
			"Input tags/templates are not expanded unless --expand is set; "+
			"extends paths use the working directory (URLs allowed in extends).")
	lintCmd.Flags().StringP(failSeverityFlagName, failSeverityFlagShort, "error",
		"Results of this severity or above will trigger a failure exit code. "+
			"Allowed: [ error | warn | info | hint ]")
	lintCmd.Flags().BoolP(displayOnlyFailuresFlagName, displayOnlyFailuresShort, false,
		"Only output results with severity equal to or greater than --fail-severity")
	lintCmd.Flags().Bool(expandFlagName, false,
		"Lint declarative configuration after including !file content and expanding _templates/_extends "+
			"across all inputs (!env, !ref and !secret stay unresolved)")

	if err := lintCmd.MarkFlagRequired(rulesetFlagName); err != nil {
		return nil, fmt.Errorf("marking ruleset flag required: %w", err)
	}

	return lintCmd, nil
}

func runLint(command *cobra.Command, _ []string) error {
	flagErr := func(name string, err error) error {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("failed to parse --%s flag: %w", name, err),
		}
	}

	// Parse flags with error handling
	filenames, err := command.Flags().GetStringSlice(filenameFlagName)
	if err != nil {
		return flagErr(filenameFlagName, err)
	}
	recursive, err := command.Flags().GetBool(recursiveFlagName)
	if err != nil {
		return flagErr(recursiveFlagName, err)
	}
	rulesetPath, err := command.Flags().GetString(rulesetFlagName)
	if err != nil {
		return flagErr(rulesetFlagName, err)
	}
	outputFmt, err := cmd.BuildHelper(command, nil).GetOutputFormat()
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}
	outputFormat := outputFmt.String()
	failSeverity, err := command.Flags().GetString(failSeverityFlagName)
	if err != nil {
		return flagErr(failSeverityFlagName, err)
	}
	onlyFailures, err := command.Flags().GetBool(displayOnlyFailuresFlagName)
	if err != nil {
		return flagErr(displayOnlyFailuresFlagName, err)
	}
	expand, err := command.Flags().GetBool(expandFlagName)
	if err != nil {
		return flagErr(expandFlagName, err)
	}

	// Validate fail-severity value
	if !slices.Contains([]string{"error", "warn", "info", "hint"}, failSeverity) {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"invalid --fail-severity %q, must be one of: error, warn, info, hint",
				failSeverity,
			),
		}
	}

	if len(filenames) == 0 {
		return &cmd.ConfigurationError{
			Err: errors.New("at least one input file is required via -f/--filename"),
		}
	}

	// Read ruleset file
	rulesetBytes, err := os.ReadFile(rulesetPath)
	if err != nil {
		return cmd.PrepareExecutionError(
			fmt.Sprintf("failed to read ruleset file %q", rulesetPath),
			err, command,
		)
	}

	// Separate stdin from regular file paths
	hasStdin := false
	var regularPaths []string
	for _, f := range filenames {
		if f == "-" {
			hasStdin = true
		} else {
			regularPaths = append(regularPaths, f)
		}
	}

	// Determine output writer
	out := command.OutOrStdout()
	if s, ok := command.Context().Value(iostreams.StreamsKey).(*iostreams.IOStreams); ok && s != nil {
		out = s.Out
	}

	output := &lint.Output{Results: []lint.Result{}}

	if expand {
		output, err = lintExpanded(command, filenames, recursive, rulesetBytes, failSeverity, onlyFailures)
		if err != nil {
			return err
		}
	}

	// Handle stdin input
	if hasStdin && !expand {
		stdinData, err := lint.ReadFromStdin(command.InOrStdin())
		if err != nil {
			return cmd.PrepareExecutionError(
				"failed to read from stdin", err, command,
			)
		}

		stdinOutput, err := lint.Content(
			stdinData, rulesetBytes, failSeverity, onlyFailures, "<stdin>",
		)
		if err != nil {
			return cmd.PrepareExecutionError(
				"failed to lint stdin input", err, command,
			)
		}

		output.TotalCount += stdinOutput.TotalCount
		output.FailCount += stdinOutput.FailCount
		output.Results = append(output.Results, stdinOutput.Results...)
	}

	// Collect and lint regular files
	if len(regularPaths) > 0 && !expand {
		inputFiles, err := lint.CollectFiles(regularPaths, recursive)
		if err != nil {
			return cmd.PrepareExecutionError(
				"failed to resolve input files", err, command,
			)
		}

		if len(inputFiles) == 0 {
			return cmd.PrepareExecutionError(
				"no YAML files found in the specified path(s)",
				fmt.Errorf("no YAML files found"), command,
			)
		}

		fileOutput, err := lint.Files(
			inputFiles, rulesetBytes, failSeverity, onlyFailures,
		)
		if err != nil {
			return cmd.PrepareExecutionError(
				"linting failed", err, command,
			)
		}

		output.TotalCount += fileOutput.TotalCount
		output.FailCount += fileOutput.FailCount
		output.Results = append(output.Results, fileOutput.Results...)
	}

	// Format and write output
	if err := lint.FormatOutput(out, output, outputFormat); err != nil {
		return cmd.PrepareExecutionError(
			"failed to write output", err, command,
		)
	}

	// Return error if there are failures to trigger non-zero exit code
	if output.FailCount > 0 {
		return cmd.PrepareExecutionError(
			"linting errors detected",
			fmt.Errorf(
				"found %d linting violation(s) at or above %q severity",
				output.FailCount, failSeverity,
			),
			command,
		)
	}

	return nil
}

// lintExpanded expands the inputs as one declarative configuration and lints
// each document that is not empty after expansion.
func lintExpanded(
	command *cobra.Command,
	filenames []string,
	recursive bool,
	rulesetBytes []byte,
	failSeverity string,
	onlyFailures bool,
) (*lint.Output, error) {
	sources, err := loader.ParseSources(filenames)
	if err != nil {
		return nil, &cmd.ConfigurationError{Err: err}
	}
	for _, source := range sources {
		if source.Type == loader.SourceTypeURL {
			return nil, &cmd.ConfigurationError{
				Err: fmt.Errorf("--%s does not support URL inputs: %s", expandFlagName, source.Path),
			}
		}
	}

	documents, err := loader.New().WithStdin(command.InOrStdin()).ExpandSources(command.Context(), sources, recursive)
	if err != nil {
		return nil, cmd.PrepareExecutionError(
			"failed to expand declarative configuration", err, command,
		)
	}

	inputs := make([]lint.Input, 0, len(documents))
	for _, document := range documents {
		if document.Empty {
			continue
		}
		inputs = append(inputs, lint.Input{
			Name: document.SourcePath, Content: document.Content, Expanded: true,
		})
	}
	if len(inputs) == 0 {
		return nil, cmd.PrepareExecutionError(
			"nothing to lint after expansion",
			errors.New("the inputs are empty or only define _templates; add configuration or files that extend them"),
			command,
		)
	}

	output, err := lint.Inputs(inputs, rulesetBytes, failSeverity, onlyFailures)
	if err != nil {
		return nil, cmd.PrepareExecutionError("linting failed", err, command)
	}
	return output, nil
}
