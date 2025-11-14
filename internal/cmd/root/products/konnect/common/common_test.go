package common

import (
	"fmt"
	"testing"

	configtest "github.com/kong/kongctl/test/config"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func newTestConfig(initial map[string]string) (*configtest.MockConfigHook, map[string]string) {
	store := make(map[string]string)
	for k, v := range initial {
		store[k] = v
	}

	cfg := &configtest.MockConfigHook{
		GetStringMock: func(key string) string {
			return store[key]
		},
		GetBoolMock: func(string) bool {
			return false
		},
		GetIntMock: func(string) int {
			return 0
		},
		SaveMock: func() error { return nil },
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
