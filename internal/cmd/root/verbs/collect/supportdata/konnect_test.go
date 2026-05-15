package supportdata

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kong/kong-deployment-toolkit/pkg/collector"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	configtest "github.com/kong/kongctl/test/config"
)

func TestKonnectFlagRegistration(t *testing.T) {
	cmd := NewKonnectCmd()

	tests := []struct {
		name     string
		flagName string
		flagType string
	}{
		// Konnect specific flags
		{"control-plane", "control-plane", "string"},
		{"pat", "pat", "string"},
		{"base-url", "base-url", "string"},
		{"region", "region", "string"},
		// Runtime flags (shared with on-prem)
		{"namespace", "namespace", "string"},
		{"runtime", "runtime", "string"},
		{"target-images", "target-images", "stringSlice"},
		{"target-pods", "target-pods", "stringSlice"},
		{"prefix-dir", "prefix-dir", "string"},
		// Common flags should also be present
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
				"flag %q type mismatch", tt.flagName)
		})
	}
}

func TestKonnectCmdProperties(t *testing.T) {
	cmd := NewKonnectCmd()
	assert.Equal(t, "konnect", cmd.Use)
}

func TestBuildKonnectConfig_KonnectModeEnabled(t *testing.T) {
	mock := newMockConfig()

	commonFlags := &CommonFlags{}
	flags := &konnectFlags{}

	cfg, err := buildKonnectConfig(mock, commonFlags, flags)
	require.NoError(t, err)

	assert.True(t, cfg.KonnectMode,
		"KonnectMode should always be true for konnect subcommand")
}

func TestBuildKonnectConfig_PATMapping(t *testing.T) {
	t.Run("PAT flag maps to RBACHeaders", func(t *testing.T) {
		mock := newMockConfig()

		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			PAT: "kpat_test123",
		}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		require.Len(t, cfg.RBACHeaders, 1,
			"PAT should be placed in RBACHeaders")
		assert.Equal(t, "kpat_test123", cfg.RBACHeaders[0])
	})

	t.Run("PAT from config maps to RBACHeaders", func(t *testing.T) {
		mock := newMockConfig()
		mock.GetStringMock = func(key string) string {
			if key == konnectcommon.PATConfigPath {
				return "kpat_config_token"
			}
			return ""
		}

		commonFlags := &CommonFlags{}
		flags := &konnectFlags{}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		require.Len(t, cfg.RBACHeaders, 1)
		assert.Equal(t, "kpat_config_token", cfg.RBACHeaders[0])
	})

	t.Run("PAT flag overrides PAT config", func(t *testing.T) {
		mock := newMockConfig()
		mock.GetStringMock = func(key string) string {
			if key == konnectcommon.PATConfigPath {
				return "kpat_from_config"
			}
			return ""
		}

		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			PAT: "kpat_from_flag",
		}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		require.Len(t, cfg.RBACHeaders, 1)
		assert.Equal(t, "kpat_from_flag", cfg.RBACHeaders[0],
			"flag PAT should override config PAT")
	})
}

func TestBuildKonnectConfig_BaseURLAndRegion(t *testing.T) {
	t.Run("default base URL when nothing set", func(t *testing.T) {
		// ResolveBaseURL returns BaseURLDefault when no config is set,
		// and also calls SetString on the config.
		setValues := map[string]string{}
		mock := &configtest.MockConfigHook{
			GetStringMock:      func(string) string { return "" },
			GetBoolMock:        func(string) bool { return false },
			GetIntMock:         func(string) int { return 0 },
			GetIntOrElseMock:   func(_ string, orElse int) int { return orElse },
			SaveMock:           func() error { return nil },
			BindFlagMock:       func(string, *pflag.Flag) error { return nil },
			GetProfileMock:     func() string { return "default" },
			GetStringSlickMock: func(string) []string { return nil },
			SetStringMock: func(k, v string) {
				setValues[k] = v
			},
			SetMock:     func(string, any) {},
			GetMock:     func(string) any { return nil },
			GetPathMock: func() string { return "" },
		}

		commonFlags := &CommonFlags{}
		flags := &konnectFlags{}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		assert.Equal(t, konnectcommon.BaseURLDefault, cfg.KongAddr,
			"should use default base URL")
	})

	t.Run("base-url flag overrides default", func(t *testing.T) {
		mock := newMockConfig()

		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			BaseURL: "https://custom.api.konghq.com",
		}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		assert.Equal(t, "https://custom.api.konghq.com", cfg.KongAddr)
	})

	t.Run("region flag constructs URL", func(t *testing.T) {
		mock := newMockConfig()

		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			Region: "eu",
		}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		assert.Equal(t, "https://eu.api.konghq.com", cfg.KongAddr)
	})

	t.Run("base-url flag takes precedence over region flag", func(t *testing.T) {
		mock := newMockConfig()

		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			BaseURL: "https://explicit.api.konghq.com",
			Region:  "eu",
		}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		assert.Equal(t, "https://explicit.api.konghq.com", cfg.KongAddr,
			"base-url should take precedence over region")
	})

	t.Run("invalid region returns error", func(t *testing.T) {
		mock := newMockConfig()

		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			Region: "invalid region!",
		}

		_, err := buildKonnectConfig(mock, commonFlags, flags)
		assert.Error(t, err, "invalid region should produce an error")
	})
}

