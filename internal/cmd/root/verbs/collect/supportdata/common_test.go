package supportdata

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kong/kong-deployment-toolkit/pkg/collector"
	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/iostreams"
	configtest "github.com/kong/kongctl/test/config"
)

// newMockConfig returns a MockConfigHook that returns zero values for
// everything. Callers override individual mocks as needed.
func newMockConfig() *configtest.MockConfigHook {
	return &configtest.MockConfigHook{
		GetStringMock:      func(string) string { return "" },
		GetBoolMock:        func(string) bool { return false },
		GetIntMock:         func(string) int { return 0 },
		GetIntOrElseMock:   func(_ string, orElse int) int { return orElse },
		BindFlagMock:       func(string, *pflag.Flag) error { return nil },
		GetProfileMock:     func() string { return "default" },
		GetStringSlickMock: func(string) []string { return nil },
		SetStringMock:      func(string, string) {},
		SetMock:            func(string, any) {},
		GetMock:            func(string) any { return nil },
		GetPathMock:        func() string { return "" },
	}
}

func TestCommonFlagRegistration(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	var flags CommonFlags
	RegisterCommonFlags(cmd, &flags)

	tests := []struct {
		name     string
		flagName string
		flagType string
	}{
		{"output-dir", "output-dir", "string"},
		{"sanitize", "sanitize", "bool"},
		{"line-limit", "line-limit", "int64"},
		{"logs-since", "logs-since", "string"},
		{"redact", "redact", "stringSlice"},
		{"disable-kdd", "disable-kdd", "bool"},
		{"dump-workspaces", "dump-workspaces", "bool"},
		{"tls-skip-verify", "tls-skip-verify", "bool"},
		{"ca-cert", "ca-cert", "string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmd.Flags().Lookup(tt.flagName)
			require.NotNil(t, f, "flag %q should be registered", tt.flagName)
			assert.Equal(t, tt.flagType, f.Value.Type(),
				"flag %q should have type %s", tt.flagName, tt.flagType)
		})
	}
}

func TestCommonFlagDefaults(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	var flags CommonFlags
	RegisterCommonFlags(cmd, &flags)

	assert.Equal(t, "", flags.OutputDir)
	assert.False(t, flags.Sanitize)
	assert.Equal(t, int64(0), flags.LineLimit)
	assert.Equal(t, "", flags.LogsSince)
	assert.Nil(t, flags.RedactTerms)
	assert.False(t, flags.DisableKDD)
	assert.False(t, flags.DumpWorkspaces)
	assert.False(t, flags.TLSSkipVerify)
	assert.Equal(t, "", flags.CACertPath)
}

func TestApplyCommonFlags_LogsSince(t *testing.T) {
	tests := []struct {
		name                    string
		logsSince               string
		wantDockerLogsSince     string
		wantK8sLogsSinceSeconds int64
	}{
		{
			name:                    "valid duration 1h",
			logsSince:               "1h",
			wantDockerLogsSince:     "1h",
			wantK8sLogsSinceSeconds: 3600,
		},
		{
			name:                    "valid duration 30m",
			logsSince:               "30m",
			wantDockerLogsSince:     "30m",
			wantK8sLogsSinceSeconds: 1800,
		},
		{
			name:                    "valid duration 5s",
			logsSince:               "5s",
			wantDockerLogsSince:     "5s",
			wantK8sLogsSinceSeconds: 5,
		},
		{
			name:                    "invalid duration preserved for docker",
			logsSince:               "yesterday",
			wantDockerLogsSince:     "yesterday",
			wantK8sLogsSinceSeconds: 0,
		},
		{
			name:                    "empty does not change config",
			logsSince:               "",
			wantDockerLogsSince:     "",
			wantK8sLogsSinceSeconds: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := collector.DefaultConfig()
			flags := &CommonFlags{LogsSince: tt.logsSince}
			ApplyCommonFlags(flags, cfg)

			assert.Equal(t, tt.wantDockerLogsSince, cfg.DockerLogsSince)
			assert.Equal(t, tt.wantK8sLogsSinceSeconds, cfg.K8sLogsSinceSeconds)
		})
	}
}

