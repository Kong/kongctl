package sdk

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadRuntimeContextFromEnv(t *testing.T) {
	path := writeRuntimeContext(t, `{
  "schema_version": 1,
  "matched_command_path": {
    "id": "get_debug",
    "extension_id": "kong/debug",
    "path": ["get", "debug"]
  },
  "invocation": {
    "original_args": ["get", "debug", "--limit", "1"],
    "remaining_args": ["--limit", "1"]
  },
  "resolved": {
    "profile": "dev",
    "base_url": "https://example.test",
    "output": "json",
    "log_level": "debug",
    "color_theme": "kong-light",
    "config_file": "/tmp/kongctl/config.yaml",
    "extension_data_dir": "/tmp/kongctl/extensions/data/kong/debug",
    "auth_mode": "pat",
    "auth_source": "flag_or_config"
  },
  "output": {
    "format": "json",
    "color_theme": "kong-light",
    "jq": {
      "expression": ".id",
      "raw_output": true,
      "color": "never",
      "color_theme": "github"
    }
  },
  "host": {
    "kongctl_path": "/usr/local/bin/kongctl",
    "kongctl_version": "0.0.1"
  },
  "session": {
    "id": "abc123",
    "contribution_stack": ["get_debug"],
    "depth": 1,
    "max_depth": 5
  },
  "future_field": true
}`)
	t.Setenv(ContextEnvName, path)

	runtimeCtx, err := LoadRuntimeContextFromEnv()

	require.NoError(t, err)
	require.Equal(t, "kong/debug", runtimeCtx.MatchedCommandPath.ExtensionID)
	require.Equal(t, []string{"--limit", "1"}, runtimeCtx.Args())
	require.Equal(t, "/tmp/kongctl/extensions/data/kong/debug", runtimeCtx.DataDir())
	require.Equal(t, "/usr/local/bin/kongctl", runtimeCtx.KongctlPath())
	require.Equal(t, ".id", runtimeCtx.OutputSettings.JQ.Expression)
}

func TestLoadRuntimeContextFromEnvRequiresPath(t *testing.T) {
	t.Setenv(ContextEnvName, "")

	runtimeCtx, err := LoadRuntimeContextFromEnv()

	require.Nil(t, runtimeCtx)
	require.ErrorContains(t, err, ContextEnvName+" is not set")
}

func writeRuntimeContext(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "context.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}
