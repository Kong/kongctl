//go:build e2e

package harness

import (
	"os"
	"strings"
	"testing"
)

func TestSystemAccountAccessTokenEndpointExpansion(t *testing.T) {
	endpoint, ok := createResourceEndpoints["system-account-access-token"]
	if !ok {
		t.Fatalf("system-account-access-token endpoint missing")
	}

	path, err := endpoint.expandPath(map[string]string{"systemAccountId": "system/account 123"})
	if err != nil {
		t.Fatalf("expandPath() error = %v", err)
	}

	want := "/v3/system-accounts/system%2Faccount%20123/access-tokens"
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if !endpoint.UseGlobal {
		t.Fatalf("system account access token endpoint must use global API")
	}
}

func TestRedactSensitiveJSONRedactsTokenFields(t *testing.T) {
	body := []byte(`{"id":"token-id","token":"secret-token","nested":{"api_token":"nested-secret"},"items":[{"secret":"value"}]}`)

	got := string(redactSensitiveJSONBytes(body))
	for _, leaked := range []string{"secret-token", "nested-secret", `"secret": "value"`} {
		if strings.Contains(got, leaked) {
			t.Fatalf("redacted body leaked %q: %s", leaked, got)
		}
	}
	for _, want := range []string{`"token": "***"`, `"api_token": "***"`, `"secret": "***"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("redacted body missing %q: %s", want, got)
		}
	}
}

func TestAppendCheckRedactsSensitiveAssignments(t *testing.T) {
	step := &Step{ChecksPath: t.TempDir() + "/checks.log"}

	step.AppendCheck("SET VAR: apiToken=%s other=value", "secret-token")

	data := mustReadFile(t, step.ChecksPath)
	if strings.Contains(data, "secret-token") {
		t.Fatalf("checks.log leaked token: %s", data)
	}
	if !strings.Contains(data, "apiToken=***") {
		t.Fatalf("checks.log missing redacted token: %s", data)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(data)
}
