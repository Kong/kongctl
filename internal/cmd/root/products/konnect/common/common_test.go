package common

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/auth"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	kprofile "github.com/kong/kongctl/internal/profile"
	utilviper "github.com/kong/kongctl/internal/util/viper"
	configtest "github.com/kong/kongctl/test/config"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func newTestConfig(initial map[string]string) (*configtest.MockConfigHook, map[string]string) {
	store := make(map[string]string)
	maps.Copy(store, initial)

	cfg := &configtest.MockConfigHook{
		GetStringMock: func(key string) string {
			return store[key]
		},
		GetBoolMock: func(string) bool {
			return false
		},
		GetIntMock: func(key string) int {
			value, err := strconv.Atoi(store[key])
			if err != nil {
				return 0
			}
			return value
		},
		BindFlagMock: func(string, *pflag.Flag) error {
			return nil
		},
		GetProfileMock:     func() string { return "default" },
		GetStringSlickMock: func(string) []string { return nil },
		SetStringMock: func(k, v string) {
			store[k] = v
		},
		SetMock: func(k string, v any) {
			store[k] = fmt.Sprint(v)
		},
		GetMock: func(k string) any {
			return store[k]
		},
		GetPathMock: func() string { return "" },
	}

	return cfg, store
}

func TestBuildBaseURLFromRegion(t *testing.T) {
	tests := []struct {
		name        string
		region      string
		expectedURL string
		wantErr     bool
	}{
		{name: "us", region: "us", expectedURL: "https://us.api.konghq.com"},
		{name: "mixed case", region: "Eu", expectedURL: "https://eu.api.konghq.com"},
		{name: "global", region: "global", expectedURL: GlobalBaseURL},
		{name: "invalid chars", region: "bad/region", wantErr: true},
		{name: "empty", region: " ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := BuildBaseURLFromRegion(tt.region)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedURL, url)
		})
	}
}

func TestResolveBaseURL(t *testing.T) {
	t.Run("explicit base url wins", func(t *testing.T) {
		cfg, store := newTestConfig(map[string]string{
			BaseURLConfigPath: "https://custom.example.com",
		})
		url, err := ResolveBaseURL(cfg)
		require.NoError(t, err)
		require.Equal(t, "https://custom.example.com", url)
		require.Equal(t, "https://custom.example.com", store[BaseURLConfigPath])
	})

	t.Run("region constructs base url", func(t *testing.T) {
		cfg, store := newTestConfig(map[string]string{
			RegionConfigPath: "eu",
		})
		url, err := ResolveBaseURL(cfg)
		require.NoError(t, err)
		require.Equal(t, "https://eu.api.konghq.com", url)
		require.Equal(t, url, store[BaseURLConfigPath])
	})

	t.Run("default fallback", func(t *testing.T) {
		cfg, store := newTestConfig(map[string]string{})
		url, err := ResolveBaseURL(cfg)
		require.NoError(t, err)
		require.Equal(t, BaseURLDefault, url)
		require.Equal(t, BaseURLDefault, store[BaseURLConfigPath])
	})

	t.Run("invalid region returns error", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			RegionConfigPath: "bad/region",
		})
		_, err := ResolveBaseURL(cfg)
		require.Error(t, err)
	})
}

func TestEnvironmentDefaultsFor(t *testing.T) {
	tests := []struct {
		name        string
		env         string
		wantBaseURL string
		wantAuthURL string
		wantClient  string
		wantErr     bool
	}{
		{
			name:        "default production",
			env:         "",
			wantBaseURL: BaseURLDefault,
			wantAuthURL: AuthBaseURLDefault,
			wantClient:  MachineClientIDDefault,
		},
		{
			name:        "com",
			env:         "com",
			wantBaseURL: BaseURLDefault,
			wantAuthURL: AuthBaseURLDefault,
			wantClient:  MachineClientIDDefault,
		},
		{
			name:        "production alias",
			env:         "production",
			wantBaseURL: BaseURLDefault,
			wantAuthURL: AuthBaseURLDefault,
			wantClient:  MachineClientIDDefault,
		},
		{
			name:        "tech",
			env:         "tech",
			wantBaseURL: TechBaseURLDefault,
			wantAuthURL: TechGlobalBaseURL,
			wantClient:  TechMachineClientID,
		},
		{name: "unknown", env: "stage", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EnvironmentDefaultsFor(tt.env)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantBaseURL, got.BaseURL)
			require.Equal(t, tt.wantAuthURL, got.AuthBaseURL)
			require.Equal(t, tt.wantClient, got.MachineClientID)
		})
	}
}

func TestBuildBaseURLFromRegionForEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		region      string
		environment string
		want        string
		wantErr     bool
	}{
		{
			name:        "production regional",
			region:      "eu",
			environment: EnvironmentProduction,
			want:        "https://eu.api.konghq.com",
		},
		{
			name:        "tech regional",
			region:      "eu",
			environment: EnvironmentTech,
			want:        "https://eu.api.konghq.tech",
		},
		{
			name:        "tech global",
			region:      "global",
			environment: EnvironmentTech,
			want:        TechGlobalBaseURL,
		},
		{
			name:        "invalid environment",
			region:      "eu",
			environment: "stage",
			wantErr:     true,
		},
		{
			name:        "invalid region",
			region:      "bad/region",
			environment: EnvironmentTech,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildBaseURLFromRegionForEnvironment(tt.region, tt.environment)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestInferEnvironmentDefaultsFromURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantOK      bool
		wantBaseURL string
	}{
		{
			name:        "tech regional URL",
			url:         "https://us.api.konghq.tech",
			wantOK:      true,
			wantBaseURL: TechBaseURLDefault,
		},
		{
			name:        "production global URL",
			url:         "https://global.api.konghq.com",
			wantOK:      true,
			wantBaseURL: BaseURLDefault,
		},
		{
			name:        "tech URL with query",
			url:         "https://global.api.konghq.tech/v3?redirect=https://example.test",
			wantOK:      true,
			wantBaseURL: TechBaseURLDefault,
		},
		{
			name:   "tech string in path does not infer",
			url:    "https://example.test/konghq.tech",
			wantOK: false,
		},
		{
			name:   "tech string in query does not infer",
			url:    "https://example.test?target=global.api.konghq.tech",
			wantOK: false,
		},
		{name: "custom URL", url: "https://example.test", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := InferEnvironmentDefaultsFromURL(tt.url)
			require.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				require.Equal(t, tt.wantBaseURL, got.BaseURL)
			}
		})
	}
}

func TestResolveHTTPTimeout(t *testing.T) {
	t.Run("default fallback", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{})

		timeout, err := ResolveHTTPTimeout(cfg)
		require.NoError(t, err)
		require.Equal(t, httpclient.DefaultHTTPClientTimeout, timeout)
	})

	t.Run("configured timeout", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			cmdcommon.HTTPTimeoutConfigPath: "15s",
		})

		timeout, err := ResolveHTTPTimeout(cfg)
		require.NoError(t, err)
		require.Equal(t, 15*time.Second, timeout)
	})

	t.Run("explicitly disabled timeout", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			cmdcommon.HTTPTimeoutConfigPath: "0s",
		})

		timeout, err := ResolveHTTPTimeout(cfg)
		require.NoError(t, err)
		require.Equal(t, time.Duration(0), timeout)
	})

	t.Run("invalid timeout returns error", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			cmdcommon.HTTPTimeoutConfigPath: "banana",
		})

		_, err := ResolveHTTPTimeout(cfg)
		require.Error(t, err)
	})
}

func TestResolveHTTPTransportOptions(t *testing.T) {
	t.Run("default fallback", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{})

		options, err := ResolveHTTPTransportOptions(cfg)
		require.NoError(t, err)
		require.Equal(t, httpclient.TransportOptions{}, options)
	})

	t.Run("configured values", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			cmdcommon.HTTPTCPUserTimeoutConfigPath:            "45s",
			cmdcommon.HTTPDisableKeepAlivesConfigPath:         "true",
			cmdcommon.HTTPRecycleConnectionsOnErrorConfigPath: "1",
		})

		options, err := ResolveHTTPTransportOptions(cfg)
		require.NoError(t, err)
		require.Equal(t, 45*time.Second, options.TCPUserTimeout)
		require.True(t, options.DisableKeepAlives)
		require.True(t, options.RecycleConnectionsOnError)
	})

	t.Run("explicitly disabled tcp user timeout", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			cmdcommon.HTTPTCPUserTimeoutConfigPath: "default",
		})

		options, err := ResolveHTTPTransportOptions(cfg)
		require.NoError(t, err)
		require.Equal(t, time.Duration(0), options.TCPUserTimeout)
	})

	t.Run("invalid duration returns error", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			cmdcommon.HTTPTCPUserTimeoutConfigPath: "banana",
		})

		_, err := ResolveHTTPTransportOptions(cfg)
		require.Error(t, err)
	})

	t.Run("invalid bool returns error", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			cmdcommon.HTTPDisableKeepAlivesConfigPath: "banana",
		})

		_, err := ResolveHTTPTransportOptions(cfg)
		require.Error(t, err)
	})
}

