//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
	"sigs.k8s.io/yaml"
)

type portal struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// apply summary shape is provided by harness.Step.Apply

// Test_Declarative_Portal_Edit_Steps validates a simple two-step scenario:
// 1) create a portal with description v1; 2) modify the description to v2.
func Test_Declarative_Portal_Edit_Steps(t *testing.T) {
	harness.RequireBinary(t)
	_ = harness.RequirePAT(t, "e2e")

	cli, err := harness.NewCLIT(t)
	if err != nil {
		t.Fatalf("harness init failed: %v", err)
	}

	// Step 000: init (v1)
	step1, err := harness.NewStep(t, cli, "000-init")
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
	// Parse expected values for step1 from manifest
	var m1 struct {
		Portals []struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
		} `yaml:"portals"`
	}
	if err := yaml.Unmarshal(b1, &m1); err != nil {
		t.Fatalf("parse manifest step1 failed: %v", err)
	}
	expectedName1, expectedDesc1 := "", ""
	if len(m1.Portals) > 0 {
		expectedName1 = m1.Portals[0].Name
		expectedDesc1 = m1.Portals[0].Description
	}
	_ = step1.SaveJSON("expected.json", map[string]any{"name": expectedName1, "description": expectedDesc1})

	// Apply v1 via helper (records apply_summary observation)
	applyOut, err := step1.Apply(dst1)
	if err != nil {
		t.Fatalf("apply v1 failed: %v", err)
	}
	if applyOut.Summary.Failed != 0 {
		t.Fatalf("apply v1: expected failed=0, got %d", applyOut.Summary.Failed)
	}

	// Validate state via get portals, filter by name
	var portals []portal
	ok := retry(5, 1200*time.Millisecond, func() bool {
		portals = nil
		if err := step1.GetAndObserve("portals", &portals, map[string]any{"name": expectedName1, "description": expectedDesc1}); err != nil {
			return false
		}
		return findPortalWithDescription(portals, expectedName1, expectedDesc1)
	})
	// Observation for portals in step1
	var target1 []portal
	for _, p := range portals {
		if p.Name == expectedName1 {
			if (p.Description == nil && expectedDesc1 == "") || (p.Description != nil && *p.Description == expectedDesc1) {
				target1 = append(target1, p)
			}
		}
	}
	_ = step1.WriteObservation(portals, target1, map[string]any{"name": expectedName1, "description": expectedDesc1})
	if !ok {
		t.Fatalf("expected to find portal name=E2E Portal Edit with description v1")
	}

	step1.AppendCheck("PASS: init step found portal with description v1")

	// Idempotency: re-apply v1 should result in 0 changes (records another apply_summary observation)
	applyOut, err = step1.Apply(dst1)
	if err != nil {
		t.Fatalf("idempotent apply v1 failed: %v", err)
	}
	if applyOut.Summary.TotalChanges != 0 {
		t.Fatalf("expected idempotent apply total_changes=0, got %d", applyOut.Summary.TotalChanges)
	}
	step1.AppendCheck("PASS: init step idempotent apply has 0 changes")

	// Step 001: modify (v2)
	step2, err := harness.NewStep(t, cli, "001-modify")
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
	// Parse expected values for step2 from manifest
	var m2 struct {
		Portals []struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
		} `yaml:"portals"`
	}
	if err := yaml.Unmarshal(b2, &m2); err != nil {
		t.Fatalf("parse manifest step2 failed: %v", err)
	}
	expectedName2, expectedDesc2 := "", ""
	if len(m2.Portals) > 0 {
		expectedName2 = m2.Portals[0].Name
		expectedDesc2 = m2.Portals[0].Description
	}
	_ = step2.SaveJSON("expected.json", map[string]any{"name": expectedName2, "description": expectedDesc2})

	applyOut, err = step2.Apply(dst2)
	if err != nil {
		t.Fatalf("apply v2 failed: %v", err)
	}
	if applyOut.Summary.Failed != 0 {
		t.Fatalf("apply v2: expected failed=0, got %d", applyOut.Summary.Failed)
	}

	portals = nil
	ok = retry(5, 1200*time.Millisecond, func() bool {
		portals = nil
		if err := step2.GetAndObserve("portals", &portals, map[string]any{"name": expectedName2, "description": expectedDesc2}); err != nil {
			return false
		}
		return findPortalWithDescription(portals, expectedName2, expectedDesc2)
	})
	// Observation for portals in step2
	var target2 []portal
	for _, p := range portals {
		if p.Name == expectedName2 {
			if (p.Description == nil && expectedDesc2 == "") || (p.Description != nil && *p.Description == expectedDesc2) {
				target2 = append(target2, p)
			}
		}
	}
	_ = step2.WriteObservation(portals, target2, map[string]any{"name": expectedName2, "description": expectedDesc2})
	if !ok {
		t.Fatalf("expected to find portal name=E2E Portal Edit with description v2")
	}
	step2.AppendCheck("PASS: modify step found portal with description v2")

	// Idempotency: re-apply v2 should result in 0 changes (records another apply_summary observation)
	applyOut, err = step2.Apply(dst2)
	if err != nil {
		t.Fatalf("idempotent apply v2 failed: %v", err)
	}
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
