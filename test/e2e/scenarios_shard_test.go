package e2e

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSelectScenariosPartitionsWithoutOverlap(t *testing.T) {
	scenarios := []string{
		"test/e2e/scenarios/apis/basic/scenario.yaml",
		"test/e2e/scenarios/apis/nested/scenario.yaml",
		"test/e2e/scenarios/auth/get-me/scenario.yaml",
		"test/e2e/scenarios/portal/basic/scenario.yaml",
		"test/e2e/scenarios/portal/pages/scenario.yaml",
	}

	seen := make(map[string]int, len(scenarios))
	for index := range 3 {
		selected := selectScenarios(scenarios, "", scenarioShard{
			Enabled: true,
			Index:   index,
			Total:   3,
		})
		for _, scenario := range selected {
			seen[scenario]++
		}
	}

	for _, scenario := range scenarios {
		if seen[scenario] != 1 {
			t.Fatalf("scenario %q assigned %d times, want exactly once", scenario, seen[scenario])
		}
	}
}

func TestSelectScenariosAllowsEmptyShards(t *testing.T) {
	scenarios := []string{
		"test/e2e/scenarios/apis/basic/scenario.yaml",
		"test/e2e/scenarios/portal/basic/scenario.yaml",
	}

	counts := make([]int, 4)
	seen := make(map[string]int, len(scenarios))
	for index := range 4 {
		selected := selectScenarios(scenarios, "", scenarioShard{
			Enabled: true,
			Index:   index,
			Total:   4,
		})
		counts[index] = len(selected)
		for _, scenario := range selected {
			seen[scenario]++
		}
	}

	if !slices.Equal(counts, []int{1, 1, 0, 0}) {
		t.Fatalf("unexpected shard counts: got %v", counts)
	}
	for _, scenario := range scenarios {
		if seen[scenario] != 1 {
			t.Fatalf("scenario %q assigned %d times, want exactly once", scenario, seen[scenario])
		}
	}
}

func TestSelectScenariosFilterBypassesSharding(t *testing.T) {
	scenarios := []string{
		"test/e2e/scenarios/apis/basic/scenario.yaml",
		"test/e2e/scenarios/portal/pages/scenario.yaml",
		"test/e2e/scenarios/smoke/version/scenario.yaml",
	}

	selected := selectScenarios(scenarios, "portal/pages", scenarioShard{
		Enabled: true,
		Index:   1,
		Total:   3,
	})

	want := []string{"test/e2e/scenarios/portal/pages/scenario.yaml"}
	if !slices.Equal(selected, want) {
		t.Fatalf("unexpected filtered scenarios: got %v want %v", selected, want)
	}
}

func TestWriteScenarioShardManifest(t *testing.T) {
	dir := t.TempDir()
	selected := []string{
		"test/e2e/scenarios/apis/basic/scenario.yaml",
		"test/e2e/scenarios/portal/pages/scenario.yaml",
	}

	err := writeScenarioShardManifest(dir, scenarioShard{
		Enabled: true,
		Index:   2,
		Total:   4,
	}, selected)
	if err != nil {
		t.Fatalf("writeScenarioShardManifest() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "assigned-scenarios.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	want := strings.Join([]string{
		"shard_index=2",
		"shard_total=4",
		"",
		"apis/basic/scenario.yaml",
		"portal/pages/scenario.yaml",
		"",
	}, "\n")
	if string(data) != want {
		t.Fatalf("unexpected manifest contents: got %q want %q", string(data), want)
	}
}
