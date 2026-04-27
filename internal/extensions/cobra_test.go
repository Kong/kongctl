package extensions

import (
	"testing"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
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