func TestBuildKonnectConfig_ControlPlane(t *testing.T) {
	t.Run("from flag", func(t *testing.T) {
		mock := newMockConfig()

		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			ControlPlane: "my-control-plane",
		}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		assert.Equal(t, "my-control-plane", cfg.KonnectControlPlaneName)
	})

	t.Run("from config", func(t *testing.T) {
		mock := newMockConfig()
		mock.GetStringMock = func(key string) string {
			if key == configKonnectControlPlane {
				return "config-cp"
			}
			return ""
		}

		commonFlags := &CommonFlags{}
		flags := &konnectFlags{}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		assert.Equal(t, "config-cp", cfg.KonnectControlPlaneName)
	})

	t.Run("flag overrides config", func(t *testing.T) {
		mock := newMockConfig()
		mock.GetStringMock = func(key string) string {
			if key == configKonnectControlPlane {
				return "config-cp"
			}
			return ""
		}

		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			ControlPlane: "flag-cp",
		}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		assert.Equal(t, "flag-cp", cfg.KonnectControlPlaneName,
			"flag should override config for control-plane")
	})
}

func TestBuildKonnectConfig_RuntimeFlags(t *testing.T) {
	t.Run("runtime flag sets Runtime", func(t *testing.T) {
		mock := newMockConfig()
		commonFlags := &CommonFlags{}
		flags := &konnectFlags{Runtime: "docker"}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		assert.Equal(t, "docker", cfg.Runtime)
		assert.True(t, cfg.KonnectMode, "KonnectMode should still be true")
	})

	t.Run("runtime from config", func(t *testing.T) {
		mock := newMockConfig()
		mock.GetStringMock = func(key string) string {
			if key == configKonnectRuntime {
				return "kubernetes"
			}
			return ""
		}
		commonFlags := &CommonFlags{}
		flags := &konnectFlags{}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		assert.Equal(t, "kubernetes", cfg.Runtime)
	})

	t.Run("runtime flag overrides config", func(t *testing.T) {
		mock := newMockConfig()
		mock.GetStringMock = func(key string) string {
			if key == configKonnectRuntime {
				return "docker"
			}
			return ""
		}
		commonFlags := &CommonFlags{}
		flags := &konnectFlags{Runtime: "vm"}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		assert.Equal(t, "vm", cfg.Runtime,
			"flag should override config for runtime")
	})

	t.Run("target-images flag", func(t *testing.T) {
		mock := newMockConfig()
		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			TargetImages: []string{"custom-kong"},
		}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		assert.Equal(t, []string{"custom-kong"}, cfg.TargetImages)
	})

	t.Run("target-pods flag", func(t *testing.T) {
		mock := newMockConfig()
		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			TargetPods: []string{"kong-0", "kong-1"},
		}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		assert.Equal(t, []string{"kong-0", "kong-1"}, cfg.TargetPods)
	})

	t.Run("prefix-dir flag", func(t *testing.T) {
		mock := newMockConfig()
		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			PrefixDir: "/opt/kong",
		}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		assert.Equal(t, "/opt/kong", cfg.PrefixDir)
	})

	t.Run("runtime config values from file", func(t *testing.T) {
		stringValues := map[string]string{
			configKonnectPrefixDir: "/custom/kong",
		}
		sliceValues := map[string][]string{
			configKonnectTargetImages: {"my-kong"},
			configKonnectTargetPods:   {"pod-a"},
		}
		mock := newMockConfig()
		mock.GetStringMock = func(key string) string {
			return stringValues[key]
		}
		mock.GetStringSlickMock = func(key string) []string {
			return sliceValues[key]
		}
		commonFlags := &CommonFlags{}
		flags := &konnectFlags{}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)

		assert.Equal(t, "/custom/kong", cfg.PrefixDir)
		assert.Equal(t, []string{"my-kong"}, cfg.TargetImages)
		assert.Equal(t, []string{"pod-a"}, cfg.TargetPods)
	})
}

