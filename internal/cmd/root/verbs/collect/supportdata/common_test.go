package supportdata

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kong/kong-deployment-toolkit/pkg/collector"
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
		SaveMock:           func() error { return nil },
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
		LineLimit:       5000,
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
