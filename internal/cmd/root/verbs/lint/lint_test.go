package lint

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/meta"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRootWithLint creates a minimal root command that provides the
// --output persistent flag and injects a config into the context so that
// BuildHelper.GetOutputFormat() works. Tests should call Execute on the
// returned root command, prefixing args with "lint".
func newTestRootWithLint(t *testing.T) *cobra.Command {
	t.Helper()

	// Enable hook traversal so both root and child PersistentPreRun execute.
	cobra.EnableTraverseRunHooks = true

	lintCmd, err := NewLintCmd()
	require.NoError(t, err)

	cfg := config.BuildProfiledConfig("default", "", viper.New())

	root := &cobra.Command{
		Use:              "kongctl",
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
		PersistentPreRun: func(c *cobra.Command, _ []string) {
			// Bind the output flag to config so GetOutputFormat() picks
			// up flag values (e.g., --output json).
			if f := c.Flags().Lookup(common.OutputFlagName); f != nil {
				_ = cfg.BindFlag(common.OutputConfigPath, f)
			}
			c.SetContext(context.WithValue(c.Context(), config.ConfigKey, cfg))
		},
	}
	root.PersistentFlags().StringP(
		common.OutputFlagName, common.OutputFlagShort,
		common.DefaultOutputFormat, "Output format (text|json|yaml)",
	)
	root.AddCommand(lintCmd)

	return root
}

func TestNewLintCmd(t *testing.T) {
	cmd, err := NewLintCmd()
	require.NoError(t, err)
	require.NotNil(t, cmd)

	assert.Equal(t, "lint", cmd.Use)
	assert.Contains(t, cmd.Short, "Lint")
	assert.Contains(t, cmd.Long, "Spectral")
	assert.Contains(t, cmd.Example, meta.CLIName)
}

func TestLintCmd_Flags(t *testing.T) {
	cmd, err := NewLintCmd()
	require.NoError(t, err)

	// Check required flags exist
	rulesetFlag := cmd.Flags().Lookup(rulesetFlagName)
	assert.NotNil(t, rulesetFlag, "should have --ruleset flag")

	filenameFlag := cmd.Flags().Lookup(filenameFlagName)
	assert.NotNil(t, filenameFlag, "should have --filename flag")

	recursiveFlag := cmd.Flags().Lookup(recursiveFlagName)
	assert.NotNil(t, recursiveFlag, "should have --recursive flag")
	assert.Equal(t, "false", recursiveFlag.DefValue)

	failSevFlag := cmd.Flags().Lookup(failSeverityFlagName)
	assert.NotNil(t, failSevFlag, "should have --fail-severity flag")
	assert.Equal(t, "error", failSevFlag.DefValue)

	displayFlag := cmd.Flags().Lookup(displayOnlyFailuresFlagName)
	assert.NotNil(t, displayFlag, "should have --display-only-failures flag")
	assert.Equal(t, "false", displayFlag.DefValue)
}

func TestLintCmd_FlagShortcuts(t *testing.T) {
	cmd, err := NewLintCmd()
	require.NoError(t, err)

	assert.NotNil(t, cmd.Flags().ShorthandLookup("r"), "should have -r shortcut")
	assert.NotNil(t, cmd.Flags().ShorthandLookup("f"), "should have -f shortcut")
	assert.NotNil(t, cmd.Flags().ShorthandLookup("R"), "should have -R shortcut")
	assert.NotNil(t, cmd.Flags().ShorthandLookup("F"), "should have -F shortcut")
	assert.NotNil(t, cmd.Flags().ShorthandLookup("D"), "should have -D shortcut")
}

func TestLintCmd_Aliases(t *testing.T) {
	cmd, err := NewLintCmd()
	require.NoError(t, err)

	assert.Contains(t, cmd.Aliases, "li")
}

func TestLintCmd_RequiresRuleset(t *testing.T) {
	root := newTestRootWithLint(t)

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetContext(context.Background())
	root.SetArgs([]string{"lint", "-f", "config.yaml"})

	err := root.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
}

func TestLintCmd_RequiresFilename(t *testing.T) {
	tmpDir := t.TempDir()
	rulesetFile := filepath.Join(tmpDir, "ruleset.yaml")
	err := os.WriteFile(rulesetFile, []byte(`
rules:
  test:
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
`), 0o600)
	require.NoError(t, err)

	root := newTestRootWithLint(t)

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetContext(context.Background())
	root.SetArgs([]string{"lint", "-r", rulesetFile})

	err = root.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one input file")
}

