//go:build e2e

package e2e

import (
	_ "embed"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
	"sigs.k8s.io/yaml"
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

	// Establish step scope: 000-apply
	step, err := harness.NewStep(t, cli, "000-apply")
	if err != nil {
		t.Fatalf("step init failed: %v", err)
	}
	// Materialize manifest into step inputs
	manifestPath := filepath.Join(step.InputsDir, "portal.yaml")
	if werr := os.WriteFile(manifestPath, portalBasicYAML, 0o644); werr != nil {
		t.Fatalf("failed writing manifest: %v", werr)
	}
	// Derive expected fields from the manifest itself
	var m struct {
		Portals []struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
		} `yaml:"portals"`
	}
	if err := yaml.Unmarshal(portalBasicYAML, &m); err != nil {
		t.Fatalf("failed to parse manifest for expectations: %v", err)
	}
	expectedName := ""
	if len(m.Portals) > 0 {
		expectedName = m.Portals[0].Name
	}
	_ = step.SaveJSON("expected.json", map[string]any{"name": expectedName, "description": m.Portals[0].Description})

	// Apply via helper; records observation under the apply command
	applyOut, err := step.Apply(manifestPath)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
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
	targetName := expectedName
	type portal struct {
		Name string `json:"name"`
	}

	var listRes []portal
	ok := retry(5, 1200*time.Millisecond, func() bool {
		listRes = nil
		if err := step.GetAndObserve("portals", &listRes, map[string]any{"name": targetName}); err != nil {
			return false
		}
		for _, p := range listRes {
			if p.Name == targetName {
				return true
			}
		}
		return false
	})
	// Attach observation to the last get portals command
	// Filter to just the target portal for clarity
	var target []portal
	for _, p := range listRes {
		if p.Name == targetName {
			target = append(target, p)
		}
	}
	_ = step.WriteObservation(listRes, target, map[string]any{"name": targetName})
	if !ok {
		t.Fatalf("expected to find portal named %q in list", targetName)
	}
	step.AppendCheck("PASS: 000-apply found portal named %s", targetName)
}

func retry(times int, delay time.Duration, f func() bool) bool {
	if times < 1 {
		times = 1
	}
	if delay <= 0 {
		delay = 500 * time.Millisecond
	}
	next := delay
	for i := 0; i < times; i++ {
		if f() {
			return true
		}
		if i+1 < times {
			time.Sleep(next)
			next *= 2
		}
	}
	return false
}

// no extra helpers
