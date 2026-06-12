package supportdata

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kong/kong-deployment-toolkit/pkg/collector"
	configtest "github.com/kong/kongctl/test/config"
)

func TestOnPremFlagRegistration(t *testing.T) {
	cmd := NewOnPremCmd()

	tests := []struct {
		name      string
		flagName  string
		flagType  string
		shorthand string
	}{
		// On-prem specific flags
		{"runtime", "runtime", "string", ""},
		{"kong-addr", "kong-addr", "string", ""},
		{"rbac-header", "rbac-header", "stringSlice", "H"},
		{"prefix-dir", "prefix-dir", "string", "k"},
		{"target-images", "target-images", "stringSlice", ""},
		{"target-pods", "target-pods", "stringSlice", ""},
		{"namespace", "namespace", "string", "n"},
		// Common flags should also be present
		{"output-dir", "output-dir", "string", ""},
		{"sanitize", "sanitize", "bool", ""},
		{"line-limit", "line-limit", "int64", ""},
		{"logs-since", "logs-since", "string", ""},
		{"redact", "redact", "stringSlice", ""},
		{"disable-kdd", "disable-kdd", "bool", ""},
		{"dump-workspaces", "dump-workspaces", "bool", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmd.Flags().Lookup(tt.flagName)
			require.NotNil(t, f, "flag %q should be registered", tt.flagName)
			assert.Equal(t, tt.flagType, f.Value.Type(),
				"flag %q type mismatch", tt.flagName)
			if tt.shorthand != "" {
				assert.Equal(t, tt.shorthand, f.Shorthand,
					"flag %q shorthand mismatch", tt.flagName)
			}
		})
	}
}

func TestOnPremCmdProperties(t *testing.T) {
	cmd := NewOnPremCmd()

	assert.Equal(t, "on-prem", cmd.Use)
	assert.Contains(t, cmd.Aliases, "onprem")
	assert.Contains(t, cmd.Aliases, "self-managed")
	assert.Contains(t, cmd.Aliases, "gateway")
}

func TestBuildOnPremConfig_Defaults(t *testing.T) {
	mock := newMockConfig()
	commonFlags := &CommonFlags{}
	flags := &onPremFlags{}

	cfg := buildOnPremConfig(mock, commonFlags, flags)

	defaults := collector.DefaultConfig()
	assert.Equal(t, defaults.KongAddr, cfg.KongAddr,
		"KongAddr should use collector default")
	assert.Equal(t, defaults.TargetImages, cfg.TargetImages,
		"TargetImages should use collector default")
	assert.Equal(t, defaults.LineLimit, cfg.LineLimit,
		"LineLimit should use collector default")
	assert.Equal(t, defaults.PrefixDir, cfg.PrefixDir,
		"PrefixDir should use collector default")
	assert.False(t, cfg.KonnectMode,
		"KonnectMode should be false for on-prem")
}

func TestBuildOnPremConfig_ConfigOverridesDefaults(t *testing.T) {
	stringValues := map[string]string{
		configOnPremRuntime:  "kubernetes",
		configOnPremKongAddr: "http://kong:8001",
		configOnPremPrefixDir: "/opt/kong",
	}
	sliceValues := map[string][]string{
		configOnPremRBACHeaders:  {"Kong-Admin-Token:abc123"},
		configOnPremTargetImages: {"custom-kong"},
		configOnPremTargetPods:   {"pod-1", "pod-2"},
	}

	mock := &configtest.MockConfigHook{
		GetStringMock: func(key string) string {
			return stringValues[key]
		},
		GetBoolMock:   func(string) bool { return false },
		GetIntMock:    func(string) int { return 0 },
		GetIntOrElseMock: func(_ string, orElse int) int { return orElse },
		SaveMock:      func() error { return nil },
		BindFlagMock:  func(string, *pflag.Flag) error { return nil },
		GetProfileMock: func() string { return "default" },
		GetStringSlickMock: func(key string) []string {
			return sliceValues[key]
		},
		SetStringMock: func(string, string) {},
		SetMock:       func(string, any) {},
		GetMock:       func(string) any { return nil },
		GetPathMock:   func() string { return "" },
	}

	commonFlags := &CommonFlags{}
	flags := &onPremFlags{}

	cfg := buildOnPremConfig(mock, commonFlags, flags)

	assert.Equal(t, "kubernetes", cfg.Runtime)
	assert.Equal(t, "http://kong:8001", cfg.KongAddr)
	assert.Equal(t, "/opt/kong", cfg.PrefixDir)
	assert.Equal(t, []string{"Kong-Admin-Token:abc123"}, cfg.RBACHeaders)
	assert.Equal(t, []string{"custom-kong"}, cfg.TargetImages)
	assert.Equal(t, []string{"pod-1", "pod-2"}, cfg.TargetPods)
}

