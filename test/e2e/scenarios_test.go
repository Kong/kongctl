//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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

	sort.Strings(scenarios)

	// Optional filter: KONGCTL_E2E_SCENARIO exact match on scenario directory.
	// When a specific scenario is selected, bypass sharding to keep local
	// iteration predictable.
	filt := strings.TrimSpace(os.Getenv("KONGCTL_E2E_SCENARIO"))
	shard := scenarioShard{}
	if filt == "" {
		var err error
		shard, err = loadScenarioShard()
		if err != nil {
			t.Fatalf("invalid e2e shard configuration: %v", err)
		}
	}

	selected := make([]string, 0, len(scenarios))
	for _, p := range scenarios {
		if filt != "" && !scenarioMatches(p, filt) {
			continue
		}
		selected = append(selected, p)
	}

	if filt == "" && shard.Enabled {
		sharded := make([]string, 0, len(selected))
		for i, p := range selected {
			if i%shard.Total == shard.Index {
				sharded = append(sharded, p)
			}
		}
		selected = sharded
	}

	if len(selected) == 0 {
		switch {
		case filt != "":
			t.Skipf("no scenarios matched filter %q", filt)
		case shard.Enabled:
			t.Skipf("no scenarios assigned to shard %d/%d", shard.Index, shard.Total)
		default:
			t.Skip("no scenarios found")
		}
		return
	}

	for _, p := range selected {
		p := p
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

type scenarioShard struct {
	Enabled bool
	Index   int
	Total   int
}

func loadScenarioShard() (scenarioShard, error) {
	indexRaw := strings.TrimSpace(os.Getenv("KONGCTL_E2E_SHARD_INDEX"))
	totalRaw := strings.TrimSpace(os.Getenv("KONGCTL_E2E_SHARD_TOTAL"))
	if indexRaw == "" && totalRaw == "" {
		return scenarioShard{}, nil
	}
	if indexRaw == "" || totalRaw == "" {
		return scenarioShard{}, fmt.Errorf("both KONGCTL_E2E_SHARD_INDEX and KONGCTL_E2E_SHARD_TOTAL must be set")
	}

	index, err := strconv.Atoi(indexRaw)
	if err != nil {
		return scenarioShard{}, fmt.Errorf("parse KONGCTL_E2E_SHARD_INDEX: %w", err)
	}
	total, err := strconv.Atoi(totalRaw)
	if err != nil {
		return scenarioShard{}, fmt.Errorf("parse KONGCTL_E2E_SHARD_TOTAL: %w", err)
	}
	if total < 1 {
		return scenarioShard{}, fmt.Errorf("KONGCTL_E2E_SHARD_TOTAL must be >= 1")
	}
	if index < 0 || index >= total {
		return scenarioShard{}, fmt.Errorf(
			"KONGCTL_E2E_SHARD_INDEX must be between 0 and %d inclusive",
			total-1,
		)
	}

	return scenarioShard{
		Enabled: true,
		Index:   index,
		Total:   total,
	}, nil
}
