package meta

import "testing"

func TestUserAgentDefaultsToDev(t *testing.T) {
	original := CLIVersion()
	t.Cleanup(func() {
		SetCLIVersion(original)
	})

	SetCLIVersion("")

	if got := CLIVersion(); got != DefaultCLIVersion {
		t.Fatalf("CLIVersion() = %q, want %q", got, DefaultCLIVersion)
	}

	if got := UserAgent(); got != "kongctl/dev" {
		t.Fatalf("UserAgent() = %q, want %q", got, "kongctl/dev")
	}
}

func TestUserAgentUsesConfiguredVersion(t *testing.T) {
	original := CLIVersion()
	t.Cleanup(func() {
		SetCLIVersion(original)
	})

	SetCLIVersion(" 0.5.0 ")

	if got := UserAgent(); got != "kongctl/v0.5.0" {
		t.Fatalf("UserAgent() = %q, want %q", got, "kongctl/v0.5.0")
	}

	if got := CLIVersion(); got != "0.5.0" {
		t.Fatalf("CLIVersion() = %q, want %q", got, "0.5.0")
	}
}

func TestUserAgentKeepsPrefixedVersion(t *testing.T) {
	original := CLIVersion()
	t.Cleanup(func() {
		SetCLIVersion(original)
	})

	SetCLIVersion(" v0.5.0 ")

	if got := UserAgent(); got != "kongctl/v0.5.0" {
		t.Fatalf("UserAgent() = %q, want %q", got, "kongctl/v0.5.0")
	}
}
