package config

import (
	"testing"

	utilviper "github.com/kong/kongctl/internal/util/viper"
)

func TestBuildProfiledConfig_ProfileEnvWithDashes(t *testing.T) {
	t.Setenv("KONGCTL_TEAM_A_B_C_KONNECT_PAT", "token-123")

	profile := "team-a-b-c"
	mainv := utilviper.NewViper("nonexistent.yaml")
	mainv.Set(profile, map[string]any{})

	cfg := BuildProfiledConfig(profile, "nonexistent.yaml", mainv)

	if got := cfg.GetString("konnect.pat"); got != "token-123" {
		t.Fatalf("expected konnect.pat to be %q, got %q", "token-123", got)
	}
}

func TestBuildProfiledConfig_EmptyConfigFileWithEnvVar(t *testing.T) {
	t.Setenv("KONGCTL_DEFAULT_KONNECT_PAT", "token-from-env")

	profile := "default"
	// Simulate an empty config file - main viper has no profile keys
	mainv := utilviper.NewViper("nonexistent.yaml")

	cfg := BuildProfiledConfig(profile, "nonexistent.yaml", mainv)

	// Should still read environment variable even when config file is empty
	if got := cfg.GetString("konnect.pat"); got != "token-from-env" {
		t.Fatalf("expected konnect.pat to be %q, got %q", "token-from-env", got)
	}
}

func TestBuildProfiledConfig_ProfileHTTPTimeoutEnv(t *testing.T) {
	t.Setenv("KONGCTL_E2E_HTTP_TIMEOUT", "13s")

	profile := "e2e"
	mainv := utilviper.NewViper("nonexistent.yaml")
	mainv.Set(profile, map[string]any{})

	cfg := BuildProfiledConfig(profile, "nonexistent.yaml", mainv)

	if got := cfg.GetString("http-timeout"); got != "13s" {
		t.Fatalf("expected http-timeout to be %q, got %q", "13s", got)
	}
}

func TestBuildProfiledConfig_ProfileHTTPTransportEnv(t *testing.T) {
	t.Setenv("KONGCTL_E2E_HTTP_TCP_USER_TIMEOUT", "45s")
	t.Setenv("KONGCTL_E2E_HTTP_DISABLE_KEEPALIVES", "true")
	t.Setenv("KONGCTL_E2E_HTTP_RECYCLE_CONNECTIONS_ON_ERROR", "1")

	profile := "e2e"
	mainv := utilviper.NewViper("nonexistent.yaml")
	mainv.Set(profile, map[string]any{})

	cfg := BuildProfiledConfig(profile, "nonexistent.yaml", mainv)

	if got := cfg.GetString("http-tcp-user-timeout"); got != "45s" {
		t.Fatalf("expected http-tcp-user-timeout to be %q, got %q", "45s", got)
	}
	if got := cfg.GetString("http-disable-keepalives"); got != "true" {
		t.Fatalf("expected http-disable-keepalives to be %q, got %q", "true", got)
	}
	if got := cfg.GetString("http-recycle-connections-on-error"); got != "1" {
		t.Fatalf("expected http-recycle-connections-on-error to be %q, got %q", "1", got)
	}
}
