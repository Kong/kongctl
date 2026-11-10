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

const expandTestRuleset = `
rules:
  portal-auth-required:
    description: "portals must enable authentication"
    given: "$.portals[*]"
    severity: error
    then:
      field: authentication_enabled
      function: truthy
`

func writeExpandTestFiles(t *testing.T, teamAuth string) (string, string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	templates := filepath.Join(tmpDir, "templates.yaml")
	require.NoError(t, os.WriteFile(templates, []byte(`
_templates:
  standard-portal:
    authentication_enabled: true
`), 0o600))
	team := filepath.Join(tmpDir, "team.yaml")
	require.NoError(t, os.WriteFile(team, []byte(`
portals:
  - ref: team-portal
    name: Team Portal
    _extends: standard-portal
`+teamAuth), 0o600))
	ruleset := filepath.Join(tmpDir, "ruleset.yaml")
	require.NoError(t, os.WriteFile(ruleset, []byte(expandTestRuleset), 0o600))
	return templates, team, ruleset
}

func runLintForTest(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newTestRootWithLint(t)
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetContext(t.Context())
	root.SetArgs(append([]string{"lint"}, args...))
	err := root.Execute()
	return output.String(), err
}

func TestLintCmd_WithoutExpandReportsTemplateValuesMissing(t *testing.T) {
	templates, team, ruleset := writeExpandTestFiles(t, "")

	out, err := runLintForTest(t, "-f", templates, "-f", team, "-r", ruleset)
	require.Error(t, err, "raw lint does not see template values")
	assert.Contains(t, out, team+":")
	assert.NotContains(t, out, "(expanded)")
}

func TestLintCmd_ExpandSeesTemplateValues(t *testing.T) {
	templates, team, ruleset := writeExpandTestFiles(t, "")

	out, err := runLintForTest(t, "-f", templates, "-f", team, "-r", ruleset, "--expand")
	require.NoError(t, err, out)
}

func TestLintCmd_ExpandReportsOverrideViolation(t *testing.T) {
	templates, team, ruleset := writeExpandTestFiles(t, "    authentication_enabled: false\n")

	out, err := runLintForTest(t, "-f", templates, "-f", team, "-r", ruleset, "--expand")
	require.Error(t, err)
	assert.Contains(t, out, team+":")
	assert.Contains(t, out, "(expanded)")
}

func TestLintCmd_ExpandJSONMarksExpandedResults(t *testing.T) {
	templates, team, ruleset := writeExpandTestFiles(t, "    authentication_enabled: false\n")

	out, err := runLintForTest(t, "-f", templates, "-f", team, "-r", ruleset, "--expand", "--output", "json")
	require.Error(t, err)
	assert.Contains(t, out, `"expanded": true`)
}

func TestLintCmd_ExpandTemplatesOnlyFails(t *testing.T) {
	templates, _, ruleset := writeExpandTestFiles(t, "")

	out, err := runLintForTest(t, "-f", templates, "-r", ruleset, "--expand")
	require.Error(t, err)
	assert.Contains(t, err.Error()+out, "only define _templates")
}

func TestLintCmd_ExpandUnknownTemplateFails(t *testing.T) {
	_, team, ruleset := writeExpandTestFiles(t, "")

	out, err := runLintForTest(t, "-f", team, "-r", ruleset, "--expand")
	require.Error(t, err)
	assert.Contains(t, err.Error()+out, `unknown template "standard-portal"`)
}

func TestLintCmd_ExpandRejectsURLInputs(t *testing.T) {
	_, _, ruleset := writeExpandTestFiles(t, "")

	_, err := runLintForTest(t, "-f", "https://example.com/config.yaml", "-r", ruleset, "--expand")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support URL inputs")
}

func TestLintCmd_ExpandReportsViolationFromFileFragment(t *testing.T) {
	_, team, ruleset := writeExpandTestFiles(t, "    authentication_enabled: !file ./files/auth.yaml\n")
	require.NoError(t, os.MkdirAll(filepath.Join(filepath.Dir(team), "files"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(team), "files", "auth.yaml"), []byte("false\n"), 0o600))

	out, err := runLintForTest(t, "-f", team, "-f", filepath.Join(filepath.Dir(team), "templates.yaml"),
		"-r", ruleset, "--expand")
	require.Error(t, err)
	assert.Contains(t, out, team+":")
	assert.Contains(t, out, "(expanded)")
}

func TestLintCmd_ExpandMissingFileFragmentFails(t *testing.T) {
	templates, team, ruleset := writeExpandTestFiles(t, "    description: !file ./files/missing.md\n")

	out, err := runLintForTest(t, "-f", templates, "-f", team, "-r", ruleset, "--expand")
	require.Error(t, err)
	assert.Contains(t, err.Error()+out, "missing.md")
}

func TestLintCmd_ExpandReportsEngineErrors(t *testing.T) {
	templates, team, _ := writeExpandTestFiles(t, "")
	ruleset := filepath.Join(filepath.Dir(team), "invalid-ruleset.yaml")
	require.NoError(t, os.WriteFile(ruleset, []byte(`
rules:
  broken-selector:
    severity: hint
    given: '$.portals['
    then:
      function: truthy
`), 0o600))

	_, err := runLintForTest(t, "-f", templates, "-f", team, "-r", ruleset, "--expand")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken-selector")
}