// An invalid --logs-since must not leave a Kubernetes window set by an earlier
// configuration layer in place, or the two runtimes silently diverge.
func TestApplyCommonFlags_LogsSinceDoesNotLeakAcrossLayers(t *testing.T) {
	cfg := collector.DefaultConfig()

	// Layer 1: config supplies a valid duration.
	mock := newMockConfig()
	mock.GetStringMock = func(key string) string {
		if key == configLogsSince {
			return "10m"
		}
		return ""
	}
	ApplyCommonConfig(mock, cfg)
	require.Equal(t, int64(600), cfg.K8sLogsSinceSeconds, "precondition")

	// Layer 2: a flag overrides it with a value Docker accepts but which is
	// not a Go duration, such as an RFC3339 timestamp.
	ApplyCommonFlags(&CommonFlags{LogsSince: "2026-07-01T00:00:00Z"}, cfg)

	assert.Equal(t, "2026-07-01T00:00:00Z", cfg.DockerLogsSince)
	assert.Equal(t, int64(0), cfg.K8sLogsSinceSeconds,
		"stale window from the config layer must not survive the flag override")
}

func TestApplyCommonFlags_AutoSanitize(t *testing.T) {
	t.Run("dump-workspaces enables sanitize", func(t *testing.T) {
		cfg := collector.DefaultConfig()
		flags := &CommonFlags{DumpWorkspaces: true}
		ApplyCommonFlags(flags, cfg)

		assert.True(t, cfg.DumpWorkspaceConfigs)
		assert.True(t, cfg.SanitizeConfigs,
			"SanitizeConfigs should auto-enable when DumpWorkspaceConfigs is true")
	})

	t.Run("explicit sanitize preserved with dump-workspaces", func(t *testing.T) {
		cfg := collector.DefaultConfig()
		flags := &CommonFlags{Sanitize: true, DumpWorkspaces: true}
		ApplyCommonFlags(flags, cfg)

		assert.True(t, cfg.SanitizeConfigs)
		assert.True(t, cfg.DumpWorkspaceConfigs)
	})

	t.Run("sanitize not forced when dump-workspaces false", func(t *testing.T) {
		cfg := collector.DefaultConfig()
		flags := &CommonFlags{DumpWorkspaces: false, Sanitize: false}
		ApplyCommonFlags(flags, cfg)

		assert.False(t, cfg.SanitizeConfigs)
	})
}

func TestApplyCommonFlags_AllFields(t *testing.T) {
	cfg := collector.DefaultConfig()
	flags := &CommonFlags{
		OutputDir:      "/tmp/support",
		Sanitize:       true,
		LineLimit:      5000,
		LogsSince:      "2h",
		RedactTerms:    []string{"password", "secret"},
		DisableKDD:     true,
		DumpWorkspaces: true,
	}
	ApplyCommonFlags(flags, cfg)

	assert.Equal(t, "/tmp/support", cfg.OutputDir)
	assert.True(t, cfg.SanitizeConfigs)
	assert.Equal(t, int64(5000), cfg.LineLimit)
	assert.Equal(t, "2h", cfg.DockerLogsSince)
	assert.Equal(t, int64(7200), cfg.K8sLogsSinceSeconds)
	assert.Equal(t, []string{"password", "secret"}, cfg.RedactTerms)
	assert.True(t, cfg.DisableKDD)
	assert.True(t, cfg.DumpWorkspaceConfigs)
}

