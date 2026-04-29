package extensions

import (
	"context"
	"testing"

	"github.com/kong/kongctl/internal/build"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	jqoutput "github.com/kong/kongctl/internal/cmd/output/jq"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/stretchr/testify/require"
)

func TestHostEnvironmentPropagatesReentrantContext(t *testing.T) {
	cfg := newTestHook()

	env := hostEnvironment(cfg, "/tmp/context.json")

	require.Contains(t, env, "KONGCTL_EXTENSION_CONTEXT=/tmp/context.json")
	require.Contains(t, env, "KONGCTL_PROFILE=default")
}

func TestHostEnvironmentPropagatesPATForProfile(t *testing.T) {
	cfg := newTestHook()
	cfg.SetString(konnectcommon.PATConfigPath, "test-pat")

	env := hostEnvironment(cfg, "/tmp/context.json")

	require.Contains(t, env, "KONGCTL_EXTENSION_KONNECT_PAT=test-pat")
	require.Contains(t, env, "KONGCTL_DEFAULT_KONNECT_PAT=test-pat")
}

func TestProfileEnvNameNormalizesProfileAndConfigPath(t *testing.T) {
	require.Equal(t,
		"KONGCTL_TEAM_A_KONNECT_PAT",
		profileEnvName("team-a", konnectcommon.PATConfigPath),
	)
}

func TestBuildRuntimeContextIncludesOutputSettings(t *testing.T) {
	cfg := newTestHook()
	cfg.SetString(cmdcommon.OutputConfigPath, cmdcommon.JSON.String())
	cfg.SetString(cmdcommon.ColorThemeConfigPath, "kong-dark")
	cfg.SetString(jqoutput.DefaultExpressionConfigPath, ".id")
	cfg.Set(jqoutput.RawOutputConfigPath, true)
	cfg.SetString(jqoutput.ColorEnabledConfigPath, "never")
	cfg.SetString(jqoutput.ColorThemeConfigPath, "github")

	ext := mustExtension(t, `
schema_version: 1
publisher: kong
name: debug
runtime:
  command: kongctl-ext-debug
command_paths:
  - id: get_debug
    path:
      - name: get
      - name: debug
`)
	runtimeCtx, err := NewStore(t.TempDir()).buildRuntimeContext(
		cfg,
		nil,
		ext,
		ext.CommandPaths[0],
		[]string{"get", "debug"},
		[]string{"--limit", "1"},
		"",
		t.TempDir(),
		"abc123",
		[]string{"get_debug"},
		1,
	)

	require.NoError(t, err)
	require.Equal(t, cmdcommon.JSON.String(), runtimeCtx.Resolved.Output)
	require.Equal(t, "kong-dark", runtimeCtx.Resolved.ColorTheme)
	require.Equal(t, cmdcommon.JSON.String(), runtimeCtx.Output.Format)
	require.Equal(t, "kong-dark", runtimeCtx.Output.ColorTheme)
	require.Equal(t, ".id", runtimeCtx.Output.JQ.Expression)
	require.True(t, runtimeCtx.Output.JQ.RawOutput)
	require.Equal(t, "never", runtimeCtx.Output.JQ.Color)
	require.Equal(t, "github", runtimeCtx.Output.JQ.ColorTheme)
}

func TestDispatchRejectsIncompatibleExtensionBeforeRuntimeResolution(t *testing.T) {
	streams := iostreams.NewTestIOStreamsOnly()
	cfg := newTestHook()
	ext := mustExtension(t, `
schema_version: 1
publisher: kong
name: debug
compatibility:
  min_version: 9.0.0
runtime:
  command: missing-runtime
command_paths:
  - id: get_debug
    path:
      - name: get
      - name: debug
`)

	err := NewStore(t.TempDir()).Dispatch(
		context.Background(),
		streams,
		cfg,
		&build.Info{Version: "1.0.0"},
		ext,
		ext.CommandPaths[0],
		[]string{"get", "debug"},
		nil,
		"",
	)

	require.ErrorContains(t, err, "extension kong/debug is not compatible")
	require.NotContains(t, err.Error(), "missing-runtime")
}
