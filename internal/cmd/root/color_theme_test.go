package root

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/theme"
	"github.com/stretchr/testify/require"
)

func TestColorThemeCompatibility(t *testing.T) {
	previous := theme.CurrentName()
	configured := theme.IsConfiguredExplicitly()
	t.Cleanup(func() {
		require.NoError(t, theme.SetCurrent(previous))
		theme.SetConfiguredExplicitly(configured)
	})

	for _, source := range []string{"profile", "environment", "flag"} {
		for _, name := range []string{"coolnight", "kanagawa", "kanagwa"} {
			t.Run(source+"/"+name, func(t *testing.T) {
				t.Setenv("KONGCTL_WORKSPACE_COLOR_THEME", "")
				path := filepath.Join(t.TempDir(), "config.yaml")
				contents := "workspace:\n  color-theme: auto\n"
				args := []string{"get", "profiles", "--profile", "workspace", "--config-file", path}
				switch source {
				case "profile":
					contents = "workspace:\n  color-theme: " + name + "\n"
				case "environment":
					t.Setenv("KONGCTL_WORKSPACE_COLOR_THEME", name)
				case "flag":
					args = append(args, "--color-theme", name)
				}
				require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

				result := executeRootForTest(t, args...)
				command, _, err := rootCmd.Find([]string{"get", "profiles"})
				require.NoError(t, err)
				switch name {
				case "coolnight":
					require.Zero(t, result.exitCode, result.stderr)
					require.NotContains(t, result.stderr, "warning:")
					require.Equal(t, "cool_night", theme.CurrentName())
					require.True(t, theme.IsConfiguredExplicitly())
					require.Empty(t, theme.FallbackFromContext(command.Context()))
				case "kanagawa":
					require.Zero(t, result.exitCode, result.stderr)
					require.Contains(t, result.stderr, `warning: color theme "kanagawa" is no longer available`)
					require.Contains(t, result.stderr, `using "auto"`)
					require.Contains(t, result.stderr, "--color-theme <name>")
					require.Contains(t, result.stderr, "kongctl list themes --color-theme auto")
					require.NotContains(t, result.stdout, "warning:")
					require.Equal(t, theme.DetectDefaultName(), theme.CurrentName())
					require.Equal(t, theme.AutoName, currConfig.GetString(common.ColorThemeConfigPath))
					require.False(t, theme.IsConfiguredExplicitly())
					require.Equal(t, name, theme.FallbackFromContext(command.Context()))
				case "kanagwa":
					require.Equal(t, 1, result.exitCode)
					require.Contains(t, result.stderr, "kanagwa")
					require.Contains(t, result.stderr, "kongctl list themes --color-theme auto")
					require.NotContains(t, result.stderr, "warning:")
				}
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, contents, string(data), "theme selection must not rewrite configuration")
			})
		}
	}
}
