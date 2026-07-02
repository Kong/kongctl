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

func TestEventGatewayChildEndpointExpansion(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		want     string
	}{
		{
			name:     "backend cluster",
			resource: "event-gateway-backend-cluster",
			want:     "/v1/event-gateways/gateway%2F123/backend-clusters",
		},
		{
			name:     "virtual cluster",
			resource: "event-gateway-virtual-cluster",
			want:     "/v1/event-gateways/gateway%2F123/virtual-clusters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint, ok := createResourceEndpoints[tt.resource]
			if !ok {
				t.Fatalf("%s endpoint missing", tt.resource)
			}

			path, err := endpoint.expandPath(map[string]string{"gatewayId": "gateway/123"})
			if err != nil {
				t.Fatalf("expandPath() error = %v", err)
			}

			if path != tt.want {
				t.Fatalf("path = %q, want %q", path, tt.want)
			}
		})
	}
}

func TestRedactSensitiveJSONRedactsTokenFields(t *testing.T) {
	body := []byte(
		`{"id":"token-id","token":"secret-token","nested":{"api_token":"nested-secret"},"items":[{"secret":"value"}]}`,
	)

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

func TestRedactSensitiveCommandArtifactStringRedactsCreateTokenJSON(t *testing.T) {
	got := RedactSensitiveCommandArtifactString(
		[]string{"create", "pat", "-o", "json"},
		`{"id":"token-id","token":"secret-token"}`,
	)

	if strings.Contains(got, "secret-token") {
		t.Fatalf("redacted output leaked token: %s", got)
	}
	if !strings.Contains(got, `"token": "***"`) {
		t.Fatalf("redacted output missing token placeholder: %s", got)
	}
	if !strings.Contains(got, "token-id") {
		t.Fatalf("redacted output removed non-sensitive id: %s", got)
	}
}

func TestRedactSensitiveCommandArtifactStringRedactsRawTokenOutput(t *testing.T) {
	got := RedactSensitiveCommandArtifactString(
		[]string{"create", "pat", "-o", "token"},
		"kpat_secret_value\n",
	)

	if strings.Contains(got, "kpat_secret_value") {
		t.Fatalf("redacted output leaked token: %s", got)
	}
	if got != "***\n" {
		t.Fatalf("redacted output = %q, want placeholder", got)
	}
}

func TestRedactSensitiveCommandArtifactStringRedactsExplicitKonnectRawTokenOutput(t *testing.T) {
	got := RedactSensitiveCommandArtifactString(
		[]string{"konnect", "create", "pat", "-o", "token"},
		"kpat_secret_value\n",
	)

	if strings.Contains(got, "kpat_secret_value") {
		t.Fatalf("redacted output leaked token: %s", got)
	}
	if got != "***\n" {
		t.Fatalf("redacted output = %q, want placeholder", got)
	}
}

func TestRedactSensitiveCommandArtifactStringRedactsEnvOutput(t *testing.T) {
	got := RedactSensitiveCommandArtifactString(
		[]string{"create", "spat", "-o", "env"},
		"export KONGCTL_E2E_KONNECT_PAT='secret-token'\n",
	)

	if strings.Contains(got, "secret-token") {
		t.Fatalf("redacted output leaked token: %s", got)
	}
	if !strings.Contains(got, "export KONGCTL_E2E_KONNECT_PAT=***") {
		t.Fatalf("redacted output missing redacted export: %s", got)
	}
}

func TestRedactSensitiveCommandArtifactValueRedactsRawStdout(t *testing.T) {
	got := RedactSensitiveCommandArtifactValue(
		[]string{"create", "pat", "-o", "token"},
		map[string]any{"stdout": "kpat_secret_value\n"},
	).(map[string]any)

	if got["stdout"] != "***\n" {
		t.Fatalf("stdout = %q, want redacted placeholder", got["stdout"])
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
