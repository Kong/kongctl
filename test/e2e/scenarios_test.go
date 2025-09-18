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
	// Optional filter: KONGCTL_E2E_SCENARIO substring/path
	filt := os.Getenv("KONGCTL_E2E_SCENARIO")
	for _, p := range scenarios {
		p := p
		if filt != "" && !strings.Contains(p, filt) && p != filt {
			continue
		}
		t.Run(p, func(t *testing.T) {
			if err := scenario.Run(t, p); err != nil {
				t.Fatalf("scenario failed: %v", err)
			}
		})
	}
}