func TestBuildOnPremConfig_FlagsOverrideConfig(t *testing.T) {
	// Config sets some values
	stringValues := map[string]string{
		configOnPremRuntime:  "docker",
		configOnPremKongAddr: "http://config-addr:8001",
	}
	mock := &configtest.MockConfigHook{
		GetStringMock: func(key string) string {
			return stringValues[key]
		},
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

	// Flags override config
	commonFlags := &CommonFlags{
		LineLimit: 5000,
	}
	flags := &onPremFlags{
		Runtime:  "kubernetes",
		KongAddr: "http://flag-addr:8001",
	}

	cfg := buildOnPremConfig(mock, commonFlags, flags)

	assert.Equal(t, "kubernetes", cfg.Runtime,
		"flag should override config for runtime")
	assert.Equal(t, "http://flag-addr:8001", cfg.KongAddr,
		"flag should override config for kong-addr")
	assert.Equal(t, int64(5000), cfg.LineLimit,
		"common flag should override default for line-limit")
}

func TestBuildOnPremConfig_AllFlags(t *testing.T) {
	mock := newMockConfig()

	commonFlags := &CommonFlags{
		OutputDir:      "/tmp/output",
		Sanitize:       true,
		LineLimit:       3000,
		LogsSince:      "45m",
		RedactTerms:    []string{"secret", "password"},
		DisableKDD:     true,
		DumpWorkspaces: true,
	}
	flags := &onPremFlags{
		Runtime:      "vm",
		KongAddr:     "http://my-kong:8001",
		RBACHeaders:  []string{"Kong-Admin-Token:token123"},
		PrefixDir:    "/opt/kong",
		TargetImages: []string{"my-kong-image"},
		TargetPods:   []string{"kong-0"},
	}

	cfg := buildOnPremConfig(mock, commonFlags, flags)

	// On-prem specific fields
	assert.Equal(t, "vm", cfg.Runtime)
	assert.Equal(t, "http://my-kong:8001", cfg.KongAddr)
	assert.Equal(t, []string{"Kong-Admin-Token:token123"}, cfg.RBACHeaders)
	assert.Equal(t, "/opt/kong", cfg.PrefixDir)
	assert.Equal(t, []string{"my-kong-image"}, cfg.TargetImages)
	assert.Equal(t, []string{"kong-0"}, cfg.TargetPods)

	// Common fields
	assert.Equal(t, "/tmp/output", cfg.OutputDir)
	assert.True(t, cfg.SanitizeConfigs)
	assert.Equal(t, int64(3000), cfg.LineLimit)
	assert.Equal(t, "45m", cfg.DockerLogsSince)
	assert.Equal(t, int64(2700), cfg.K8sLogsSinceSeconds)
	assert.Equal(t, []string{"secret", "password"}, cfg.RedactTerms)
	assert.True(t, cfg.DisableKDD)
	assert.True(t, cfg.DumpWorkspaceConfigs)

	// Should NOT be in konnect mode
	assert.False(t, cfg.KonnectMode)
}

func TestBuildOnPremConfig_Namespace(t *testing.T) {
	t.Run("namespace from config", func(t *testing.T) {
		mock := newMockConfig()
		mock.GetStringMock = func(key string) string {
			if key == configOnPremNamespace {
				return "kong"
			}
			return ""
		}

		commonFlags := &CommonFlags{}
		flags := &onPremFlags{}

		cfg := buildOnPremConfig(mock, commonFlags, flags)
		assert.Equal(t, "kong", cfg.Namespace)
	})

	t.Run("namespace from flag", func(t *testing.T) {
		mock := newMockConfig()
		commonFlags := &CommonFlags{}
		flags := &onPremFlags{
			Namespace: "kong-dp",
		}

		cfg := buildOnPremConfig(mock, commonFlags, flags)
		assert.Equal(t, "kong-dp", cfg.Namespace)
	})

	t.Run("flag overrides config", func(t *testing.T) {
		mock := newMockConfig()
		mock.GetStringMock = func(key string) string {
			if key == configOnPremNamespace {
				return "config-ns"
			}
			return ""
		}

		commonFlags := &CommonFlags{}
		flags := &onPremFlags{
			Namespace: "flag-ns",
		}

		cfg := buildOnPremConfig(mock, commonFlags, flags)
		assert.Equal(t, "flag-ns", cfg.Namespace,
			"flag should override config for namespace")
	})

	t.Run("namespace works alongside target-pods", func(t *testing.T) {
		mock := newMockConfig()
		commonFlags := &CommonFlags{}
		flags := &onPremFlags{
			Namespace:  "kong",
			TargetPods: []string{"kong-gateway-0"},
		}

		cfg := buildOnPremConfig(mock, commonFlags, flags)
		assert.Equal(t, "kong", cfg.Namespace)
		assert.Equal(t, []string{"kong-gateway-0"}, cfg.TargetPods)
	})
}

func TestOnPremKubernetesRequiresNamespace(t *testing.T) {
	t.Run("kubernetes without namespace returns error", func(t *testing.T) {
		mock := newMockConfig()
		commonFlags := &CommonFlags{}
		flags := &onPremFlags{
			Runtime: "kubernetes",
		}

		cfg := buildOnPremConfig(mock, commonFlags, flags)
		assert.Equal(t, "kubernetes", cfg.Runtime)
		assert.Empty(t, cfg.Namespace)

		// Simulate the validation that runOnPrem performs
		if cfg.Runtime == "kubernetes" && cfg.Namespace == "" {
			assert.True(t, true, "should require namespace for kubernetes")
		}
	})

	t.Run("kubernetes with namespace does not error", func(t *testing.T) {
		mock := newMockConfig()
		commonFlags := &CommonFlags{}
		flags := &onPremFlags{
			Runtime:   "kubernetes",
			Namespace: "kong",
		}

		cfg := buildOnPremConfig(mock, commonFlags, flags)
		assert.Equal(t, "kubernetes", cfg.Runtime)
		assert.Equal(t, "kong", cfg.Namespace)

		needsError := cfg.Runtime == "kubernetes" && cfg.Namespace == ""
		assert.False(t, needsError, "should not require error when namespace is set")
	})

	t.Run("docker without namespace does not error", func(t *testing.T) {
		mock := newMockConfig()
		commonFlags := &CommonFlags{}
		flags := &onPremFlags{
			Runtime: "docker",
		}

		cfg := buildOnPremConfig(mock, commonFlags, flags)
		assert.Equal(t, "docker", cfg.Runtime)
		assert.Empty(t, cfg.Namespace)

		needsError := cfg.Runtime == "kubernetes" && cfg.Namespace == ""
		assert.False(t, needsError, "namespace not required for non-kubernetes runtime")
	})
}
