package patch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Command structure tests ---

func TestNewPatchCmd(t *testing.T) {
	cmd, err := NewPatchCmd()
	require.NoError(t, err)
	require.NotNil(t, cmd)

	assert.Equal(t, "patch", cmd.Use)
	assert.Contains(t, cmd.Short, "patches")
	assert.Contains(t, cmd.Example, meta.CLIName)
	assert.Equal(t, []string{"p"}, cmd.Aliases)
}

func TestPatchCmdVerb(t *testing.T) {
	assert.Equal(t, verbs.Patch, Verb)
	assert.Equal(t, "patch", Verb.String())
}

func TestNewPatchCmdHasFileSubcommand(t *testing.T) {
	patchCmd, err := NewPatchCmd()
	require.NoError(t, err)

	var fileCmd *bool
	for _, sub := range patchCmd.Commands() {
		if sub.Name() == "file" {
			found := true
			fileCmd = &found
		}
	}
	require.NotNil(t, fileCmd, "expected 'file' subcommand")
}

func TestFileCmdFlags(t *testing.T) {
	patchCmd, err := NewPatchCmd()
	require.NoError(t, err)

	var fileCmdFound bool
	for _, sub := range patchCmd.Commands() {
		if sub.Name() == "file" {
			fileCmdFound = true

			selectorFlag := sub.Flags().Lookup("selector")
			require.NotNil(t, selectorFlag)
			assert.Equal(t, "s", selectorFlag.Shorthand)

			valueFlag := sub.Flags().Lookup("value")
			require.NotNil(t, valueFlag)
			assert.Equal(t, "v", valueFlag.Shorthand)

			outputFileFlag := sub.Flags().Lookup("output-file")
			require.NotNil(t, outputFileFlag)
			assert.Equal(t, "-", outputFileFlag.DefValue)
			assert.Empty(t, outputFileFlag.Shorthand)

			formatFlag := sub.Flags().Lookup("format")
			require.NotNil(t, formatFlag)
			assert.Equal(t, "yaml", formatFlag.DefValue)
		}
	}
	require.True(t, fileCmdFound, "expected 'file' subcommand")
}

// --- parseOutputFormat tests ---

func TestParseOutputFormat(t *testing.T) {
	tests := []struct {
		name        string
		format      string
		expectError bool
	}{
		{name: "yaml", format: "yaml"},
		{name: "YAML uppercase", format: "YAML"},
		{name: "json", format: "json"},
		{name: "JSON uppercase", format: "JSON"},
		{name: "invalid format", format: "xml", expectError: true},
		{name: "empty string", format: "", expectError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseOutputFormat(tt.format)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unsupported output format")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// --- Validation tests ---

func TestRunFilePatch_MutualExclusivity(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte("key: value\n"), 0o600))

	patchFile := filepath.Join(tmpDir, "patch.yaml")
	require.NoError(t, os.WriteFile(patchFile, []byte("- selectors:\n  - $\n  values:\n    foo: bar\n"), 0o600))

	err := runFilePatch(
		[]string{inputFile, patchFile},
		[]string{"$"},
		[]string{"foo:\"bar\""},
		"-", "yaml",
	)
	require.Error(t, err)

	var cfgErr *cmd.ConfigurationError
	require.ErrorAs(t, err, &cfgErr)
	assert.Contains(t, cfgErr.Error(), "cannot combine")
}

func TestRunFilePatch_NoPatchesProvided(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte("key: value\n"), 0o600))

	err := runFilePatch(
		[]string{inputFile},
		nil,
		nil,
		"-", "yaml",
	)
	require.Error(t, err)

	var cfgErr *cmd.ConfigurationError
	require.ErrorAs(t, err, &cfgErr)
	assert.Contains(t, cfgErr.Error(), "provide either")
}

func TestRunFilePatch_InvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte("key: value\n"), 0o600))

	err := runFilePatch(
		[]string{inputFile},
		[]string{"$"},
		[]string{"foo:\"bar\""},
		"-", "xml",
	)
	require.Error(t, err)

	var cfgErr *cmd.ConfigurationError
	require.ErrorAs(t, err, &cfgErr)
	assert.Contains(t, cfgErr.Error(), "unsupported output format")
}

