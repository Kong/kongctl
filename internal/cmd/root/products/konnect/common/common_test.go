package common

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/konnect/auth"
	"github.com/kong/kongctl/internal/konnect/httpclient"
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
			HTTPRetryInitialIntervalConfigPath:    "200",
			HTTPRetryMaxIntervalConfigPath:        "5000",
			HTTPRetryBackoffFactorConfigPath:      "1.5",
			HTTPRetryOnConnectionErrorsConfigPath: "true",
		})

		rc, err := ResolveRetryConfig(cfg)
		require.NoError(t, err)
		require.Equal(t, 3, rc.MaxAttempts)
		require.Equal(t, httpclient.RetryStrategyDefault, rc.Strategy)
		require.Equal(t, 200, rc.InitialIntervalMS)
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

	t.Run("legacy top-level values are ignored", func(t *testing.T) {
		cfg, _ := newTestConfig(map[string]string{
			cmdcommon.HTTPRetryMaxAttemptsConfigPath: "3",
		})

		rc, err := ResolveRetryConfig(cfg)
		require.NoError(t, err)
		require.Equal(t, httpclient.DefaultRetryMaxAttempts, rc.MaxAttempts)
	})
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