func TestLintCmd_LintFileWithViolations(t *testing.T) {
	tmpDir := t.TempDir()

	inputFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(inputFile, []byte(`host: example.com`), 0o600)
	require.NoError(t, err)

	rulesetFile := filepath.Join(tmpDir, "ruleset.yaml")
	err = os.WriteFile(rulesetFile, []byte(`
rules:
  must-have-name:
    description: "name is required"
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
`), 0o600)
	require.NoError(t, err)

	root := newTestRootWithLint(t)

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetContext(context.Background())
	root.SetArgs([]string{"lint", "-f", inputFile, "-r", rulesetFile})

	err = root.Execute()
	assert.Error(t, err, "should fail when violations exist")
	assert.Contains(t, output.String(), "Linting Violations")
}

func TestLintCmd_LintFileNoViolations(t *testing.T) {
	tmpDir := t.TempDir()

	inputFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(inputFile, []byte(`name: my-service`), 0o600)
	require.NoError(t, err)

	rulesetFile := filepath.Join(tmpDir, "ruleset.yaml")
	err = os.WriteFile(rulesetFile, []byte(`
rules:
  must-have-name:
    description: "name is required"
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
`), 0o600)
	require.NoError(t, err)

	root := newTestRootWithLint(t)

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetContext(context.Background())
	root.SetArgs([]string{"lint", "-f", inputFile, "-r", rulesetFile})

	err = root.Execute()
	assert.NoError(t, err, "should succeed with no violations")
}

func TestLintCmd_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()

	inputFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(inputFile, []byte(`host: example.com`), 0o600)
	require.NoError(t, err)

	rulesetFile := filepath.Join(tmpDir, "ruleset.yaml")
	err = os.WriteFile(rulesetFile, []byte(`
rules:
  must-have-name:
    description: "name is required"
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
`), 0o600)
	require.NoError(t, err)

	root := newTestRootWithLint(t)

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetContext(context.Background())
	root.SetArgs([]string{"lint", "-f", inputFile, "-r", rulesetFile, "--output", "json"})

	err = root.Execute()
	require.Error(t, err, "should fail when violations exist")
	assert.Contains(t, err.Error(), "linting violation")
	assert.Contains(t, output.String(), `"total_count"`)
	assert.Contains(t, output.String(), `"results"`)
}

func TestLintCmd_YAMLOutput(t *testing.T) {
	tmpDir := t.TempDir()

	inputFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(inputFile, []byte(`host: example.com`), 0o600)
	require.NoError(t, err)

	rulesetFile := filepath.Join(tmpDir, "ruleset.yaml")
	err = os.WriteFile(rulesetFile, []byte(`
rules:
  must-have-name:
    description: "name is required"
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
`), 0o600)
	require.NoError(t, err)

	root := newTestRootWithLint(t)

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetContext(context.Background())
	root.SetArgs([]string{"lint", "-f", inputFile, "-r", rulesetFile, "--output", "yaml"})

	err = root.Execute()
	require.Error(t, err, "should fail when violations exist")
	assert.Contains(t, err.Error(), "linting violation")
	assert.Contains(t, output.String(), "total_count:")
	assert.Contains(t, output.String(), "results:")
}

func TestLintCmd_InvalidFailSeverity(t *testing.T) {
	tmpDir := t.TempDir()

	inputFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(inputFile, []byte(`name: test`), 0o600)
	require.NoError(t, err)

	rulesetFile := filepath.Join(tmpDir, "ruleset.yaml")
	err = os.WriteFile(rulesetFile, []byte(`
rules:
  test:
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
`), 0o600)
	require.NoError(t, err)

	root := newTestRootWithLint(t)

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetContext(context.Background())
	root.SetArgs([]string{
		"lint", "-f", inputFile, "-r", rulesetFile, "--fail-severity", "critical",
	})

	err = root.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --fail-severity")
}

func TestLintCmd_NonExistentInputFile(t *testing.T) {
	tmpDir := t.TempDir()

	rulesetFile := filepath.Join(tmpDir, "ruleset.yaml")
	err := os.WriteFile(rulesetFile, []byte(`
rules:
  test:
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
`), 0o600)
	require.NoError(t, err)

	root := newTestRootWithLint(t)

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetContext(context.Background())
	root.SetArgs([]string{"lint", "-f", "/nonexistent/file.yaml", "-r", rulesetFile})

	err = root.Execute()
	assert.Error(t, err)
}

func TestLintCmd_NonExistentRulesetFile(t *testing.T) {
	tmpDir := t.TempDir()

	inputFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(inputFile, []byte(`name: test`), 0o600)
	require.NoError(t, err)

	root := newTestRootWithLint(t)

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetContext(context.Background())
	root.SetArgs([]string{"lint", "-f", inputFile, "-r", "/nonexistent/ruleset.yaml"})

	err = root.Execute()
	assert.Error(t, err)
}

