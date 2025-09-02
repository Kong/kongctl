//go:build e2e

package harness

import (
	"fmt"
	"os"
	"testing"
)

// RequireBinary verifies the test binary is available; fails the test if not.
func RequireBinary(t *testing.T) string {
	t.Helper()
	bin, err := BinPath()
	if err != nil {
		t.Fatalf("failed to prepare kongctl binary: %v", err)
	}
	Debugf("RequireBinary: bin=%s", bin)
	return bin
}

// RequirePAT ensures the PAT env for the given profile is set. Skips the test if missing.
// Pattern: KONGCTL_<PROFILE>_KONNECT_PAT
func RequirePAT(t *testing.T, profile string) string {
	t.Helper()
	envName := fmt.Sprintf("KONGCTL_%s_KONNECT_PAT", upper(profile))
	val := os.Getenv(envName)
	if val == "" {
		t.Skipf("skipping: %s not set for e2e", envName)
	}
	Debugf("RequirePAT: %s is set", envName)
	return val
}

func upper(s string) string {
	// local helper; avoid pulling strings for 1 call-site
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c = c - 'a' + 'A'
		}
		b[i] = c
	}
	return string(b)
}
