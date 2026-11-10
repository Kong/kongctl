package lint

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lintpkg "github.com/kong/kongctl/internal/lint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestLintCmd_ExpandReadsCommandStdinWithCrossFileTemplate(t *testing.T) {
	templates, _, ruleset := writeExpandTestFiles(t, "")
	root := newTestRootWithLint(t)
	root.SetIn(strings.NewReader("portals:\n  - _extends: standard-portal\n"))
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"lint", "--expand", "-f", "-", "-f", templates, "-r", ruleset, "--output", "json"})
	require.NoError(t, root.Execute(), output.String())
	var result lintpkg.Output
	require.NoError(t, json.Unmarshal(output.Bytes(), &result))
	assert.Zero(t, result.TotalCount)
}

func TestLintCmd_ExpandStdinIncludesUseWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "auth.yaml"), []byte("true\n"), 0o600))
	ruleset := filepath.Join(dir, "rules.yaml")
	require.NoError(t, os.WriteFile(ruleset, []byte(expandTestRuleset), 0o600))
	root := newTestRootWithLint(t)
	root.SetIn(strings.NewReader("portals:\n  - authentication_enabled: !file ./auth.yaml\n"))
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"lint", "--expand", "-f", "-", "-r", ruleset})
	require.NoError(t, root.Execute(), output.String())
}

func TestLintCmd_ExpandOverrideFixesViolation(t *testing.T) {
	templates, team, ruleset := writeExpandTestFiles(t, "    authentication_enabled: true\n")
	require.NoError(t, os.WriteFile(templates, []byte(`
_templates:
  standard-portal:
    authentication_enabled: false
`), 0o600))
	out, err := runLintForTest(t, "--expand", "-f", templates, "-f", team, "-r", ruleset)
	require.NoError(t, err, out)
}

func TestLintCmd_ExpandYAMLMarksExpandedResults(t *testing.T) {
	templates, team, ruleset := writeExpandTestFiles(t, "    authentication_enabled: false\n")
	out, err := runLintForTest(t, "--expand", "-f", templates, "-f", team, "-r", ruleset, "--output", "yaml")
	require.Error(t, err)
	var result lintpkg.Output
	require.NoError(t, yaml.Unmarshal([]byte(out), &result), out)
	require.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].Expanded)
	assert.Equal(t, team, result.Results[0].File)
}

func TestLintCmd_ExpandRecursiveFragmentsReachRules(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "fragments"), 0o700))
	fragment := filepath.Join(dir, "fragments", "auth.yaml")
	require.NoError(t, os.WriteFile(fragment, []byte("false\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"),
		[]byte("portals:\n  - authentication_enabled: !file ./fragments/auth.yaml\n"), 0o600))
	ruleset := filepath.Join(t.TempDir(), "rules.yaml")
	require.NoError(t, os.WriteFile(ruleset, []byte(expandTestRuleset), 0o600))
	out, err := runLintForTest(t, "--expand", "-R", "-f", dir, "-r", ruleset, "--output", "json")
	require.Error(t, err, "the included false value must violate the portal rule")
	var result lintpkg.Output
	require.NoError(t, json.Unmarshal([]byte(out), &result), out)
	require.NotEmpty(t, result.Results)
	assert.Equal(t, filepath.Join(dir, "config.yaml"), result.Results[0].File)
	assert.Equal(t, "portal-auth-required", result.Results[0].RuleID)
	for _, finding := range result.Results {
		assert.True(t, finding.Expanded)
	}
	// A selected scalar also reaches the engine, retaining raw lint's behavior
	// (including any engine limitation), rather than aborting preprocessing.
	raw, err := lintpkg.Content([]byte("false\n"), []byte(expandTestRuleset), "error", false, fragment)
	require.NoError(t, err)
	require.Len(t, result.Results, 1+len(raw.Results))
	for i, finding := range raw.Results {
		assert.Equal(t, finding.RuleID, result.Results[i+1].RuleID)
		assert.Equal(t, fragment, result.Results[i+1].File)
	}
}

func TestLintCmd_ExpandKeepsInputsSeparate(t *testing.T) {
	dir := t.TempDir()
	for _, filename := range []string{"one.yaml", "two.yaml"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, filename),
			[]byte("apis:\n  - ref: duplicate\n    name: payments\n"), 0o600))
	}
	ruleset := filepath.Join(t.TempDir(), "rules.yaml")
	require.NoError(t, os.WriteFile(ruleset, []byte(`
rules:
  at-most-one-api-per-document:
    severity: error
    given: $.apis
    then:
      function: length
      functionOptions:
        max: 1
`), 0o600))
	out, err := runLintForTest(t, "--expand", "-f", dir, "-r", ruleset)
	require.NoError(t, err, out)
}

func TestLintCmd_ExpandChecksSelectedMappingFragmentSeparately(t *testing.T) {
	dir := t.TempDir()
	fragment := filepath.Join(dir, "fragment.yaml")
	config := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(fragment, []byte("authentication_enabled: false\n"), 0o600))
	require.NoError(t, os.WriteFile(config, []byte("portals:\n  - !file ./fragment.yaml\n"), 0o600))
	ruleset := filepath.Join(t.TempDir(), "rules.yaml")
	require.NoError(t, os.WriteFile(ruleset, []byte(`
rules:
  enable-auth:
    severity: error
    given: $..authentication_enabled
    then:
      function: truthy
`), 0o600))
	out, err := runLintForTest(t, "--expand", "-f", dir, "-r", ruleset, "--output", "json")
	require.Error(t, err)
	var result lintpkg.Output
	require.NoError(t, json.Unmarshal([]byte(out), &result), out)
	require.Len(t, result.Results, 2)
	assert.Equal(t, config, result.Results[0].File)
	assert.Equal(t, fragment, result.Results[1].File)
}
