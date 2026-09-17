package patch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

const taggedPatchInput = `ai_gateway_models:
  - ref: model-gemini-3.1-flash-lite-preview
    ai_gateway: !ref lab-agentic
    targets:
      - name: gemini-3.1-flash-lite-preview
        config:
          input_cost: 0.075
file: !file ./nonexistent.txt
env: !env PATCH_REPRO_NONEXISTENT
secret: !secret
  source: !env PATCH_REPRO_NONEXISTENT
lookup: !lookup {name: example}
external: !external example
items: !custom [!ref example]
`

const customerPatchSelector = "$.ai_gateway_models[?(@.ref=='model-gemini-3.1-flash-lite-preview')]" +
	".targets[?(@.name=='gemini-3.1-flash-lite-preview')].config"

func patchTestField(t *testing.T, node *yaml.Node, key string) *yaml.Node {
	t.Helper()
	require.Equal(t, yaml.MappingNode, node.Kind)
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	t.Fatalf("missing field %q", key)
	return nil
}

func TestRunFilePatch_PreserveTags(t *testing.T) {
	for _, mode := range []string{"inline", "patch-file", "no-match", "no-op"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "input.yaml")
			output := filepath.Join(dir, "output.yaml")
			require.NoError(t, os.WriteFile(input, []byte(taggedPatchInput), 0o600))
			args := []string{input}
			selectors, values := []string{customerPatchSelector}, []string{"input_cost:0.25"}
			if mode == "no-match" {
				selectors = []string{"$.missing"}
			}
			if mode == "patch-file" || mode == "no-op" {
				patchFile := filepath.Join(dir, "patch.yaml")
				content := "patches:\n  - selectors:\n      - " + customerPatchSelector +
					"\n    values: {input_cost: 0.25}\n"
				if mode == "no-op" {
					content = "patches: []\n"
				}
				require.NoError(t, os.WriteFile(patchFile, []byte(content), 0o600))
				args = append(args, patchFile)
				selectors, values = nil, nil
			}
			require.NoError(t, runFilePatch(args, selectors, values, output, "yaml"))
			root, err := readPatchInput(output)
			require.NoError(t, err)
			for key, tag := range map[string]string{
				"file": "!file", "env": "!env", "secret": "!secret",
				"lookup": "!lookup", "external": "!external", "items": "!custom",
			} {
				assert.Equal(t, tag, patchTestField(t, root, key).Tag)
			}
			assert.Equal(t, "./nonexistent.txt", patchTestField(t, root, "file").Value)
			assert.Equal(t, "PATCH_REPRO_NONEXISTENT", patchTestField(t, root, "env").Value)
			source := patchTestField(t, patchTestField(t, root, "secret"), "source")
			assert.Equal(t, "!env", source.Tag)
			assert.Equal(t, "PATCH_REPRO_NONEXISTENT", source.Value)
			assert.Equal(t, "!ref", patchTestField(t, root, "items").Content[0].Tag)
			model := patchTestField(t, root, "ai_gateway_models").Content[0]
			assert.Equal(t, "!ref", patchTestField(t, model, "ai_gateway").Tag)
			assert.Equal(t, "lab-agentic", patchTestField(t, model, "ai_gateway").Value)
			target := patchTestField(t, model, "targets").Content[0]
			cost := patchTestField(t, patchTestField(t, target, "config"), "input_cost")
			want := "0.25"
			if mode == "no-match" || mode == "no-op" {
				want = "0.075"
			}
			assert.Equal(t, want, cost.Value)
		})
	}
}

