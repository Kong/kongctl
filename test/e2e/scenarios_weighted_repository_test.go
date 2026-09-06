//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestWeightedRepositoryPlan validates the real corpus without executing any
// scenario or contacting Konnect. The build job runs this in the compiled binary.
func TestWeightedRepositoryPlan(t *testing.T) {
	root := "scenarios"
	if _, err := os.Stat(root); os.IsNotExist(err) {
		root = "test/e2e/scenarios"
	}
	var paths []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == "scenario.yaml" {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no repository scenarios found")
	}
	slices.Sort(paths)
	pins, err := loadScenarioAssignments(paths)
	if err != nil {
		t.Fatal(err)
	}
	// Derive pin destinations from manifests, not a maintained scenario mapping.
	// Add unpinned destinations to exercise balancing across five organizations.
	orgSet := map[string]bool{}
	for _, pin := range pins {
		orgSet[pin.Environment] = true
	}
	envs := slices.Sorted(maps.Keys(orgSet))
	for i := 0; len(envs) < 5; i++ {
		name := fmt.Sprintf("weighted-test-org-%d", i)
		if !orgSet[name] {
			envs = append(envs, name)
		}
	}
	var reference []byte
	for i, env := range envs {
		dir := t.TempDir()
		cfg := scenarioSelectionConfig{
			Shard:      scenarioShard{Enabled: true, Index: i, Total: len(envs)},
			CurrentEnv: env, AllowedEnvs: envs, Assignments: pins, ValidateEnvs: true, EnforceEnv: true,
		}
		if err := writeWeightedScenarioReport(dir, paths, cfg); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "proposed-scenario-assignments.json"))
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 && string(data) != string(reference) {
			t.Fatal("matrix organizations generated different full-pool plans")
		}
		reference = data
		var report weightedScenarioReport
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		selected, err := selectScenariosWithConfig(paths, cfg)
		if err != nil {
			t.Fatal(err)
		}
		var current []string
		for _, item := range report.Organizations[i].Legacy {
			current = append(current, item.Scenario)
		}
		for j := range selected {
			selected[j] = normalizeScenarioPath(selected[j])
		}
		if !slices.Equal(current, selected) {
			t.Fatalf("report for %s does not match actual selector", env)
		}
		active, allocation, err := selectScenariosForExecution(paths, cfg)
		if err != nil {
			t.Fatal(err)
		}
		var weighted []string
		for _, item := range report.Organizations[i].Weighted {
			weighted = append(weighted, item.Scenario)
		}
		for j := range active {
			active[j] = normalizeScenarioPath(active[j])
		}
		if !slices.Equal(active, weighted) || allocation != report.Allocation {
			t.Fatal("active weighted selection does not match comparison report")
		}
		seen := map[string]bool{}
		var currentTotal, proposedTotal int64
		for _, org := range report.Organizations {
			if i == 0 {
				t.Logf("%s: modulo=%d scenarios/%.2fs, weighted=%d scenarios/%.2fs",
					org.Environment, len(org.Legacy), float64(org.LegacyMS)/1000,
					len(org.Weighted), float64(org.WeightedMS)/1000)
			}
			currentTotal += org.LegacyMS
			proposedTotal += org.WeightedMS
			for _, item := range org.Weighted {
				if seen[item.Scenario] {
					t.Fatalf("duplicate proposed scenario %s", item.Scenario)
				}
				seen[item.Scenario] = true
				if pin := pins[item.Scenario].Environment; pin != "" && pin != org.Environment {
					t.Fatalf("scenario %s moved away from pin %s", item.Scenario, pin)
				}
			}
		}
		if len(seen) != len(paths) || currentTotal != proposedTotal {
			t.Fatal("proposed assignment changed coverage or total weight")
		}
	}
	t.Logf("validated %d scenarios and %d pins across %d organizations", len(paths), len(pins), len(envs))
}
