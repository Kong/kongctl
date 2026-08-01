package text

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestFlagUsageIncludesDefaults(t *testing.T) {
	cmd, _ := configuredCommand(t, nil)
	usage := cmd.PersistentFlags().FlagUsages()
	require.Contains(t, usage, "- Allowed    : [ compact|auto|wide ]\n")
	require.Contains(t, usage, "- Allowed    : [ compact|full ]\n")
	require.Equal(t, 2, strings.Count(usage, "- Default    : [ compact ]"))
}

func TestResolvePrecedenceAndDefaults(t *testing.T) {
	tests := []struct {
		name         string
		profile      map[string]any
		layoutFlag   string
		idFormatFlag string
		want         Settings
	}{
		{
			name: "defaults",
			want: Settings{Layout: LayoutCompact, IDFormat: IDFormatCompact},
		},
		{
			name: "profile values",
			profile: map[string]any{
				"text": map[string]any{"layout": "auto", "id-format": "full"},
			},
			want: Settings{Layout: LayoutAuto, IDFormat: IDFormatFull},
		},
		{
			name: "flags override profile",
			profile: map[string]any{
				"text": map[string]any{"layout": "auto", "id-format": "full"},
			},
			layoutFlag:   "wide",
			idFormatFlag: "compact",
			want:         Settings{Layout: LayoutWide, IDFormat: IDFormatCompact},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, cfg := configuredCommand(t, tt.profile)
			if tt.layoutFlag != "" {
				require.NoError(t, cmd.PersistentFlags().Set(LayoutFlagName, tt.layoutFlag))
			}
			if tt.idFormatFlag != "" {
				require.NoError(t, cmd.PersistentFlags().Set(IDFormatFlagName, tt.idFormatFlag))
			}

			got, err := Resolve(cmd, cfg, "text")
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestResolveRejectsInvalidTextSettings(t *testing.T) {
	cmd, cfg := configuredCommand(t, map[string]any{
		"text": map[string]any{"layout": "panoramic"},
	})

	_, err := Resolve(cmd, cfg, "text")
	require.EqualError(t, err, `invalid text.layout value "panoramic", must be one of [compact auto wide]`)
}

func TestResolveNonTextOutput(t *testing.T) {
	t.Run("profile settings are ignored", func(t *testing.T) {
		cmd, cfg := configuredCommand(t, map[string]any{
			"text": map[string]any{"layout": "panoramic", "id-format": "invalid"},
		})

		got, err := Resolve(cmd, cfg, "json")
		require.NoError(t, err)
		require.Equal(t, Settings{Layout: LayoutCompact, IDFormat: IDFormatCompact}, got)
	})

	t.Run("explicit flag is rejected", func(t *testing.T) {
		cmd, cfg := configuredCommand(t, nil)
		require.NoError(t, cmd.PersistentFlags().Set(LayoutFlagName, "wide"))

		_, err := Resolve(cmd, cfg, "yaml")
		require.EqualError(
			t,
			err,
			"--text-layout and --text-id-format are only supported with --output text",
		)
	})
}

func configuredCommand(t *testing.T, profile map[string]any) (*cobra.Command, config.Hook) {
	t.Helper()
	mainConfig := viper.New()
	mainConfig.Set("default", profile)
	cfg := config.BuildProfiledConfig("default", "config.yaml", mainConfig)
	cmd := &cobra.Command{Use: "kongctl"}
	AddFlags(cmd.PersistentFlags())
	require.NoError(t, BindFlags(cfg, cmd.PersistentFlags()))
	return cmd, cfg
}
