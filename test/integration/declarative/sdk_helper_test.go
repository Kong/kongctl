//go:build integration
// +build integration

package declarative_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	kongctlconfig "github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
	"github.com/spf13/pflag"
)

// GetSDKFactory returns either a real or mock SDK factory based on environment
func GetSDKFactory(t *testing.T) helpers.SDKAPIFactory {
	if token := os.Getenv("KONNECT_INTEGRATION_TOKEN"); token != "" {
		t.Logf("Using real Konnect SDK with token from KONNECT_INTEGRATION_TOKEN")
		return func(cfg kongctlconfig.Hook, _ *slog.Logger) (helpers.SDKAPI, error) {
			// Override token from environment
			cfg.Set("konnect.token", token)
			
			// For real SDK, we would need to build the actual SDK here
			// For now, return an error indicating real SDK is not implemented
			return nil, fmt.Errorf(
				"real Konnect SDK integration not yet implemented - remove KONNECT_INTEGRATION_TOKEN to use mocks")
		}
	}
	
	t.Logf("Using mock Konnect SDK (set KONNECT_INTEGRATION_TOKEN to use real API)")
	return func(_ kongctlconfig.Hook, _ *slog.Logger) (helpers.SDKAPI, error) {
		// Return mock SDK with test-specific factories
		return &helpers.MockKonnectSDK{
			T:             t,
			PortalFactory: func() helpers.PortalAPI {
				return NewMockPortalAPI(t)
			},
			AppAuthStrategiesFactory: func() helpers.AppAuthStrategiesAPI {
				return NewMockAppAuthStrategiesAPI(t)
			},
		}, nil
	}
}

// SetupTestContext creates a context with all necessary values for testing
func SetupTestContext(t *testing.T) context.Context {
	ctx := context.Background()
	
	// Add SDK factory
	sdkFactory := GetSDKFactory(t)
	ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, sdkFactory)
	
	// Add config
	testConfig := GetTestConfig()
	ctx = context.WithValue(ctx, kongctlconfig.ConfigKey, testConfig)
	
	// Add logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors in tests
	}))
	ctx = context.WithValue(ctx, log.LoggerKey, logger)
	
	// Add IO streams
	streams := &iostreams.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	ctx = context.WithValue(ctx, iostreams.StreamsKey, streams)
	
	return ctx
}

// GetTestConfig returns a minimal test configuration
func GetTestConfig() kongctlconfig.Hook {
	// Create a simple mock config
	return &mockConfig{
		values: map[string]interface{}{
			"konnect.base_url": "https://us.api.konghq.com",
			"konnect.pat":      "test-pat-token", // Dummy PAT to prevent auth errors
		},
	}
}

// mockConfig implements kongctlconfig.Hook for testing
type mockConfig struct {
	values map[string]interface{}
}

func (m *mockConfig) Save() error {
	return nil
}

func (m *mockConfig) GetString(key string) string {
	if v, ok := m.values[key].(string); ok {
		return v
	}
	return ""
}

func (m *mockConfig) GetBool(key string) bool {
	if v, ok := m.values[key].(bool); ok {
		return v
	}
	return false
}

func (m *mockConfig) GetInt(key string) int {
	if v, ok := m.values[key].(int); ok {
		return v
	}
	return 0
}

func (m *mockConfig) GetIntOrElse(key string, orElse int) int {
	if v, ok := m.values[key].(int); ok {
		return v
	}
	return orElse
}

func (m *mockConfig) GetStringSlice(key string) []string {
	if v, ok := m.values[key].([]string); ok {
		return v
	}
	return nil
}

func (m *mockConfig) SetString(key string, value string) {
	if m.values == nil {
		m.values = make(map[string]interface{})
	}
	m.values[key] = value
}

func (m *mockConfig) Set(key string, value interface{}) {
	if m.values == nil {
		m.values = make(map[string]interface{})
	}
	m.values[key] = value
}

func (m *mockConfig) Get(key string) interface{} {
	return m.values[key]
}

func (m *mockConfig) BindFlag(_ string, _ *pflag.Flag) error {
	return nil
}

func (m *mockConfig) GetProfile() string {
	return "test"
}

func (m *mockConfig) GetPath() string {
	return ""
}