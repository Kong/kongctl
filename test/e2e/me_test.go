//go:build e2e

package e2e

import (
	"context"
	"os"
	"testing"

	"github.com/kong/kongctl/test/e2e/harness"
)

// Test_GetMe_JSON_UserPAT validates that a user PAT returns profile info.
func Test_GetMe_JSON_UserPAT(t *testing.T) {
	// This test is opt-in. Only run when explicitly enabled.
	if !truthy(os.Getenv("KONGCTL_E2E_RUN_USER_ME")) {
		t.Skip("skipping: KONGCTL_E2E_RUN_USER_ME not enabled")
	}
	harness.RequireBinary(t)
	// Require the user PAT for the default e2e profile.
	_ = harness.RequirePAT(t, "e2e")

	cli, err := harness.NewCLIT(t)
	if err != nil {
		t.Fatalf("harness init failed: %v", err)
	}

	var out struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		// Optional fields are ignored here; presence of ID/Email is enough.
	}

	res, err := cli.RunJSON(context.Background(), &out, "get", "me")
	if err != nil {
		t.Fatalf("command failed: exit=%d stderr=%s err=%v", res.ExitCode, res.Stderr, err)
	}
	if out.ID == "" {
		t.Fatalf("expected non-empty id in response")
	}
	if out.Email == "" {
		t.Fatalf("expected non-empty email in response")
	}
}

// Test_GetMe_JSON_SystemPAT_Denied validates that a system account PAT is denied for /users/me.
// Skips if KONGCTL_E2E_SA_KONNECT_PAT is not set.
func Test_GetMe_JSON_SystemPAT_Denied(t *testing.T) {
	harness.RequireBinary(t)
	sa := os.Getenv("KONGCTL_E2E_SA_KONNECT_PAT")
	if sa == "" {
		t.Skip("skipping: KONGCTL_E2E_SA_KONNECT_PAT not set")
	}

	cli, err := harness.NewCLIT(t)
	if err != nil {
		t.Fatalf("harness init failed: %v", err)
	}
	// Override the PAT for this run to use the SA token under the e2e profile.
	cli.WithEnv(map[string]string{"KONGCTL_E2E_KONNECT_PAT": sa})

	res, err := cli.Run(context.Background(), "get", "me")
	if err == nil {
		t.Fatalf("expected command to fail for system account, got success: stdout=%q", res.Stdout)
	}
	if res.ExitCode == 0 {
		t.Fatalf("expected non-zero exit code for system account")
	}
	// Allow either 403 or a clear permission/forbidden wording in stderr.
	if !(contains(res.Stderr, "403") || containsFold(res.Stderr, "forbidden") || containsFold(res.Stderr, "permission")) {
		t.Fatalf("expected forbidden/permission error, got stderr=%q", res.Stderr)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || containsNext(s, sub)) }
func containsNext(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func containsFold(s, sub string) bool {
	// Simple case-insensitive contains without pulling strings.ToLower for minimal deps
	// This is fine for small strings in tests.
	// Convert both to rune slices may be overkill; loop ascii only.
	n, m := len(s), len(sub)
	if m == 0 {
		return true
	}
	for i := 0; i+m <= n; i++ {
		match := true
		for j := 0; j < m; j++ {
			a := s[i+j]
			b := sub[j]
			if 'A' <= a && a <= 'Z' {
				a = a - 'A' + 'a'
			}
			if 'A' <= b && b <= 'Z' {
				b = b - 'A' + 'a'
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func truthy(v string) bool {
	switch v {
	case "1", "true", "TRUE", "True", "yes", "on", "YES", "On", "Y", "y":
		return true
	default:
		return false
	}
}
