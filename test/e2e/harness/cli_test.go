//go:build e2e

package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
)

func TestWriteProfileConfigIncludesHTTPSettings(t *testing.T) {
	clearKonnectTargetEnv(t)
	t.Setenv("KONGCTL_E2E_HTTP_TIMEOUT", "13s")
	t.Setenv("KONGCTL_E2E_HTTP_TCP_USER_TIMEOUT", "45s")
	t.Setenv("KONGCTL_E2E_HTTP_DISABLE_KEEPALIVES", "true")
	t.Setenv("KONGCTL_E2E_HTTP_RECYCLE_CONNECTIONS_ON_ERROR", "1")

	cfgDir := t.TempDir()
	if err := writeProfileConfig(cfgDir, "e2e", "json", "debug"); err != nil {
		t.Fatalf("writeProfileConfig() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfgDir, "kongctl", "config.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "http-timeout: 13s") {
		t.Fatalf("config missing http-timeout: %s", content)
	}
	if !strings.Contains(content, "http-tcp-user-timeout: 45s") {
		t.Fatalf("config missing http-tcp-user-timeout: %s", content)
	}
	if !strings.Contains(content, "http-disable-keepalives: true") {
		t.Fatalf("config missing http-disable-keepalives: %s", content)
	}
	if !strings.Contains(content, "http-recycle-connections-on-error: true") {
		t.Fatalf("config missing http-recycle-connections-on-error: %s", content)
	}
	for _, want := range []string{
		"konnect:",
		"environment: " + konnectcommon.EnvironmentProduction,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("config missing %q: %s", want, content)
		}
	}
	for _, forbidden := range []string{
		"base-url:",
		"base-auth-url:",
		"machine-client-id:",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("config unexpectedly contains %q: %s", forbidden, content)
		}
	}
}

func TestWriteProfileConfigOmitsDisabledHTTPTimeouts(t *testing.T) {
	clearKonnectTargetEnv(t)
	t.Setenv("KONGCTL_E2E_HTTP_TIMEOUT", "0s")
	t.Setenv("KONGCTL_E2E_HTTP_TCP_USER_TIMEOUT", "default")

	cfgDir := t.TempDir()
	if err := writeProfileConfig(cfgDir, "e2e", "json", "debug"); err != nil {
		t.Fatalf("writeProfileConfig() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfgDir, "kongctl", "config.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	content := string(data)
	if strings.Contains(content, "http-timeout:") {
		t.Fatalf("config unexpectedly contains http-timeout: %s", content)
	}
	if strings.Contains(content, "http-tcp-user-timeout:") {
		t.Fatalf("config unexpectedly contains http-tcp-user-timeout: %s", content)
	}
}

func TestWriteProfileConfigIncludesTechKonnectSettings(t *testing.T) {
	clearKonnectTargetEnv(t)
	t.Setenv(KonnectEnvironmentEnvName, konnectcommon.EnvironmentTech)

	cfgDir := t.TempDir()
	if err := writeProfileConfig(cfgDir, "e2e", "json", "debug"); err != nil {
		t.Fatalf("writeProfileConfig() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfgDir, "kongctl", "config.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	content := string(data)
	for _, want := range []string{
		"environment: " + konnectcommon.EnvironmentTech,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("config missing %q: %s", want, content)
		}
	}
	for _, forbidden := range []string{
		"base-url:",
		"base-auth-url:",
		"machine-client-id:",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("config unexpectedly contains %q: %s", forbidden, content)
		}
	}
}

func TestWriteProfileConfigIncludesExplicitKonnectEndpointOverrides(t *testing.T) {
	clearKonnectTargetEnv(t)
	t.Setenv(KonnectEnvironmentEnvName, konnectcommon.EnvironmentTech)
	t.Setenv(KonnectBaseURLEnvName, "https://regional.example.test")
	t.Setenv(KonnectBaseAuthURLEnvName, "https://global.example.test")
	t.Setenv(KonnectMachineClientIDEnvName, "client-id")

	cfgDir := t.TempDir()
	if err := writeProfileConfig(cfgDir, "e2e", "json", "debug"); err != nil {
		t.Fatalf("writeProfileConfig() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfgDir, "kongctl", "config.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	content := string(data)
	for _, want := range []string{
		"environment: " + konnectcommon.EnvironmentTech,
		"base-url: https://regional.example.test",
		"base-auth-url: https://global.example.test",
		"machine-client-id: client-id",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("config missing %q: %s", want, content)
		}
	}
}

func TestWithEnvReplacesExistingKeys(t *testing.T) {
	cli := &CLI{
		Env: []string{
			"EXISTING=old",
			"OTHER=value",
		},
	}

	cli.WithEnv(map[string]string{"EXISTING": "new"})

	var existing []string
	for _, kv := range cli.Env {
		if strings.HasPrefix(kv, "EXISTING=") {
			existing = append(existing, kv)
		}
	}
	if len(existing) != 1 {
		t.Fatalf("EXISTING entries = %v, want one", existing)
	}
	if existing[0] != "EXISTING=new" {
		t.Fatalf("EXISTING entry = %q, want EXISTING=new", existing[0])
	}
}

func TestSupportsHarnessOutputArgSkipsFixedOutputCommands(t *testing.T) {
	tests := [][]string{
		{"dump", "declarative", "--resources=apis"},
		{"plan", "-f", "config.yaml"},
		{"scaffold", "api"},
	}

	for _, args := range tests {
		if supportsHarnessOutputArg(args) {
			t.Fatalf("%s command must not receive harness-managed output flags", args[0])
		}
	}
}

func TestSupportsHarnessOutputArgAllowsOtherCommands(t *testing.T) {
	if !supportsHarnessOutputArg([]string{"apply", "-f", "config.yaml"}) {
		t.Fatalf("apply command should support harness-managed output flags")
	}
}

func TestHasOutputArgRecognizesShortOutputEquals(t *testing.T) {
	if !hasOutputArg([]string{"get", "apis", "-o=json"}) {
		t.Fatalf("expected -o=json to be recognized as an output flag")
	}
}
