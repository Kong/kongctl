//go:build e2e

package e2e

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
	"sigs.k8s.io/yaml"
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

	// Step 000: reset org (captured as a step)
	stepReset, err := harness.NewStep(t, cli, "000-reset_org")
	if err != nil {
		t.Fatalf("step init failed: %v", err)
	}
	if err := stepReset.ResetOrg("before_test"); err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	// Establish step scope: 001-apply and copy embedded fixtures into it
	step, err := harness.NewStep(t, cli, "001-apply")
	if err != nil {
		t.Fatalf("step init failed: %v", err)
	}
	root := step.InputsDir
	materializeFS(t, portalFullFS, "testdata/declarative/portal/full", root)
	// Derive expected fields from portal.yaml
	portalYAML, rErr := os.ReadFile(filepath.Join(root, "portal.yaml"))
	if rErr != nil {
		t.Fatalf("failed reading portal.yaml: %v", rErr)
	}
	var m struct {
		Portals []struct {
			Name string `yaml:"name"`
		} `yaml:"portals"`
	}
	if err := yaml.Unmarshal(portalYAML, &m); err != nil {
		t.Fatalf("failed parsing portal.yaml: %v", err)
	}
	expectedName := ""
	if len(m.Portals) > 0 {
		expectedName = m.Portals[0].Name
	}
	_ = step.SaveJSON("expected.json", map[string]any{"name": expectedName})

	// Apply both portal and APIs files via helper
	applyOut, err := step.Apply(filepath.Join(root, "portal.yaml"), filepath.Join(root, "apis.yaml"))
	if err != nil {
		t.Fatalf("apply failed: %v", err)
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
		if err := step.GetAndObserve("portals", &portals, map[string]any{"name": expectedName}); err != nil {
			return false
		}
		for _, p := range portals {
			if p.Name == expectedName {
				return true
			}
		}
		return false
	})
	// Observation for portals: include full list and target subset
	var targetPortals []portal
	for _, p := range portals {
		if p.Name == expectedName {
			targetPortals = append(targetPortals, p)
		}
	}
	_ = step.WriteObservation(portals, targetPortals, map[string]any{"name": expectedName})
	if !ok {
		t.Fatalf("expected to find portal named %q", expectedName)
	}

	// Verify at least one API resource exists
	type api struct {
		Name string `json:"name"`
	}
	var apis []api
	ok = retry(6, 1500*time.Millisecond, func() bool {
		apis = nil
		if err := step.GetAndObserve("apis", &apis, map[string]any{"min_count": 1}); err != nil {
			return false
		}
		return len(apis) >= 1
	})
	// Observation for apis: assertion is len>=1, keep full list as target for traceability
	_ = step.WriteObservation(apis, apis, map[string]any{"min_count": 1})
	if !ok {
		t.Fatalf("expected to find at least one API after apply")
	}
	step.AppendCheck("PASS: 001-apply found portal and at least one API")
}
