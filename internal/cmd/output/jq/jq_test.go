package jq

import (
	"bytes"
	"testing"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

type stubConfig struct {
	values     map[string]string
	boolValues map[string]bool
}

func (s stubConfig) Save() error                           { return nil }
func (s stubConfig) GetString(key string) string           { return s.values[key] }
func (s stubConfig) GetBool(key string) bool               { return s.boolValues[key] }
func (s stubConfig) GetInt(string) int                     { return 0 }
func (s stubConfig) GetIntOrElse(_ string, orElse int) int { return orElse }
func (s stubConfig) GetStringSlice(string) []string        { return nil }
func (s stubConfig) SetString(string, string)              {}
func (s stubConfig) Set(string, any)                       {}
func (s stubConfig) Get(string) any                        { return nil }
func (s stubConfig) BindFlag(string, *pflag.Flag) error    { return nil }
func (s stubConfig) GetProfile() string                    { return "default" }
func (s stubConfig) GetPath() string                       { return "" }

func TestResolveSettingsDefaults(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	AddFlags(command.Flags())

	settings, err := ResolveSettings(command, nil)
	require.NoError(t, err)
	require.Equal(t, "", settings.Filter)
	require.Equal(t, cmdcommon.ColorModeAuto, settings.ColorMode)
	require.Equal(t, DefaultTheme, settings.Theme)
}

func TestResolveSettingsEmptyFilterDefaultsToIdentity(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	AddFlags(command.Flags())
	require.NoError(t, command.Flags().Set(FlagName, ""))

	settings, err := ResolveSettings(command, nil)
	require.NoError(t, err)
	require.Equal(t, ".", settings.Filter)
}

func TestResolveSettingsReadsRawOutputShortFlag(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	AddFlags(command.Flags())
	require.NoError(t, command.Flags().Parse([]string{"-r"}))

	settings, err := ResolveSettings(command, nil)
	require.NoError(t, err)
	require.True(t, settings.RawOutput)
}

func TestResolveSettingsReadsColorConfig(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	AddFlags(command.Flags())

	cfg := stubConfig{
		values: map[string]string{
			ColorEnabledConfigPath: "always",
			ColorThemeConfigPath:   "github",
		},
		boolValues: map[string]bool{
			RawOutputConfigPath: true,
		},
	}

	settings, err := ResolveSettings(command, cfg)
	require.NoError(t, err)
	require.Equal(t, cmdcommon.ColorModeAlways, settings.ColorMode)
	require.Equal(t, "github", settings.Theme)
	require.True(t, settings.RawOutput)
}

func TestResolveSettingsUsesDefaultExpressionFromConfigWhenFlagUnset(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	AddFlags(command.Flags())

	cfg := stubConfig{
		values: map[string]string{
			DefaultExpressionConfigPath: ".[].name",
			ColorEnabledConfigPath:      "auto",
		},
		boolValues: map[string]bool{},
	}

	settings, err := ResolveSettings(command, cfg)
	require.NoError(t, err)
	require.Equal(t, ".[].name", settings.Filter)
}

func TestResolveSettingsFlagOverridesDefaultExpressionConfig(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	AddFlags(command.Flags())
	require.NoError(t, command.Flags().Set(FlagName, ".foo"))

	cfg := stubConfig{
		values: map[string]string{
			DefaultExpressionConfigPath: ".bar",
			ColorEnabledConfigPath:      "auto",
		},
		boolValues: map[string]bool{},
	}

	settings, err := ResolveSettings(command, cfg)
	require.NoError(t, err)
	require.Equal(t, ".foo", settings.Filter)
}

func TestResolveSettingsExplicitEmptyFlagRemainsIdentity(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	AddFlags(command.Flags())
	require.NoError(t, command.Flags().Set(FlagName, ""))

	cfg := stubConfig{
		values: map[string]string{
			DefaultExpressionConfigPath: ".bar",
			ColorEnabledConfigPath:      "auto",
		},
		boolValues: map[string]bool{},
	}

	settings, err := ResolveSettings(command, cfg)
	require.NoError(t, err)
	require.Equal(t, ".", settings.Filter)
}

func TestResolveSettingsIgnoresDefaultExpressionWhenCommandDoesNotSupportJQ(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	cfg := stubConfig{
		values: map[string]string{
			DefaultExpressionConfigPath: ".[].name",
		},
	}

	settings, err := ResolveSettings(command, cfg)
	require.NoError(t, err)
	require.Equal(t, "", settings.Filter)
}

func TestValidateOutputFormatRejectsText(t *testing.T) {
	err := ValidateOutputFormat(cmdcommon.TEXT, Settings{Filter: "."})
	require.Error(t, err)
	require.Contains(t, err.Error(), "only supported")
}

func TestValidateOutputFormatRejectsRawOutputWithoutFilter(t *testing.T) {
	err := ValidateOutputFormat(cmdcommon.JSON, Settings{RawOutput: true})
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires")
}

func TestApplyToRawJSON(t *testing.T) {
	payload := map[string]any{"foo": map[string]any{"bar": 42}, "baz": "x"}
	settings := Settings{
		Filter:    ".foo.bar",
		ColorMode: cmdcommon.ColorModeNever,
		Theme:     DefaultTheme,
	}

	result, handled, err := ApplyToRaw(payload, cmdcommon.JSON, settings, &bytes.Buffer{})
	require.NoError(t, err)
	require.False(t, handled)
	require.Equal(t, float64(42), result)
}

func TestApplyToRawYAML(t *testing.T) {
	payload := map[string]any{"foo": map[string]any{"bar": "value"}}
	settings := Settings{
		Filter:    ".foo",
		ColorMode: cmdcommon.ColorModeNever,
		Theme:     DefaultTheme,
	}

	result, handled, err := ApplyToRaw(payload, cmdcommon.YAML, settings, &bytes.Buffer{})
	require.NoError(t, err)
	require.False(t, handled)
	require.Equal(t, map[string]any{"bar": "value"}, result)
}

func TestApplyToRawColorizedJSONWritesDirectly(t *testing.T) {
	payload := map[string]any{"foo": map[string]any{"bar": 1}}
	settings := Settings{
		Filter:    ".",
		ColorMode: cmdcommon.ColorModeAlways,
		Theme:     DefaultTheme,
	}

	buf := &bytes.Buffer{}
	result, handled, err := ApplyToRaw(payload, cmdcommon.JSON, settings, buf)
	require.NoError(t, err)
	require.True(t, handled)
	require.Nil(t, result)
	require.Contains(t, buf.String(), "\x1b[")
}

func TestApplyToRawRawOutputWritesUnquotedStrings(t *testing.T) {
	payload := []map[string]any{
		{"name": "example-api"},
		{"name": "other-api"},
	}
	settings := Settings{
		Filter:    ".[].name",
		RawOutput: true,
	}

	buf := &bytes.Buffer{}
	result, handled, err := ApplyToRaw(payload, cmdcommon.JSON, settings, buf)
	require.NoError(t, err)
	require.True(t, handled)
	require.Nil(t, result)
	require.Equal(t, "example-api\nother-api\n", buf.String())
}

func TestApplyRawFilterMixedValues(t *testing.T) {
	buf := &bytes.Buffer{}
	err := ApplyRawFilter(
		[]byte(`{"items":[{"name":"a","enabled":true},{"name":"b","enabled":false}]}`),
		".items[] | .name, .enabled",
		buf,
	)
	require.NoError(t, err)
	require.Equal(t, "a\ntrue\nb\nfalse\n", buf.String())
}

func TestApplyFilterRejectsInvalidExpression(t *testing.T) {
	_, err := ApplyFilter([]byte(`{"foo":1}`), ".foo[")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid jq expression")
}