// --- Inline patch tests ---

func TestRunFilePatch_InlineSetValue(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte("services:\n  - name: svc1\n    port: 80\n"), 0o600))

	outputFile := filepath.Join(tmpDir, "output.yaml")
	err := runFilePatch(
		[]string{inputFile},
		[]string{"$..services[*]"},
		[]string{"read_timeout:30000"},
		outputFile, "yaml",
	)
	require.NoError(t, err)

	result, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	content := string(result)
	assert.Contains(t, content, "read_timeout")
	assert.Contains(t, content, "30000")
	assert.Contains(t, content, "name: svc1")
}

func TestRunFilePatch_InlineMultipleValues(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte("services:\n  - name: svc1\n"), 0o600))

	outputFile := filepath.Join(tmpDir, "output.yaml")
	err := runFilePatch(
		[]string{inputFile},
		[]string{"$..services[*]"},
		[]string{"read_timeout:30000", "write_timeout:60000"},
		outputFile, "yaml",
	)
	require.NoError(t, err)

	result, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	content := string(result)
	assert.Contains(t, content, "read_timeout")
	assert.Contains(t, content, "30000")
	assert.Contains(t, content, "write_timeout")
	assert.Contains(t, content, "60000")
}

func TestRunFilePatch_InlineRemoveKey(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte("name: test\ndebug: true\nport: 8080\n"), 0o600))

	outputFile := filepath.Join(tmpDir, "output.yaml")
	err := runFilePatch(
		[]string{inputFile},
		[]string{"$"},
		[]string{"debug:"},
		outputFile, "yaml",
	)
	require.NoError(t, err)

	result, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	content := string(result)
	assert.NotContains(t, content, "debug")
	assert.Contains(t, content, "name: test")
	assert.Contains(t, content, "port")
}

func TestRunFilePatch_InlineSetStringValue(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte("name: old\n"), 0o600))

	outputFile := filepath.Join(tmpDir, "output.yaml")
	err := runFilePatch(
		[]string{inputFile},
		[]string{"$"},
		[]string{`name:"new-name"`},
		outputFile, "yaml",
	)
	require.NoError(t, err)

	result, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	assert.Contains(t, string(result), "new-name")
}

// --- Patch file tests ---

func TestRunFilePatch_PatchFile(t *testing.T) {
	tmpDir := t.TempDir()

	inputFile := filepath.Join(tmpDir, "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte("services:\n  - name: svc1\n    port: 80\n"), 0o600))

	patchFileContent := `patches:
  - selectors:
    - $..services[*]
    values:
      read_timeout: 30000
`
	patchFile := filepath.Join(tmpDir, "patches.yaml")
	require.NoError(t, os.WriteFile(patchFile, []byte(patchFileContent), 0o600))

	outputFile := filepath.Join(tmpDir, "output.yaml")
	err := runFilePatch(
		[]string{inputFile, patchFile},
		nil, nil,
		outputFile, "yaml",
	)
	require.NoError(t, err)

	result, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	content := string(result)
	assert.Contains(t, content, "read_timeout")
	assert.Contains(t, content, "30000")
}

func TestRunFilePatch_PatchFileWithFormatVersion(t *testing.T) {
	tmpDir := t.TempDir()

	inputFile := filepath.Join(tmpDir, "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte("name: test\nport: 80\n"), 0o600))

	patchFileContent := `_format_version: "1.0"
patches:
  - selectors:
    - $
    values:
      env: production
`
	patchFile := filepath.Join(tmpDir, "patches.yaml")
	require.NoError(t, os.WriteFile(patchFile, []byte(patchFileContent), 0o600))

	outputFile := filepath.Join(tmpDir, "output.yaml")
	err := runFilePatch(
		[]string{inputFile, patchFile},
		nil, nil,
		outputFile, "yaml",
	)
	require.NoError(t, err)

	result, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	content := string(result)
	assert.Contains(t, content, "env")
	assert.Contains(t, content, "production")
}

func TestRunFilePatch_MultiplePatchFiles(t *testing.T) {
	tmpDir := t.TempDir()

	inputFile := filepath.Join(tmpDir, "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte("name: test\nport: 80\n"), 0o600))

	patch1Content := `patches:
  - selectors:
    - $
    values:
      env: staging
`
	patch1 := filepath.Join(tmpDir, "patch1.yaml")
	require.NoError(t, os.WriteFile(patch1, []byte(patch1Content), 0o600))

	patch2Content := `patches:
  - selectors:
    - $
    values:
      region: us-east-1
`
	patch2 := filepath.Join(tmpDir, "patch2.yaml")
	require.NoError(t, os.WriteFile(patch2, []byte(patch2Content), 0o600))

	outputFile := filepath.Join(tmpDir, "output.yaml")
	err := runFilePatch(
		[]string{inputFile, patch1, patch2},
		nil, nil,
		outputFile, "yaml",
	)
	require.NoError(t, err)

	result, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	content := string(result)
	assert.Contains(t, content, "env")
	assert.Contains(t, content, "staging")
	assert.Contains(t, content, "region")
	assert.Contains(t, content, "us-east-1")
}

// --- Output format tests ---

func TestRunFilePatch_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte("name: test\nport: 8080\n"), 0o600))

	outputFile := filepath.Join(tmpDir, "output.json")
	err := runFilePatch(
		[]string{inputFile},
		[]string{"$"},
		[]string{"env:\"dev\""},
		outputFile, "json",
	)
	require.NoError(t, err)

	result, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	// Verify it's valid JSON
	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(result, &parsed))
	assert.Equal(t, "test", parsed["name"])
	assert.Equal(t, "dev", parsed["env"])
}

