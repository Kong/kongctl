//go:build e2e

package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteProfileConfigIncludesHTTPSettings(t *testing.T) {
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
}
