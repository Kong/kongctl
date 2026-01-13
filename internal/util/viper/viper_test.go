package viper

import "testing"

func TestNewViperEnvKeyReplacer(t *testing.T) {
	t.Setenv("KONGCTL_LOG_LEVEL", "debug")
	t.Setenv("KONGCTL_KONNECT_DECLARATIVE_BASE_DIR", "/tmp")

	v := NewViper("nonexistent.yaml")

	if got := v.GetString("log-level"); got != "debug" {
		t.Fatalf("expected log-level to be %q, got %q", "debug", got)
	}
	if got := v.GetString("konnect.declarative.base-dir"); got != "/tmp" {
		t.Fatalf("expected konnect.declarative.base-dir to be %q, got %q", "/tmp", got)
	}
}

func TestNewViperEnvKeyReplacerProfileWithDashes(t *testing.T) {
	t.Setenv("KONGCTL_TEAM_A_B_C_KONNECT_PAT", "token-123")

	v := NewViper("nonexistent.yaml")
	v.Set("team-a-b-c", map[string]any{})

	profile := v.Sub("team-a-b-c")
	if profile == nil {
		t.Fatal("expected profile viper, got nil")
	}

	if got := profile.GetString("konnect.pat"); got != "token-123" {
		t.Fatalf("expected konnect.pat to be %q, got %q", "token-123", got)
	}
}