func TestLintCmd_DirectoryInput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create input files in a subdirectory
	inputDir := filepath.Join(tmpDir, "configs")
	err := os.MkdirAll(inputDir, 0o755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(inputDir, "a.yaml"), []byte(`name: a`), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(inputDir, "b.yaml"), []byte(`name: b`), 0o600)
	require.NoError(t, err)

	rulesetFile := filepath.Join(tmpDir, "ruleset.yaml")
	err = os.WriteFile(rulesetFile, []byte(`
rules:
  must-have-name:
    description: "name is required"
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
`), 0o600)
	require.NoError(t, err)

	root := newTestRootWithLint(t)

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetContext(context.Background())
	root.SetArgs([]string{"lint", "-f", inputDir, "-r", rulesetFile})

	err = root.Execute()
	assert.NoError(t, err, "should pass when files have required name field")
}

func TestLintCmd_StdinInput(t *testing.T) {
	tmpDir := t.TempDir()

	rulesetFile := filepath.Join(tmpDir, "ruleset.yaml")
	err := os.WriteFile(rulesetFile, []byte(`
rules:
  must-have-name:
    description: "name is required"
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
`), 0o600)
	require.NoError(t, err)

	root := newTestRootWithLint(t)

	stdinData := bytes.NewReader([]byte(`host: example.com`))

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetIn(stdinData)
	root.SetContext(context.Background())
	root.SetArgs([]string{"lint", "-f", "-", "-r", rulesetFile})

	err = root.Execute()
	assert.Error(t, err, "should fail when stdin content has violations")
}

func TestLintCmd_FailSeverityWarn(t *testing.T) {
	tmpDir := t.TempDir()

	inputFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(inputFile, []byte(`host: example.com`), 0o600)
	require.NoError(t, err)

	rulesetFile := filepath.Join(tmpDir, "ruleset.yaml")
	err = os.WriteFile(rulesetFile, []byte(`
rules:
  should-have-name:
    description: "name is recommended"
    given: "$"
    severity: warn
    then:
      field: name
      function: truthy
`), 0o600)
	require.NoError(t, err)

	// With --fail-severity=error, warnings should NOT cause failure
	root1 := newTestRootWithLint(t)

	var output1 bytes.Buffer
	root1.SetOut(&output1)
	root1.SetErr(&output1)
	root1.SetContext(context.Background())
	root1.SetArgs([]string{
		"lint", "-f", inputFile, "-r", rulesetFile, "--fail-severity", "error",
	})

	err = root1.Execute()
	assert.NoError(t, err, "warnings should not cause failure when fail-severity is error")

	// With --fail-severity=warn, warnings SHOULD cause failure
	root2 := newTestRootWithLint(t)

	var output2 bytes.Buffer
	root2.SetOut(&output2)
	root2.SetErr(&output2)
	root2.SetContext(context.Background())
	root2.SetArgs([]string{
		"lint", "-f", inputFile, "-r", rulesetFile, "--fail-severity", "warn",
	})

	err = root2.Execute()
	assert.Error(t, err, "warnings should cause failure when fail-severity is warn")
}

func TestLintCmd_WithIOStreams(t *testing.T) {
	tmpDir := t.TempDir()

	inputFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(inputFile, []byte(`name: my-service`), 0o600)
	require.NoError(t, err)

	rulesetFile := filepath.Join(tmpDir, "ruleset.yaml")
	err = os.WriteFile(rulesetFile, []byte(`
rules:
  must-have-name:
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
`), 0o600)
	require.NoError(t, err)

	root := newTestRootWithLint(t)

	var outBuf, errBuf bytes.Buffer
	streams := &iostreams.IOStreams{
		Out:    &outBuf,
		ErrOut: &errBuf,
	}

	ctx := context.WithValue(context.Background(), iostreams.StreamsKey, streams)
	root.SetContext(ctx)
	root.SetArgs([]string{"lint", "-f", inputFile, "-r", rulesetFile})

	err = root.Execute()
	assert.NoError(t, err)
}

const (
	cmdMalformedSelectorRuleset = `
rules:
  must-have-name:
    description: "name is required"
    given: "$.apis["
    severity: error
    then:
      field: name
      function: truthy
`
	cmdZeroMatchRuleset = `
rules:
  missing-path:
    description: "this path does not exist"
    given: "$.does_not_exist"
    severity: error
    then:
      field: name
      function: truthy
`
	cmdValidNameRuleset = `
rules:
  must-have-name:
    description: "name is required"
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
`
	cmdSampleInput = `
name: test-service
host: example.com
`
)

