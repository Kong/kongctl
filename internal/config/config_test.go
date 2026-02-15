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