func TestBuildKonnectConfig_Defaults(t *testing.T) {
	mock := newMockConfig()

	commonFlags := &CommonFlags{}
	flags := &konnectFlags{}

	cfg, err := buildKonnectConfig(mock, commonFlags, flags)
	require.NoError(t, err)

	defaults := collector.DefaultConfig()
	assert.Equal(t, defaults.LineLimit, cfg.LineLimit,
		"LineLimit should preserve collector default")
	assert.Equal(t, defaults.PrefixDir, cfg.PrefixDir,
		"PrefixDir should preserve collector default")
	assert.Equal(t, defaults.TargetImages, cfg.TargetImages,
		"TargetImages should preserve collector default")
	assert.Empty(t, cfg.Runtime,
		"Runtime should be empty by default (auto-detect)")
	assert.True(t, cfg.KonnectMode)
}

func TestBuildKonnectConfig_Namespace(t *testing.T) {
	t.Run("namespace from config", func(t *testing.T) {
		mock := newMockConfig()
		mock.GetStringMock = func(key string) string {
			if key == configKonnectNamespace {
				return "kong"
			}
			return ""
		}

		commonFlags := &CommonFlags{}
		flags := &konnectFlags{}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)
		assert.Equal(t, "kong", cfg.Namespace)
	})

	t.Run("namespace from flag", func(t *testing.T) {
		mock := newMockConfig()
		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			Namespace: "kong-dp",
		}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)
		assert.Equal(t, "kong-dp", cfg.Namespace)
	})

	t.Run("flag overrides config", func(t *testing.T) {
		mock := newMockConfig()
		mock.GetStringMock = func(key string) string {
			if key == configKonnectNamespace {
				return "config-ns"
			}
			return ""
		}

		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			Namespace: "flag-ns",
		}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)
		assert.Equal(t, "flag-ns", cfg.Namespace,
			"flag should override config for namespace")
	})

	t.Run("namespace works alongside target-pods", func(t *testing.T) {
		mock := newMockConfig()
		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			Namespace:  "kong",
			TargetPods: []string{"kong-gateway-0"},
		}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)
		assert.Equal(t, "kong", cfg.Namespace)
		assert.Equal(t, []string{"kong-gateway-0"}, cfg.TargetPods)
	})
}

func TestKonnectKubernetesRequiresNamespace(t *testing.T) {
	t.Run("kubernetes without namespace returns error", func(t *testing.T) {
		mock := newMockConfig()
		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			Runtime: "kubernetes",
		}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)
		assert.Equal(t, "kubernetes", cfg.Runtime)
		assert.Empty(t, cfg.Namespace)

		if cfg.Runtime == "kubernetes" && cfg.Namespace == "" {
			assert.True(t, true, "should require namespace for kubernetes")
		}
	})

	t.Run("kubernetes with namespace does not error", func(t *testing.T) {
		mock := newMockConfig()
		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			Runtime:   "kubernetes",
			Namespace: "kong",
		}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)
		assert.Equal(t, "kubernetes", cfg.Runtime)
		assert.Equal(t, "kong", cfg.Namespace)

		needsError := cfg.Runtime == "kubernetes" && cfg.Namespace == ""
		assert.False(t, needsError, "should not require error when namespace is set")
	})

	t.Run("docker without namespace does not error", func(t *testing.T) {
		mock := newMockConfig()
		commonFlags := &CommonFlags{}
		flags := &konnectFlags{
			Runtime: "docker",
		}

		cfg, err := buildKonnectConfig(mock, commonFlags, flags)
		require.NoError(t, err)
		assert.Equal(t, "docker", cfg.Runtime)
		assert.Empty(t, cfg.Namespace)

		needsError := cfg.Runtime == "kubernetes" && cfg.Namespace == ""
		assert.False(t, needsError, "namespace not required for non-kubernetes runtime")
	})
}