func TestRunFilePatch_JSONInput(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.json")
	require.NoError(t, os.WriteFile(inputFile, []byte(`{"name":"test","port":80}`), 0o600))

	outputFile := filepath.Join(tmpDir, "output.yaml")
	err := runFilePatch(
		[]string{inputFile},
		[]string{"$"},
		[]string{"env:\"prod\""},
		outputFile, "yaml",
	)
	require.NoError(t, err)

	result, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	content := string(result)
	assert.Contains(t, content, "name: test")
	assert.Contains(t, content, "env: prod")
}

// --- Error case tests ---

func TestRunFilePatch_NonexistentInputFile(t *testing.T) {
	err := runFilePatch(
		[]string{"/nonexistent/file.yaml"},
		[]string{"$"},
		[]string{"foo:\"bar\""},
		"-", "yaml",
	)
	require.Error(t, err)

	var execErr *cmd.ExecutionError
	require.ErrorAs(t, err, &execErr)
	assert.Contains(t, execErr.Error(), "failed to read input file")
}

func TestRunFilePatch_NonexistentPatchFile(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte("key: value\n"), 0o600))

	err := runFilePatch(
		[]string{inputFile, "/nonexistent/patch.yaml"},
		nil, nil,
		"-", "yaml",
	)
	require.Error(t, err)

	var execErr *cmd.ExecutionError
	require.ErrorAs(t, err, &execErr)
	assert.Contains(t, execErr.Error(), "failed to parse patch file")
}

func TestRunFilePatch_InvalidPatchFileContent(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte("key: value\n"), 0o600))

	patchFile := filepath.Join(tmpDir, "bad-patch.yaml")
	require.NoError(t, os.WriteFile(patchFile, []byte("this is not a valid patch file: [[["), 0o600))

	err := runFilePatch(
		[]string{inputFile, patchFile},
		nil, nil,
		"-", "yaml",
	)
	require.Error(t, err)

	var execErr *cmd.ExecutionError
	require.ErrorAs(t, err, &execErr)
}

func TestRunFilePatch_InvalidValueFlag(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte("key: value\n"), 0o600))

	// A value without a colon separator should fail validation
	err := runFilePatch(
		[]string{inputFile},
		[]string{"$"},
		[]string{"invalid-no-colon"},
		"-", "yaml",
	)
	require.Error(t, err)

	var execErr *cmd.ExecutionError
	require.ErrorAs(t, err, &execErr)
}

// --- Stdout output test ---

func TestRunFilePatch_StdoutOutput(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte("name: test\n"), 0o600))

	// "-" means stdout; this should not error
	err := runFilePatch(
		[]string{inputFile},
		[]string{"$"},
		[]string{"added:true"},
		"-", "yaml",
	)
	require.NoError(t, err)
}
