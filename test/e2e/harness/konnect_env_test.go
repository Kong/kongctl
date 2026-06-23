//go:build e2e

package harness

import (
	"testing"

	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
)

func clearKonnectTargetEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		KonnectEnvironmentEnvName,
		KonnectBaseURLEnvName,
		KonnectBaseAuthURLEnvName,
		KonnectMachineClientIDEnvName,
	} {
		t.Setenv(name, "")
	}
}

func TestResolveKonnectTargetFromEnvDefaultsToProduction(t *testing.T) {
	clearKonnectTargetEnv(t)

	target, err := ResolveKonnectTargetFromEnv()
	if err != nil {
		t.Fatalf("ResolveKonnectTargetFromEnv() error = %v", err)
	}

	if target.BaseURL != konnectcommon.BaseURLDefault {
		t.Fatalf("BaseURL = %q, want %q", target.BaseURL, konnectcommon.BaseURLDefault)
	}
	if target.BaseAuthURL != konnectcommon.AuthBaseURLDefault {
		t.Fatalf("BaseAuthURL = %q, want %q", target.BaseAuthURL, konnectcommon.AuthBaseURLDefault)
	}
	if target.MachineClientID != konnectcommon.MachineClientIDDefault {
		t.Fatalf("MachineClientID = %q, want %q", target.MachineClientID, konnectcommon.MachineClientIDDefault)
	}
}

func TestResolveKonnectTargetFromEnvSupportsTech(t *testing.T) {
	clearKonnectTargetEnv(t)
	t.Setenv(KonnectEnvironmentEnvName, konnectcommon.EnvironmentTech)

	target, err := ResolveKonnectTargetFromEnv()
	if err != nil {
		t.Fatalf("ResolveKonnectTargetFromEnv() error = %v", err)
	}

	if target.BaseURL != konnectcommon.TechBaseURLDefault {
		t.Fatalf("BaseURL = %q, want %q", target.BaseURL, konnectcommon.TechBaseURLDefault)
	}
	if target.BaseAuthURL != konnectcommon.TechGlobalBaseURL {
		t.Fatalf("BaseAuthURL = %q, want %q", target.BaseAuthURL, konnectcommon.TechGlobalBaseURL)
	}
	if target.MachineClientID != konnectcommon.TechMachineClientID {
		t.Fatalf("MachineClientID = %q, want %q", target.MachineClientID, konnectcommon.TechMachineClientID)
	}
}

func TestResolveKonnectTargetFromEnvInfersTechFromBaseURL(t *testing.T) {
	clearKonnectTargetEnv(t)
	t.Setenv(KonnectBaseURLEnvName, "https://eu.api.konghq.tech")

	target, err := ResolveKonnectTargetFromEnv()
	if err != nil {
		t.Fatalf("ResolveKonnectTargetFromEnv() error = %v", err)
	}

	if target.BaseURL != "https://eu.api.konghq.tech" {
		t.Fatalf("BaseURL = %q, want explicit override", target.BaseURL)
	}
	if target.BaseAuthURL != konnectcommon.TechGlobalBaseURL {
		t.Fatalf("BaseAuthURL = %q, want %q", target.BaseAuthURL, konnectcommon.TechGlobalBaseURL)
	}
	if target.MachineClientID != konnectcommon.TechMachineClientID {
		t.Fatalf("MachineClientID = %q, want %q", target.MachineClientID, konnectcommon.TechMachineClientID)
	}
}

func TestResolveKonnectTargetFromEnvInfersMachineClientWithExplicitAuthURL(t *testing.T) {
	clearKonnectTargetEnv(t)
	t.Setenv(KonnectBaseURLEnvName, "https://eu.api.konghq.tech")
	t.Setenv(KonnectBaseAuthURLEnvName, "https://global.example.test")

	target, err := ResolveKonnectTargetFromEnv()
	if err != nil {
		t.Fatalf("ResolveKonnectTargetFromEnv() error = %v", err)
	}

	if target.BaseAuthURL != "https://global.example.test" {
		t.Fatalf("BaseAuthURL = %q, want explicit override", target.BaseAuthURL)
	}
	if target.MachineClientID != konnectcommon.TechMachineClientID {
		t.Fatalf("MachineClientID = %q, want %q", target.MachineClientID, konnectcommon.TechMachineClientID)
	}
}

func TestResolveKonnectTargetFromEnvAllowsExplicitOverrides(t *testing.T) {
	clearKonnectTargetEnv(t)
	t.Setenv(KonnectEnvironmentEnvName, konnectcommon.EnvironmentTech)
	t.Setenv(KonnectBaseURLEnvName, "https://regional.example.test")
	t.Setenv(KonnectBaseAuthURLEnvName, "https://global.example.test")
	t.Setenv(KonnectMachineClientIDEnvName, "client-id")

	target, err := ResolveKonnectTargetFromEnv()
	if err != nil {
		t.Fatalf("ResolveKonnectTargetFromEnv() error = %v", err)
	}

	if target.BaseURL != "https://regional.example.test" {
		t.Fatalf("BaseURL = %q, want explicit override", target.BaseURL)
	}
	if target.BaseAuthURL != "https://global.example.test" {
		t.Fatalf("BaseAuthURL = %q, want explicit override", target.BaseAuthURL)
	}
	if target.MachineClientID != "client-id" {
		t.Fatalf("MachineClientID = %q, want explicit override", target.MachineClientID)
	}
}

func TestKonnectBaseURLFromRegionUsesSelectedEnvironment(t *testing.T) {
	clearKonnectTargetEnv(t)
	t.Setenv(KonnectEnvironmentEnvName, konnectcommon.EnvironmentTech)

	got, err := KonnectBaseURLFromRegion("global")
	if err != nil {
		t.Fatalf("KonnectBaseURLFromRegion() error = %v", err)
	}

	if got != konnectcommon.TechGlobalBaseURL {
		t.Fatalf("KonnectBaseURLFromRegion() = %q, want %q", got, konnectcommon.TechGlobalBaseURL)
	}
}