func TestResolveRetryConfig(t *testing.T) {
	t.Run("defaults when unset", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{})

		rc, err := ResolveRetryConfig(cfg)
		require.NoError(t, err)
		require.Equal(t, httpclient.RetryStrategyDefault, rc.Strategy)
		require.Equal(t, httpclient.DefaultRetryMaxAttempts, rc.MaxAttempts)
		require.Equal(t, httpclient.DefaultRetryInitialIntervalMS, rc.InitialIntervalMS)
		require.Equal(t, httpclient.DefaultRetryMaxIntervalMS, rc.MaxIntervalMS)
		require.Equal(t, httpclient.DefaultRetryBackoffFactor, rc.BackoffFactor)
		require.False(t, rc.RetryConnectionErrors)
	})

	t.Run("explicit values", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			HTTPRetryMaxAttemptsConfigPath:        "3",
			HTTPRetryInitialIntervalConfigPath:    "500",
			HTTPRetryMaxIntervalConfigPath:        "5000",
			HTTPRetryBackoffFactorConfigPath:      "1.5",
			HTTPRetryOnConnectionErrorsConfigPath: "true",
		})

		rc, err := ResolveRetryConfig(cfg)
		require.NoError(t, err)
		require.Equal(t, 3, rc.MaxAttempts)
		require.Equal(t, httpclient.RetryStrategyDefault, rc.Strategy)
		require.Equal(t, 500, rc.InitialIntervalMS)
		require.Equal(t, 5000, rc.MaxIntervalMS)
		require.Equal(t, 1.5, rc.BackoffFactor)
		require.True(t, rc.RetryConnectionErrors)
	})

	t.Run("Max attempts = 1 disables retries", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			HTTPRetryMaxAttemptsConfigPath: "1",
		})

		rc, err := ResolveRetryConfig(cfg)
		require.NoError(t, err)
		require.Equal(t, httpclient.RetryStrategyNone, rc.Strategy)
		require.Equal(t, 1, rc.MaxAttempts)
	})

	t.Run("invalid max attempts returns error", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			HTTPRetryMaxAttemptsConfigPath: "banana",
		})

		_, err := ResolveRetryConfig(cfg)
		require.Error(t, err)
	})

	t.Run("negative max attempts returns error", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			HTTPRetryMaxAttemptsConfigPath: "-1",
		})

		_, err := ResolveRetryConfig(cfg)
		require.Error(t, err)
	})

	t.Run("invalid initial interval returns error", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			HTTPRetryInitialIntervalConfigPath: "banana",
		})

		_, err := ResolveRetryConfig(cfg)
		require.Error(t, err)

		cfg, _ = newTestConfig(map[string]string{
			HTTPRetryInitialIntervalConfigPath: "-1",
		})

		_, err = ResolveRetryConfig(cfg)
		require.Error(t, err)
	})

	t.Run("initial interval below min returns error", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			HTTPRetryInitialIntervalConfigPath: "50",
		})
		_, err := ResolveRetryConfig(cfg)
		require.Error(t, err)
	})

	t.Run("initial interval above max returns error", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			HTTPRetryInitialIntervalConfigPath: "31000",
		})
		_, err := ResolveRetryConfig(cfg)
		require.Error(t, err)
	})

	t.Run("max interval below min returns error", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			HTTPRetryMaxIntervalConfigPath: "500",
		})
		_, err := ResolveRetryConfig(cfg)
		require.Error(t, err)
	})

	t.Run("max interval above max returns error", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			HTTPRetryMaxIntervalConfigPath: "301000",
		})
		_, err := ResolveRetryConfig(cfg)
		require.Error(t, err)
	})

	t.Run("initial interval greater than max interval returns error", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			HTTPRetryInitialIntervalConfigPath: "5000",
			HTTPRetryMaxIntervalConfigPath:     "2000",
		})
		_, err := ResolveRetryConfig(cfg)
		require.Error(t, err)
	})

	t.Run("max attempts above cap returns error", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			HTTPRetryMaxAttemptsConfigPath: "11",
		})
		_, err := ResolveRetryConfig(cfg)
		require.Error(t, err)
	})

	t.Run("invalid backoff factor returns error", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			HTTPRetryBackoffFactorConfigPath: "not-a-number",
		})

		_, err := ResolveRetryConfig(cfg)
		require.Error(t, err)
	})

	t.Run("backoff factor below minimum returns error", func(t *testing.T) {
		for _, v := range []string{"0.5", "0.001", "0.999", "1", "1.499"} {
			cfg, _ := newTestConfig(map[string]string{
				HTTPRetryBackoffFactorConfigPath: v,
			})
			_, err := ResolveRetryConfig(cfg)
			require.Error(t, err, "factor %s should be rejected", v)
		}
	})

	t.Run("backoff factor above maximum returns error", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			HTTPRetryBackoffFactorConfigPath: "3.1",
		})
		_, err := ResolveRetryConfig(cfg)
		require.Error(t, err)
	})

	t.Run("cumulative backoff budget above maximum returns error", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			HTTPRetryMaxAttemptsConfigPath:     "10",
			HTTPRetryInitialIntervalConfigPath: "30000",
			HTTPRetryMaxIntervalConfigPath:     "120000",
			HTTPRetryBackoffFactorConfigPath:   "3",
		})
		_, err := ResolveRetryConfig(cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cumulative backoff budget")
	})

	t.Run("non-finite backoff factor is rejected", func(t *testing.T) {
		for _, v := range []string{"NaN", "+Inf", "-Inf", "Inf"} {
			cfg, _ := newTestConfig(map[string]string{
				HTTPRetryBackoffFactorConfigPath: v,
			})
			_, err := ResolveRetryConfig(cfg)
			require.Error(t, err, "factor %s should be rejected", v)
		}
	})
}

