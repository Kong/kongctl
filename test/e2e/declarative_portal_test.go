//go:build e2e

package e2e

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
)

//go:embed testdata/declarative/portal/basic/portal.yaml
var portalBasicYAML []byte

// Test_Declarative_Apply_Portal_Basic_JSON applies the basic portal example and asserts success.
func Test_Declarative_Apply_Portal_Basic_JSON(t *testing.T) {
	harness.RequireBinary(t)

	// Require a PAT for the e2e profile; skip if not set.
	_ = harness.RequirePAT(t, "e2e")

	cli, err := harness.NewCLIT(t)
	if err != nil {
		t.Fatalf("harness init failed: %v", err)
	}
	// Token is provided via environment per-profile; no override required.

	// Apply basic portal configuration (JSON output, auto-approve).
	var applyOut struct {
		Execution struct {
			DryRun bool `json:"dry_run"`
		} `json:"execution"`
		Summary struct {
			Applied      int    `json:"applied"`
			Failed       int    `json:"failed"`
			Status       string `json:"status"`
			TotalChanges int    `json:"total_changes"`
		} `json:"summary"`
	}

	// Materialize the embedded manifest into the test workdir for reproducibility
	workdir, err := cli.TempWorkdir()
	if err != nil {
		t.Fatalf("workdir init failed: %v", err)
	}
	manifestPath := filepath.Join(workdir, "portal.yaml")
	if werr := os.WriteFile(manifestPath, portalBasicYAML, 0o644); werr != nil {
		t.Fatalf("failed writing manifest: %v", werr)
	}

	res, err := cli.RunJSON(context.Background(), &applyOut,
		"apply", "-f", manifestPath, "--auto-approve",
	)
	if err != nil {
		t.Fatalf("apply failed: exit=%d stderr=%s err=%v", res.ExitCode, res.Stderr, err)
	}
	if applyOut.Execution.DryRun {
		t.Fatalf("expected dry_run=false")
	}
	if applyOut.Summary.Failed != 0 {
		t.Fatalf("expected no failed changes, got %d", applyOut.Summary.Failed)
	}
	if applyOut.Summary.Status == "" {
		t.Fatalf("expected summary.status to be set")
	}

	// Verify the portal exists by listing portals and searching by name.
	// Allow for eventual consistency with small retries.
	const targetName = "My Simple Portal"
	type portal struct {
		Name string `json:"name"`
	}

	var listRes []portal
	ok := retry(5, 1200*time.Millisecond, func() bool {
		listRes = nil
		rr, e := cli.RunJSON(context.Background(), &listRes, "get", "portals")
		if e != nil {
			// log and retry
			_ = rr
			return false
		}
		for _, p := range listRes {
			if p.Name == targetName {
				return true
			}
		}
		return false
	})
	if !ok {
		t.Fatalf("expected to find portal named %q in list", targetName)
	}
}

func retry(times int, delay time.Duration, f func() bool) bool {
	for i := 0; i < times; i++ {
		if f() {
			return true
		}
		time.Sleep(delay)
	}
	return false
}

// no extra helpers
