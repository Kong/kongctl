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
