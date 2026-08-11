package extensions

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	jqoutput "github.com/kong/kongctl/internal/cmd/output/jq"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestRegisterCommandsAddsExtensionUnderOpenBuiltInRoot(t *testing.T) {
	root := testRootCommand()
	ext := mustExtension(t, `
schema_version: 1
publisher: kong
name: foo
runtime:
  command: kongctl-ext-foo
command_paths:
  - path:
      - name: get
      - name: foo
        aliases: [foos]
    summary: Get Foo resources
`)

	err := RegisterCommands(root, NewStore(t.TempDir()), []Extension{ext})

	require.NoError(t, err)
	getCmd, _, err := root.Find([]string{"get", "foos"})
	require.NoError(t, err)
	require.Equal(t, "foo", getCmd.Name())
	require.Contains(t, getCmd.Short, "[extension: kong/foo]")
}

func TestRegisterCommandsRejectsBuiltInResourceCollision(t *testing.T) {
	root := testRootCommand()
	ext := mustExtension(t, `
schema_version: 1
publisher: kong
name: foo
runtime:
  command: kongctl-ext-foo
command_paths:
  - path:
      - name: get
      - name: apis
`)

	err := RegisterCommands(root, NewStore(t.TempDir()), []Extension{ext})

	require.ErrorContains(t, err, "collides with existing command")
}

func TestRegisterCommandsAddsHiddenRecoveryStubForUnavailableExtension(t *testing.T) {
	root := testRootCommand()
	ext := mustExtension(t, `
schema_version: 1
publisher: kong
name: foo
runtime:
  command: kongctl-ext-foo
command_paths:
  - path:
      - name: get
      - name: foo
    summary: Get Foo resources
`)
	ext.Health = unhealthy(
		ExtensionHealthUnavailable,
		"linked_source_unavailable",
		"linked source is unavailable",
		"restore the source or uninstall the extension",
	)
	require.NoError(t, RegisterCommands(root, NewStore(t.TempDir()), []Extension{ext}))

	command, _, err := root.Find([]string{"get", "foo"})
	require.NoError(t, err)
	require.True(t, command.Hidden)

	root.SetArgs([]string{"get", "foo"})
	err = root.Execute()
	require.ErrorContains(t, err, `extension "kong/foo": linked source is unavailable`)
}

func TestUnavailableExtensionCachedHelpExplainsRecovery(t *testing.T) {
	root := testRootCommand()
	ext := mustExtension(t, `
schema_version: 1
publisher: kong
name: foo
runtime:
  command: kongctl-ext-foo
command_paths:
  - path:
      - name: get
      - name: foo
    summary: Get Foo resources
`)
	ext.Health = unhealthy(
		ExtensionHealthUnavailable,
		"linked_source_unavailable",
		"linked source is unavailable",
		"restore the source or uninstall the extension",
	)
	require.NoError(t, RegisterCommands(root, NewStore(t.TempDir()), []Extension{ext}))
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"get", "foo", "--help"})

	require.NoError(t, root.Execute())
	require.Contains(t, out.String(), "Status: unavailable")
	require.Contains(t, out.String(), "restore the source or uninstall the extension")
}

func TestRegisterInstalledCommandsIsolatesACommandCollision(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "extensions"))
	badSource := t.TempDir()
	badManifest := []byte(`schema_version: 1
publisher: kong
name: bad
runtime:
  command: kongctl-ext-bad
command_paths:
  - path:
      - name: get
      - name: bad
  - path:
      - name: get
      - name: apis
`)
	require.NoError(t, os.WriteFile(filepath.Join(badSource, ManifestFileName), badManifest, 0o600))
	badRuntime := filepath.Join(badSource, "kongctl-ext-bad")
	require.NoError(t, os.WriteFile(badRuntime, []byte("#!/bin/sh\n"), 0o600))
	require.NoError(t, os.Chmod(badRuntime, 0o700))
	_, err := store.LinkLocal(badSource, "dev", time.Unix(100, 0))
	require.NoError(t, err)

	goodSource := t.TempDir()
	writeManifest(t, goodSource, "good", openBuiltInRootGet, "good")
	goodRuntime := filepath.Join(goodSource, "kongctl-ext-good")
	require.NoError(t, os.WriteFile(goodRuntime, []byte("#!/bin/sh\n"), 0o600))
	require.NoError(t, os.Chmod(goodRuntime, 0o700))
	_, err = store.LinkLocal(goodSource, "dev", time.Unix(100, 0))
	require.NoError(t, err)

	root := testRootCommand()
	require.NoError(t, RegisterInstalledCommands(root, store))
	getCommand, _, err := root.Find([]string{"get"})
	require.NoError(t, err)
	require.Nil(t, findChildByName(getCommand, "bad"), "colliding extension must not be partially registered")
	require.NotNil(t, findChildByName(getCommand, "good"), "healthy extensions must still be registered")
	require.NotNil(t, findChildByName(getCommand, "apis"), "built-in command must remain authoritative")
}