func TestApplyCommonConfig(t *testing.T) {
	stringValues := map[string]string{
		configOutputDir: "/out",
		configLogsSince: "10m",
	}
	boolValues := map[string]bool{
		configSanitize:       true,
		configDisableKDD:     true,
		configDumpWorkspaces: true,
	}
	intValues := map[string]int{
		configLineLimit: 2000,
	}
	sliceValues := map[string][]string{
		configRedactTerms: {"token", "key"},
	}

	mock := newMockConfig()
	mock.GetStringMock = func(key string) string {
		return stringValues[key]
	}
	mock.GetBoolMock = func(key string) bool {
		return boolValues[key]
	}
	mock.GetIntMock = func(key string) int {
		return intValues[key]
	}
	mock.GetStringSlickMock = func(key string) []string {
		return sliceValues[key]
	}

	cfg := collector.DefaultConfig()
	ApplyCommonConfig(mock, cfg)

	assert.Equal(t, "/out", cfg.OutputDir)
	assert.True(t, cfg.SanitizeConfigs)
	assert.Equal(t, int64(2000), cfg.LineLimit)
	assert.Equal(t, "10m", cfg.DockerLogsSince)
	assert.Equal(t, int64(600), cfg.K8sLogsSinceSeconds)
	assert.Equal(t, []string{"token", "key"}, cfg.RedactTerms)
	assert.True(t, cfg.DisableKDD)
	assert.True(t, cfg.DumpWorkspaceConfigs)
}

func TestApplyCommonConfig_EmptyValues(t *testing.T) {
	mock := newMockConfig()
	defaults := collector.DefaultConfig()
	cfg := collector.DefaultConfig()
	ApplyCommonConfig(mock, cfg)

	// Defaults should be preserved when config returns zero values
	assert.Equal(t, defaults.KongAddr, cfg.KongAddr)
	assert.Equal(t, defaults.LineLimit, cfg.LineLimit)
	assert.Equal(t, defaults.PrefixDir, cfg.PrefixDir)
	assert.Equal(t, defaults.TargetImages, cfg.TargetImages)
}

// FormatOutput must write through the supplied IOStreams rather than the
// process-level stdio globals, so output can be captured and redirected.
func TestFormatOutput_WritesToStreams(t *testing.T) {
	result := &collector.Result{
		ArchivePath:    "/tmp/support.zip",
		Runtime:        runtimeDocker,
		CollectedFiles: []string{"a.log", "b.log"},
	}

	t.Run("text", func(t *testing.T) {
		var out, errOut bytes.Buffer
		streams := &iostreams.IOStreams{Out: &out, ErrOut: &errOut}

		require.NoError(t, FormatOutput(streams, common.TEXT, result))

		assert.Contains(t, out.String(), "/tmp/support.zip")
		assert.Contains(t, out.String(), runtimeDocker)
		assert.Contains(t, out.String(), "Files collected: 2")
		assert.Empty(t, errOut.String(), "no warnings means nothing on stderr")
	})

	t.Run("json", func(t *testing.T) {
		var out, errOut bytes.Buffer
		streams := &iostreams.IOStreams{Out: &out, ErrOut: &errOut}

		require.NoError(t, FormatOutput(streams, common.JSON, result))

		var decoded map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &decoded))
		assert.Equal(t, "/tmp/support.zip", decoded["archive_path"])
		assert.InDelta(t, 2.0, decoded["files_collected"], 0)
	})

	t.Run("yaml", func(t *testing.T) {
		var out, errOut bytes.Buffer
		streams := &iostreams.IOStreams{Out: &out, ErrOut: &errOut}

		require.NoError(t, FormatOutput(streams, common.YAML, result))

		assert.Contains(t, out.String(), "archive_path: /tmp/support.zip")
		assert.Contains(t, out.String(), "files_collected: 2")
	})

	t.Run("warnings go to stderr", func(t *testing.T) {
		var out, errOut bytes.Buffer
		streams := &iostreams.IOStreams{Out: &out, ErrOut: &errOut}

		withWarnings := *result
		withWarnings.Warnings = []error{errors.New("kdd unavailable")}

		require.NoError(t, FormatOutput(streams, common.TEXT, &withWarnings))

		assert.Contains(t, errOut.String(), "kdd unavailable")
		assert.NotContains(t, out.String(), "kdd unavailable",
			"warnings must not pollute the parseable stdout stream")
	})
}