func writeLintFiles(t *testing.T, input, ruleset string) (inputFile, rulesetFile string) {
	t.Helper()
	tmpDir := t.TempDir()
	inputFile = filepath.Join(tmpDir, "config.yaml")
	rulesetFile = filepath.Join(tmpDir, "ruleset.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte(input), 0o600))
	require.NoError(t, os.WriteFile(rulesetFile, []byte(ruleset), 0o600))
	return inputFile, rulesetFile
}

func executeLint(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newTestRootWithLint(t)
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetContext(context.Background())
	root.SetArgs(args)
	err := root.Execute()
	return output.String(), err
}

func TestLintCmd_MalformedSelector(t *testing.T) {
	inputFile, rulesetFile := writeLintFiles(t, cmdSampleInput, cmdMalformedSelectorRuleset)

	output, err := executeLint(t, "lint", "-f", inputFile, "-r", rulesetFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lint engine error")
	assert.Contains(t, err.Error(), "must-have-name")
	assert.Contains(t, err.Error(), "$.apis[")
	assert.Contains(t, err.Error(), inputFile)
	assert.NotContains(t, output, `"total_count"`)
	assert.NotContains(t, output, "Linting Violations")
}

func TestLintCmd_MalformedSelector_NotSuppressedByFlags(t *testing.T) {
	inputFile, rulesetFile := writeLintFiles(t, cmdSampleInput, cmdMalformedSelectorRuleset)

	output, err := executeLint(t,
		"lint", "-f", inputFile, "-r", rulesetFile,
		"--fail-severity", "error", "--display-only-failures",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lint engine error")
	assert.Contains(t, err.Error(), "must-have-name")
	assert.Contains(t, err.Error(), inputFile)
	assert.NotContains(t, output, `"total_count"`)
}

func TestLintCmd_MalformedSelector_JSONOutputStaysParseable(t *testing.T) {
	inputFile, rulesetFile := writeLintFiles(t, cmdSampleInput, cmdMalformedSelectorRuleset)

	output, err := executeLint(t, "lint", "-f", inputFile, "-r", rulesetFile, "--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lint engine error")
	assert.NotContains(t, output, `"total_count"`)
	assert.NotContains(t, output, `"results"`)
}

func TestLintCmd_MalformedSelector_Stdin(t *testing.T) {
	tmpDir := t.TempDir()
	rulesetFile := filepath.Join(tmpDir, "ruleset.yaml")
	require.NoError(t, os.WriteFile(rulesetFile, []byte(cmdMalformedSelectorRuleset), 0o600))

	root := newTestRootWithLint(t)
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetIn(bytes.NewReader([]byte(cmdSampleInput)))
	root.SetContext(context.Background())
	root.SetArgs([]string{"lint", "-f", "-", "-r", rulesetFile})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lint engine error")
	assert.Contains(t, err.Error(), "must-have-name")
	assert.Contains(t, err.Error(), "$.apis[")
	assert.Contains(t, err.Error(), "<stdin>")
	assert.NotContains(t, output.String(), `"total_count"`)
}

func TestLintCmd_MalformedSelector_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "configs")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))
	file1 := filepath.Join(inputDir, "a.yaml")
	require.NoError(t, os.WriteFile(file1, []byte(cmdSampleInput), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "b.yaml"), []byte(cmdSampleInput), 0o600))

	rulesetFile := filepath.Join(tmpDir, "ruleset.yaml")
	require.NoError(t, os.WriteFile(rulesetFile, []byte(cmdMalformedSelectorRuleset), 0o600))

	output, err := executeLint(t, "lint", "-f", inputDir, "-r", rulesetFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lint engine error")
	assert.Contains(t, err.Error(), "must-have-name")
	assert.Contains(t, err.Error(), "$.apis[")
	assert.Contains(t, err.Error(), file1)
	assert.NotContains(t, output, "Linting Violations")
}

func TestLintCmd_ValidSelectorZeroMatches(t *testing.T) {
	inputFile, rulesetFile := writeLintFiles(t, cmdSampleInput, cmdZeroMatchRuleset)

	output, err := executeLint(t, "lint", "-f", inputFile, "-r", rulesetFile)
	require.NoError(t, err, "valid selector with zero matches must succeed")
	assert.NotContains(t, output, "Linting Violations")
}

func TestLintCmd_ValidSelectorWithViolation(t *testing.T) {
	inputFile, rulesetFile := writeLintFiles(t, "host: example.com\n", cmdValidNameRuleset)

	output, err := executeLint(t, "lint", "-f", inputFile, "-r", rulesetFile)
	require.Error(t, err, "should fail when violations exist")
	assert.Contains(t, err.Error(), "linting violation")
	assert.Contains(t, output, "Linting Violations")
}