func TestRegisterInstalledCommandsDisablesBothCrossExtensionCollisions(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "extensions"))
	for _, name := range []string{"one", "two"} {
		source := t.TempDir()
		writeManifest(t, source, name, openBuiltInRootGet, "shared")
		runtimePath := filepath.Join(source, "kongctl-ext-"+name)
		require.NoError(t, os.WriteFile(runtimePath, []byte("#!/bin/sh\n"), 0o600))
		require.NoError(t, os.Chmod(runtimePath, 0o700))
		_, err := store.LinkLocal(source, "dev", time.Unix(100, 0))
		require.NoError(t, err)
	}

	root := testRootCommand()
	require.NoError(t, RegisterInstalledCommands(root, store))
	getCommand, _, err := root.Find([]string{"get"})
	require.NoError(t, err)
	require.Nil(t, findChildByName(getCommand, "shared"))

	extensions, err := store.List()
	require.NoError(t, err)
	extensions = MarkCommandConflicts(root, extensions)
	require.Len(t, extensions, 2)
	for _, ext := range extensions {
		require.Equal(t, ExtensionHealthConflict, ext.Health.Status)
		require.Equal(t, "command_conflict", ext.Health.Diagnostics[0].Code)
	}
}

func TestSplitExtensionArgsConsumesHostFlagsBeforeTerminator(t *testing.T) {
	root := testRootCommand()
	root.PersistentFlags().StringP(cmdcommon.ProfileFlagName, cmdcommon.ProfileFlagShort, "default", "")
	root.PersistentFlags().StringP(cmdcommon.OutputFlagName, cmdcommon.OutputFlagShort, "text", "")
	getCmd, _, err := root.Find([]string{"get"})
	require.NoError(t, err)
	getCmd.PersistentFlags().String(konnectcommon.BaseURLFlagName, "", "")
	terminal := &cobra.Command{Use: "foo"}
	getCmd.AddCommand(terminal)
	cfg := newTestHook()

	split, err := SplitExtensionArgs(
		terminal,
		[]string{
			"--profile", "dev",
			"-ojson",
			"--base-url=https://example.test",
			"--limit", "10",
			"--", "--profile", "literal",
		},
		cfg,
	)

	require.NoError(t, err)
	require.Equal(t, "dev", split.ProfileOverride)
	require.Equal(t, "json", cfg.values[cmdcommon.OutputConfigPath])
	require.Equal(t, "https://example.test", cfg.values[konnectcommon.BaseURLConfigPath])
	require.Equal(t, []string{"--limit", "10", "--profile", "literal"}, split.Remaining)
}

func TestSplitExtensionArgsConsumesJQFlagsForCustomRoot(t *testing.T) {
	root := testRootCommand()
	root.PersistentFlags().StringP(cmdcommon.OutputFlagName, cmdcommon.OutputFlagShort, "text", "")
	ext := mustExtension(t, `
schema_version: 1
publisher: kong
name: debug
runtime:
  command: kongctl-ext-debug
command_paths:
  - path:
      - name: debug
    summary: Show debug information
`)
	require.NoError(t, RegisterCommands(root, NewStore(t.TempDir()), []Extension{ext}))
	debugCmd, _, err := root.Find([]string{"debug"})
	require.NoError(t, err)
	cfg := newTestHook()

	split, err := SplitExtensionArgs(
		debugCmd,
		[]string{
			"--output", "json",
			"--jq", ".id",
			"--jq-raw-output",
			"--jq-color", "never",
			"--jq-color-theme=github",
			"remaining",
		},
		cfg,
	)

	require.NoError(t, err)
	require.Equal(t, "json", cfg.values[cmdcommon.OutputConfigPath])
	require.Equal(t, ".id", cfg.values[jqoutput.DefaultExpressionConfigPath])
	require.Equal(t, true, cfg.values[jqoutput.RawOutputConfigPath])
	require.Equal(t, "never", cfg.values[jqoutput.ColorEnabledConfigPath])
	require.Equal(t, "github", cfg.values[jqoutput.ColorThemeConfigPath])
	require.Equal(t, []string{"remaining"}, split.Remaining)
}

