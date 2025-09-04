//go:build e2e

package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
)

type portal struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// applyResp matches the JSON shape of apply output used by assertions.
type applyResp struct {
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

// Test_Declarative_Portal_Edit_Steps validates a simple two-step scenario:
// 1) create a portal with description v1; 2) modify the description to v2.
func Test_Declarative_Portal_Edit_Steps(t *testing.T) {
	harness.RequireBinary(t)
	_ = harness.RequirePAT(t, "e2e")

	cli, err := harness.NewCLIT(t)
	if err != nil {
		t.Fatalf("harness init failed: %v", err)
	}

	// Step 001: init (v1)
	step1, err := harness.NewStep(t, cli, "001-init")
	if err != nil {
		t.Fatalf("step init failed: %v", err)
	}

	// Prepare inputs
	src1 := filepath.Join("testdata", "scenarios", "declarative", "portal-edit", "001-init", "portal.yaml")
	dst1 := filepath.Join(step1.InputsDir, "portal.yaml")
	b1, rErr := os.ReadFile(src1)
	if rErr != nil {
		t.Fatalf("read manifest failed: %v", rErr)
	}
	if wErr := os.WriteFile(dst1, b1, 0o644); wErr != nil {
		t.Fatalf("write manifest failed: %v", wErr)
	}

	// Apply v1
	var applyOut applyResp
	res, err := cli.RunJSON(context.Background(), &applyOut, "apply", "-f", dst1, "--auto-approve")
	if err != nil {
		t.Fatalf("apply v1 failed: exit=%d stderr=%s err=%v", res.ExitCode, res.Stderr, err)
	}
	_ = step1.SaveJSON("apply.json", applyOut)
	if applyOut.Summary.Failed != 0 {
		t.Fatalf("apply v1: expected failed=0, got %d", applyOut.Summary.Failed)
	}

	// Validate state via get portals, filter by name
	var portals []portal
	ok := retry(5, 1200*time.Millisecond, func() bool {
		portals = nil
		rr, ge := cli.RunJSON(context.Background(), &portals, "get", "portals")
		if ge != nil {
			_ = rr // silence linter; retry on error
			return false
		}
		return findPortalWithDescription(portals, "E2E Portal Edit", "v1")
	})
	_ = step1.SaveJSON(filepath.Join("snapshots", "portals.json"), portals)
	if !ok {
		t.Fatalf("expected to find portal name=E2E Portal Edit with description v1")
	}

	step1.AppendCheck("PASS: init step found portal with description v1")

	// Idempotency: re-apply v1 should result in 0 changes
	applyOut = applyResp{}
	res, err = cli.RunJSON(context.Background(), &applyOut, "apply", "-f", dst1, "--auto-approve")
	if err != nil {
		t.Fatalf("idempotent apply v1 failed: exit=%d stderr=%s err=%v", res.ExitCode, res.Stderr, err)
	}
	_ = step1.SaveJSON("apply-idempotent.json", applyOut)
	if applyOut.Summary.TotalChanges != 0 {
		t.Fatalf("expected idempotent apply total_changes=0, got %d", applyOut.Summary.TotalChanges)
	}
	step1.AppendCheck("PASS: init step idempotent apply has 0 changes")

	// Step 002: modify (v2)
	step2, err := harness.NewStep(t, cli, "002-modify")
	if err != nil {
		t.Fatalf("step init failed: %v", err)
	}

	src2 := filepath.Join("testdata", "scenarios", "declarative", "portal-edit", "002-modify", "portal.yaml")
	dst2 := filepath.Join(step2.InputsDir, "portal.yaml")
	b2, r2 := os.ReadFile(src2)
	if r2 != nil {
		t.Fatalf("read manifest failed: %v", r2)
	}
	if w2 := os.WriteFile(dst2, b2, 0o644); w2 != nil {
		t.Fatalf("write manifest failed: %v", w2)
	}

	res, err = cli.RunJSON(context.Background(), &applyOut, "apply", "-f", dst2, "--auto-approve")
	if err != nil {
		t.Fatalf("apply v2 failed: exit=%d stderr=%s err=%v", res.ExitCode, res.Stderr, err)
	}
	_ = step2.SaveJSON("apply.json", applyOut)
	if applyOut.Summary.Failed != 0 {
		t.Fatalf("apply v2: expected failed=0, got %d", applyOut.Summary.Failed)
	}

	portals = nil
	ok = retry(5, 1200*time.Millisecond, func() bool {
		portals = nil
		rr, ge := cli.RunJSON(context.Background(), &portals, "get", "portals")
		if ge != nil {
			_ = rr
			return false
		}
		return findPortalWithDescription(portals, "E2E Portal Edit", "v2")
	})
	_ = step2.SaveJSON(filepath.Join("snapshots", "portals.json"), portals)
	if !ok {
		t.Fatalf("expected to find portal name=E2E Portal Edit with description v2")
	}
	step2.AppendCheck("PASS: modify step found portal with description v2")

	// Idempotency: re-apply v2 should result in 0 changes
	applyOut = applyResp{}
	res, err = cli.RunJSON(context.Background(), &applyOut, "apply", "-f", dst2, "--auto-approve")
	if err != nil {
		t.Fatalf("idempotent apply v2 failed: exit=%d stderr=%s err=%v", res.ExitCode, res.Stderr, err)
	}
	_ = step2.SaveJSON("apply-idempotent.json", applyOut)
	if applyOut.Summary.TotalChanges != 0 {
		t.Fatalf("expected idempotent apply total_changes=0, got %d", applyOut.Summary.TotalChanges)
	}
	step2.AppendCheck("PASS: modify step idempotent apply has 0 changes")
}

func findPortalWithDescription(portals []portal, name, desc string) bool {
	for _, p := range portals {
		if p.Name == name {
			if p.Description == nil {
				return desc == ""
			}
			return *p.Description == desc
		}
	}
	return false
}

// retry is available in other e2e files; do not redefine here.