func TestResolveRetryConfigForVerb(t *testing.T) {
	t.Run("imperative verbs force no retry", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			HTTPRetryMaxAttemptsConfigPath:        "5",
			HTTPRetryInitialIntervalConfigPath:    "500",
			HTTPRetryMaxIntervalConfigPath:        "5000",
			HTTPRetryBackoffFactorConfigPath:      "3",
			HTTPRetryOnConnectionErrorsConfigPath: "true",
		})

		rc, err := resolveRetryConfigForVerb(cfg, verbs.Get)
		require.NoError(t, err)
		require.Equal(t, httpclient.RetryStrategyNone, rc.Strategy)
		require.Equal(t, 1, rc.MaxAttempts)
		require.Equal(t, httpclient.DefaultRetryInitialIntervalMS, rc.InitialIntervalMS)
		require.Equal(t, httpclient.DefaultRetryMaxIntervalMS, rc.MaxIntervalMS)
		require.Equal(t, httpclient.DefaultRetryBackoffFactor, rc.BackoffFactor)
		require.False(t, rc.RetryConnectionErrors)
	})

	t.Run("declarative verbs use resolved retry config", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			HTTPRetryMaxAttemptsConfigPath:        "5",
			HTTPRetryInitialIntervalConfigPath:    "500",
			HTTPRetryMaxIntervalConfigPath:        "5000",
			HTTPRetryBackoffFactorConfigPath:      "3",
			HTTPRetryOnConnectionErrorsConfigPath: "true",
		})

		rc, err := resolveRetryConfigForVerb(cfg, verbs.Plan)
		require.NoError(t, err)
		require.Equal(t, httpclient.RetryStrategyBackoff, rc.Strategy)
		require.Equal(t, 5, rc.MaxAttempts)
		require.Equal(t, 500, rc.InitialIntervalMS)
		require.Equal(t, 5000, rc.MaxIntervalMS)
		require.Equal(t, 3.0, rc.BackoffFactor)
		require.True(t, rc.RetryConnectionErrors)
	})
}

func TestKonnectSDKFactoryRetrySelection(t *testing.T) {
	t.Run("default factory ignores retry config parsing", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			RegionConfigPath:                 "bad/region",
			HTTPRetryMaxAttemptsConfigPath:   "banana",
			HTTPRetryBackoffFactorConfigPath: "not-a-number",
		})

		_, err := KonnectSDKFactory(cfg, nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid konnect region")
		require.NotContains(t, err.Error(), HTTPRetryMaxAttemptsConfigPath)
	})

	t.Run("declarative verb factory validates retry config", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			RegionConfigPath:               "bad/region",
			HTTPRetryMaxAttemptsConfigPath: "banana",
		})

		_, err := KonnectSDKFactoryForVerb(verbs.Plan, cfg, nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), HTTPRetryMaxAttemptsConfigPath)
		require.NotContains(t, err.Error(), "invalid konnect region")
	})

	t.Run("imperative verb factory uses no retry config", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			RegionConfigPath:               "bad/region",
			HTTPRetryMaxAttemptsConfigPath: "banana",
		})

		_, err := KonnectSDKFactoryForVerb(verbs.Get, cfg, nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid konnect region")
		require.NotContains(t, err.Error(), HTTPRetryMaxAttemptsConfigPath)
	})
}