func TestRunFilePatch_TagMutation(t *testing.T) {
	for _, useFile := range []bool{false, true} {
		t.Run(fmt.Sprint("patch-file=", useFile), func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "input.yaml")
			output := filepath.Join(dir, "output.yaml")
			require.NoError(t, os.WriteFile(input, []byte(taggedPatchInput), 0o600))
			for _, tc := range []struct{ selector, value, fileValue string }{
				{"$.secret", `source:"literal"`, "source: literal"},
				{"$", `file:"literal"`, "file: literal"},
				{"$", `lookup:{"name":"replacement"}`, "lookup: {name: replacement}"},
				{"$.items", `["new"]`, "[new]"},
			} {
				args := []string{input}
				selectors, values := []string{tc.selector}, []string{tc.value}
				if useFile {
					path := filepath.Join(dir, "patch.yaml")
					value := tc.fileValue
					if tc.selector != "$.items" {
						value = "{" + value + "}"
					}
					data := "patches:\n  - selectors: ['" + tc.selector + "']\n    values: " + value + "\n"
					require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
					args = append(args, path)
					selectors, values = nil, nil
				}
				require.NoError(t, runFilePatch(args, selectors, values, output, "yaml"))
				input = output
			}
			root, err := readPatchInput(output)
			require.NoError(t, err)
			secret := patchTestField(t, root, "secret")
			assert.Equal(t, "!secret", secret.Tag)
			assert.Equal(t, "!!str", patchTestField(t, secret, "source").Tag)
			assert.Equal(t, "!!str", patchTestField(t, root, "file").Tag)
			assert.Equal(t, "!!map", patchTestField(t, root, "lookup").Tag)
			items := patchTestField(t, root, "items")
			assert.Equal(t, "!custom", items.Tag)
			require.Len(t, items.Content, 2)
			assert.Equal(t, "!ref", items.Content[0].Tag)
			assert.Equal(t, "!!str", items.Content[1].Tag)
		})
	}
}

func TestRunFilePatch_TaggedJSON(t *testing.T) {
	for _, inputData := range []string{
		taggedPatchInput, "key: !custom value\n", "!custom {key: value}\n",
		"key: !<tag:example.com,2026:custom> value\n", "!custom key: value\n",
	} {
		t.Run(inputData, func(t *testing.T) {
			dir := t.TempDir()
			input, output := filepath.Join(dir, "input.yaml"), filepath.Join(dir, "output.json")
			require.NoError(t, os.WriteFile(input, []byte(inputData), 0o600))
			require.NoError(t, os.WriteFile(output, []byte("existing output"), 0o600))
			err := runFilePatch([]string{input}, []string{"$.missing"}, []string{"x:1"}, output, "json")
			require.ErrorContains(t, err, "cannot output JSON")
			data, err := os.ReadFile(output)
			require.NoError(t, err)
			assert.Equal(t, "existing output", string(data))
		})
	}
	dir := t.TempDir()
	input, output := filepath.Join(dir, "input.yaml"), filepath.Join(dir, "output.json")
	require.NoError(t, os.WriteFile(input, []byte("key: !ref example\n"), 0o600))
	require.NoError(t, runFilePatch([]string{input}, []string{"$"}, []string{`key:"literal"`}, output, "json"))
	data, err := os.ReadFile(output)
	require.NoError(t, err)
	assert.JSONEq(t, `{"key":"literal"}`, string(data))

	// An alias can still reference a tagged node after its anchor field is removed.
	require.NoError(t, os.WriteFile(input, []byte("key: &ref !ref example\nalias: *ref\n"), 0o600))
	err = runFilePatch([]string{input}, []string{"$"}, []string{"key:"}, output, "json")
	require.ErrorContains(t, err, "cannot output JSON")
}

func TestRunFilePatch_InvalidDocument(t *testing.T) {
	for _, data := range []string{"", "[]", "scalar", "key: [", "key: value\n---\nother: value\n"} {
		t.Run(data, func(t *testing.T) {
			dir := t.TempDir()
			input, output := filepath.Join(dir, "input.yaml"), filepath.Join(dir, "output.yaml")
			require.NoError(t, os.WriteFile(input, []byte(data), 0o600))
			err := runFilePatch([]string{input}, []string{"$"}, []string{"x:1"}, output, "yaml")
			require.ErrorContains(t, err, "failed to read input file")
			_, err = os.Stat(output)
			require.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

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
	var parsed map[string]any
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
