package lint

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
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
)

var (
	lintUse = Verb.String()

	lintShort = i18n.T("root.verbs.lint.lintShort",
		"Lint configuration files against a ruleset")

	lintLong = normalizers.LongDesc(i18n.T("root.verbs.lint.lintLong",
		`Validate configuration files against a linting ruleset.

The ruleset file must be a Spectral-compatible YAML or JSON ruleset.
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
		Aliases: []string{"l"},
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
		"Path to a Spectral-compatible linting ruleset file (YAML or JSON)")
	lintCmd.Flags().StringP(failSeverityFlagName, failSeverityFlagShort, "error",
		"Results of this severity or above will trigger a failure exit code. "+
			"Allowed: [ error | warn | info | hint ]")
	lintCmd.Flags().BoolP(displayOnlyFailuresFlagName, displayOnlyFailuresShort, false,
		"Only output results with severity equal to or greater than --fail-severity")

	if err := lintCmd.MarkFlagRequired(rulesetFlagName); err != nil {
		return nil, fmt.Errorf("marking ruleset flag required: %w", err)
	}

	return lintCmd, nil
}

func runLint(command *cobra.Command, _ []string) error {
	// Parse flags with error handling
	filenames, err := command.Flags().GetStringSlice(filenameFlagName)
	if err != nil {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("failed to parse --%s flag: %w", filenameFlagName, err),
		}
	}
	recursive, err := command.Flags().GetBool(recursiveFlagName)
	if err != nil {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("failed to parse --%s flag: %w", recursiveFlagName, err),
		}
	}
	rulesetPath, err := command.Flags().GetString(rulesetFlagName)
	if err != nil {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("failed to parse --%s flag: %w", rulesetFlagName, err),
		}
	}
	outputFmt, err := cmd.BuildHelper(command, nil).GetOutputFormat()
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}
	outputFormat := outputFmt.String()
	failSeverity, err := command.Flags().GetString(failSeverityFlagName)
	if err != nil {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("failed to parse --%s flag: %w", failSeverityFlagName, err),
		}
	}
	onlyFailures, err := command.Flags().GetBool(displayOnlyFailuresFlagName)
	if err != nil {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("failed to parse --%s flag: %w", displayOnlyFailuresFlagName, err),
		}
	}

	// Validate fail-severity value
	validSeverities := map[string]bool{
		"error": true, "warn": true, "info": true, "hint": true,
	}
	if !validSeverities[failSeverity] {
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

	// Handle stdin input
	if hasStdin {
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
	if len(regularPaths) > 0 {
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