func TestGetSDKFactoryPrefersDefaultOverride(t *testing.T) {
	original := helpers.DefaultSDKFactory
	t.Cleanup(func() {
		helpers.DefaultSDKFactory = original
	})

	cfg, _ := newTestConfig(map[string]string{})

	helpers.DefaultSDKFactory = func(config.Hook, *slog.Logger) (helpers.SDKAPI, error) {
		return nil, context.Canceled
	}

	_, err := GetSDKFactory()(cfg, nil)

	require.ErrorIs(t, err, context.Canceled)
}

func TestResolveAccessTokenMapsMissingCredentialsToGuidance(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := newTestConfig(map[string]string{})
	cfg.GetPathMock = func() string { return filepath.Join(dir, "config.yaml") }
	source := auth.NewTokenSource(cfg, auth.TokenSourceOptions{
		RefreshURL: BaseURLDefault + RefreshPathDefault,
	})

	_, err := ResolveAccessToken(t.Context(), cfg, source)

	require.Error(t, err)
	require.Contains(t, err.Error(), "authentication token not available")
	require.Contains(t, err.Error(), "kongctl login")
	require.NotContains(t, err.Error(), dir)
	require.NotContains(t, err.Error(), "stat ")
}

func newFileBackedConfig(t *testing.T, dir, profile string, fileKeys map[string]any) *config.ProfiledConfig {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	mainv := utilviper.NewViper(path)
	for key, value := range fileKeys {
		mainv.Set(key, value)
	}
	return config.BuildProfiledConfig(profile, path, mainv)
}

func requireAuthGuidance(t *testing.T, cfg config.Hook) {
	t.Helper()
	_, err := ResolveAccessToken(t.Context(), cfg, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "authentication token not available")
	require.NotContains(t, err.Error(), "is not configured")
}

func TestResolveAccessTokenUnknownProfileReportsNotConfigured(t *testing.T) {
	dir := t.TempDir()
	cfg := newFileBackedConfig(t, dir, "tech", nil)

	_, err := ResolveAccessToken(t.Context(), cfg, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), `profile "tech" is not configured`)
	require.NotContains(t, err.Error(), "authentication token not available")
}

func TestResolveAccessTokenKnownProfilesReportAuthGuidance(t *testing.T) {
	t.Run("default profile", func(t *testing.T) {
		cfg := newFileBackedConfig(t, t.TempDir(), kprofile.DefaultProfile, nil)
		requireAuthGuidance(t, cfg)
	})

	t.Run("profile in config file", func(t *testing.T) {
		cfg := newFileBackedConfig(t, t.TempDir(), "dev", map[string]any{
			"dev.konnect.base_url": BaseURLDefault,
		})
		requireAuthGuidance(t, cfg)
	})

	t.Run("profile with environment variables", func(t *testing.T) {
		t.Setenv("KONGCTL_CI_HTTP_TIMEOUT", "13s")
		cfg := newFileBackedConfig(t, t.TempDir(), "ci", nil)
		requireAuthGuidance(t, cfg)
	})

	t.Run("profile with stored credential", func(t *testing.T) {
		dir := t.TempDir()
		credPath := filepath.Join(dir, ".work-konnect-token.json")
		require.NoError(t, os.WriteFile(credPath, []byte("{}"), 0o600))
		cfg := newFileBackedConfig(t, dir, "work", nil)
		requireAuthGuidance(t, cfg)
	})
}

func TestResolveAccessTokenPreservesContextErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	cfg, _ := newTestConfig(map[string]string{})
	source := auth.NewTokenSource(cfg, auth.TokenSourceOptions{
		RefreshURL: BaseURLDefault + RefreshPathDefault,
	})

	_, err := ResolveAccessToken(ctx, cfg, source)

	require.ErrorIs(t, err, context.Canceled)
}

func TestKonnectSDKFactoryReturnsAuthConfigurationErrors(t *testing.T) {
	cfg, _ := newTestConfig(map[string]string{
		RegionConfigPath: "bad/region",
	})

	_, err := KonnectSDKFactory(cfg, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid konnect region")
	require.NotContains(t, err.Error(), "authentication token not available")
	require.NotContains(t, err.Error(), "no access token available")
}
