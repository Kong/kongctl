package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

type stubConfig struct {
	profile string
	path    string
}

func (s stubConfig) Save() error                           { return nil }
func (s stubConfig) GetString(string) string               { return "" }
func (s stubConfig) GetBool(string) bool                   { return false }
func (s stubConfig) GetInt(string) int                     { return 0 }
func (s stubConfig) GetIntOrElse(_ string, orElse int) int { return orElse }
func (s stubConfig) GetStringSlice(string) []string        { return nil }
func (s stubConfig) SetString(string, string)              {}
func (s stubConfig) Set(string, any)                       {}
func (s stubConfig) Get(string) any                        { return nil }
func (s stubConfig) BindFlag(string, *pflag.Flag) error    { return nil }
func (s stubConfig) GetProfile() string                    { return s.profile }
func (s stubConfig) GetPath() string                       { return s.path }

func TestDeleteAccessTokenRemovesFile(t *testing.T) {
	dir := t.TempDir()

	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte{}, 0o600))

	profile := "default"
	tokenPath := filepath.Join(dir, fmt.Sprintf(".%s-konnect-token.json", profile))
	require.NoError(t, os.WriteFile(tokenPath, []byte(`{"token":"value"}`), 0o600))

	cfg := stubConfig{
		profile: profile,
		path:    configPath,
	}

	removed, err := DeleteAccessToken(cfg)
	require.NoError(t, err)
	require.True(t, removed)

	_, err = os.Stat(tokenPath)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestDeleteAccessTokenNoFile(t *testing.T) {
	dir := t.TempDir()

	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte{}, 0o600))

	cfg := stubConfig{
		profile: "default",
		path:    configPath,
	}

	removed, err := DeleteAccessToken(cfg)
	require.NoError(t, err)
	require.False(t, removed)
}
