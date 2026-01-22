//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kong/kongctl/test/e2e/harness/scenario"
)

// Test_Scenarios discovers and executes scenario.yaml files under test/e2e/scenarios.
func Test_Scenarios(t *testing.T) {
	// Discover scenario.yaml files by walking under known roots relative to this package
	var scenarios []string
	roots := []string{"scenarios", "test/e2e/scenarios"}
	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			continue
		}
		_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info != nil && !info.IsDir() && info.Name() == "scenario.yaml" {
				scenarios = append(scenarios, p)
			}
			return nil
		})
	}
	if len(scenarios) == 0 {
		t.Skip("no scenarios found")
		return
	}
	// Optional filter: KONGCTL_E2E_SCENARIO exact match on scenario directory
	filt := os.Getenv("KONGCTL_E2E_SCENARIO")
	for _, p := range scenarios {
		p := p
		if filt != "" && !scenarioMatches(p, filt) {
			continue
		}
		t.Run(p, func(t *testing.T) {
			if err := scenario.Run(t, p); err != nil {
				t.Fatalf("scenario failed: %v", err)
			}
		})
	}
}

// scenarioMatches returns true if scenarioPath matches the filter exactly.
// The filter can be specified as:
//   - "portal/email" (scenario directory relative to scenarios/)
//   - "scenarios/portal/email" (with scenarios/ prefix)
//   - "scenarios/portal/email/scenario.yaml" (full path)
func scenarioMatches(scenarioPath, filter string) bool {
	if filter == "" {
		return true
	}
	// Normalize paths to forward slashes for cross-platform compatibility
	// (filepath.Walk returns backslashes on Windows)
	scenarioPath = filepath.ToSlash(scenarioPath)
	filter = filepath.ToSlash(filter)

	// Exact match of full path
	if scenarioPath == filter {
		return true
	}
	// Normalize: extract scenario directory from the path
	// e.g., "scenarios/portal/email/scenario.yaml" -> "portal/email"
	scenarioDir := strings.TrimSuffix(scenarioPath, "/scenario.yaml")
	scenarioDir = strings.TrimPrefix(scenarioDir, "scenarios/")
	scenarioDir = strings.TrimPrefix(scenarioDir, "test/e2e/scenarios/")

	// Normalize the filter similarly
	normFilter := strings.TrimSuffix(filter, "/scenario.yaml")
	normFilter = strings.TrimPrefix(normFilter, "scenarios/")
	normFilter = strings.TrimPrefix(normFilter, "test/e2e/scenarios/")

	return scenarioDir == normFilter
}
