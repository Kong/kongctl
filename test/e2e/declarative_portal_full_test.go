//go:build e2e

package e2e

import (
	"context"
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
)

// Embed the full portal example testdata tree
//
//go:embed testdata/declarative/portal/full/**
var portalFullFS embed.FS

// materializeFS writes an embedded filesystem subtree into destDir preserving structure.
func materializeFS(t *testing.T, src fs.FS, subtree, destDir string) {
	t.Helper()
	err := fs.WalkDir(src, subtree, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(subtree, path)
		outPath := filepath.Join(destDir, rel)
		if d.IsDir() {
			return os.MkdirAll(outPath, 0o755)
		}
		b, readErr := fs.ReadFile(src, path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(outPath, b, 0o644)
	})
	if err != nil {
		t.Fatalf("failed to materialize fixtures: %v", err)
	}
}

func Test_Declarative_Apply_Portal_Full_JSON(t *testing.T) {
	harness.RequireBinary(t)
	_ = harness.RequirePAT(t, "e2e")

	cli, err := harness.NewCLIT(t)
	if err != nil {
		t.Fatalf("harness init failed: %v", err)
	}

	// Prepare inputs directory and copy embedded fixtures into it
	inputsDir, err := cli.TempWorkdir()
	if err != nil {
		t.Fatalf("inputs dir init failed: %v", err)
	}
	// Create a subfolder to keep the example self-contained
	root := filepath.Join(inputsDir, "portal_full")
	if mkErr := os.MkdirAll(root, 0o755); mkErr != nil {
		t.Fatalf("mkdir failed: %v", mkErr)
	}
	materializeFS(t, portalFullFS, "testdata/declarative/portal/full", root)

	// Apply both portal and APIs files
	var applyOut struct {
		Summary struct {
			Applied      int    `json:"applied"`
			Failed       int    `json:"failed"`
			Status       string `json:"status"`
			TotalChanges int    `json:"total_changes"`
		} `json:"summary"`
	}
	res, err := cli.RunJSON(context.Background(), &applyOut,
		"apply",
		"-f", filepath.Join(root, "portal.yaml"),
		"-f", filepath.Join(root, "apis.yaml"),
		"--auto-approve",
	)
	if err != nil {
		t.Fatalf("apply failed: exit=%d stderr=%s err=%v", res.ExitCode, res.Stderr, err)
	}
	if applyOut.Summary.Failed != 0 {
		t.Fatalf("expected no failed changes, got %d", applyOut.Summary.Failed)
	}
	if applyOut.Summary.Status == "" {
		t.Fatalf("expected non-empty summary status")
	}

	// Verify portal exists
	type portal struct {
		Name string `json:"name"`
	}
	var portals []portal
	ok := retry(6, 1500*time.Millisecond, func() bool {
		portals = nil
		_, e := cli.RunJSON(context.Background(), &portals, "get", "portals")
		if e != nil {
			return false
		}
		for _, p := range portals {
			if p.Name == "My First Portal" {
				return true
			}
		}
		return false
	})
	if !ok {
		t.Fatalf("expected to find portal named %q", "My First Portal")
	}

	// Verify at least one API resource exists
	type api struct {
		Name string `json:"name"`
	}
	var apis []api
	ok = retry(6, 1500*time.Millisecond, func() bool {
		apis = nil
		_, e := cli.RunJSON(context.Background(), &apis, "get", "apis")
		if e != nil {
			return false
		}
		return len(apis) >= 1
	})
	if !ok {
		t.Fatalf("expected to find at least one API after apply")
	}
}
