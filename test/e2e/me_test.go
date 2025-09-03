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

func truthy(v string) bool {
	switch v {
	case "1", "true", "TRUE", "True", "yes", "on", "YES", "On", "Y", "y":
		return true
	default:
		return false
	}
}