func testRootCommand() *cobra.Command {
	root := &cobra.Command{Use: "kongctl"}
	getCmd := &cobra.Command{Use: "get"}
	getCmd.AddCommand(&cobra.Command{Use: "apis"})
	listCmd := &cobra.Command{Use: "list"}
	root.AddCommand(getCmd, listCmd)
	return root
}

func mustExtension(t *testing.T, manifestYAML string) Extension {
	t.Helper()
	manifest, err := ParseManifest([]byte(manifestYAML))
	require.NoError(t, err)
	id := ExtensionID(manifest.Publisher, manifest.Name)
	return Extension{
		ID:           id,
		InstallType:  InstallTypeLinked,
		Manifest:     manifest,
		CommandPaths: manifest.CommandPaths,
	}
}

type testHook struct {
	values map[string]any
}

func newTestHook() *testHook {
	return &testHook{values: map[string]any{}}
}

func (h *testHook) GetString(key string) string {
	value, _ := h.values[key].(string)
	return value
}

func (h *testHook) GetBool(key string) bool {
	value, _ := h.values[key].(bool)
	return value
}

func (h *testHook) GetInt(key string) int {
	value, _ := h.values[key].(int)
	return value
}

func (h *testHook) GetIntOrElse(key string, orElse int) int {
	value, ok := h.values[key].(int)
	if !ok {
		return orElse
	}
	return value
}

func (h *testHook) GetStringSlice(key string) []string {
	value, _ := h.values[key].([]string)
	return value
}

func (h *testHook) SetString(key string, value string) {
	h.values[key] = value
}

func (h *testHook) Set(key string, value any) {
	h.values[key] = value
}

func (h *testHook) Get(key string) any {
	return h.values[key]
}

func (h *testHook) BindFlag(string, *pflag.Flag) error {
	return nil
}

func (h *testHook) GetProfile() string {
	return "default"
}

func (h *testHook) GetPath() string {
	return "config.yaml"
}

func (h *testHook) InConfig(string) bool {
	return false
}

func TestPrintExtensionHelpShowsAllHostFlagsByDefault(t *testing.T) {
	var buf bytes.Buffer
	contribution := CommandPath{Usage: "kongctl get foo", Summary: "Get foo"}
	require.NoError(t, PrintExtensionHelp(&buf, "kong/foo", contribution))
	out := buf.String()
	expected := "\nHost Flags:\n" +
		"  -o, --output string\tOutput format: text, json, or yaml\n" +
		"  --jq string\tFilter JSON or YAML output using a jq expression\n" +
		"  -r, --jq-raw-output\tOutput string jq results without JSON quotes\n" +
		"  --jq-color string\tColor mode for jq output: auto, always, or never\n" +
		"  --jq-color-theme string\tColor theme for jq output\n" +
		"  -p, --profile string\tConfiguration profile to use\n" +
		"  --color-theme string\tkongctl color theme\n"
	require.Contains(t, out, expected)
}

func TestPrintExtensionHelpHidesHostFlagsSection(t *testing.T) {
	var buf bytes.Buffer
	contribution := CommandPath{
		Usage: "kongctl get foo", Summary: "Get foo",
		HostFlags: &HostFlags{Hidden: true},
	}
	require.NoError(t, PrintExtensionHelp(&buf, "kong/foo", contribution))
	require.NotContains(t, buf.String(), "Host Flags:")
}

func TestPrintExtensionHelpShowsOnlySelectedHostFlags(t *testing.T) {
	var buf bytes.Buffer
	contribution := CommandPath{
		Usage: "kongctl get foo", Summary: "Get foo",
		HostFlags: &HostFlags{Only: []string{cmdcommon.OutputFlagName, cmdcommon.ProfileFlagName}},
	}
	require.NoError(t, PrintExtensionHelp(&buf, "kong/foo", contribution))
	out := buf.String()
	require.Contains(t, out, "Host Flags:")
	require.Contains(t, out, "--output")
	require.Contains(t, out, "--profile")
	require.NotContains(t, out, "--jq")
	require.NotContains(t, out, "--color-theme")
}
