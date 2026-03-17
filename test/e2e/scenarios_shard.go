package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type scenarioShard struct {
	Enabled bool
	Index   int
	Total   int
}

func selectScenarios(scenarios []string, filter string, shard scenarioShard) []string {
	selected := make([]string, 0, len(scenarios))
	for _, p := range scenarios {
		if filter != "" && !scenarioMatches(p, filter) {
			continue
		}
		selected = append(selected, p)
	}

	if filter == "" && shard.Enabled {
		sharded := make([]string, 0, len(selected))
		for i, p := range selected {
			if i%shard.Total == shard.Index {
				sharded = append(sharded, p)
			}
		}
		selected = sharded
	}

	return selected
}

func scenarioMatches(scenarioPath, filter string) bool {
	scenarioPath = filepath.ToSlash(scenarioPath)
	filter = filepath.ToSlash(filter)

	if scenarioPath == filter {
		return true
	}

	scenarioDir := strings.TrimSuffix(normalizeScenarioPath(scenarioPath), "/scenario.yaml")
	filterDir := strings.TrimSuffix(normalizeScenarioPath(filter), "/scenario.yaml")
	return scenarioDir == filterDir
}

func normalizeScenarioPath(path string) string {
	path = filepath.ToSlash(path)
	path = strings.TrimPrefix(path, "scenarios/")
	path = strings.TrimPrefix(path, "test/e2e/scenarios/")
	return path
}

func writeScenarioShardManifest(artifactsDir string, shard scenarioShard, selected []string) error {
	if !shard.Enabled || strings.TrimSpace(artifactsDir) == "" {
		return nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "shard_index=%d\n", shard.Index)
	fmt.Fprintf(&b, "shard_total=%d\n", shard.Total)
	b.WriteString("\n")
	for _, p := range selected {
		fmt.Fprintf(&b, "%s\n", normalizeScenarioPath(p))
	}

	path := filepath.Join(artifactsDir, "assigned-scenarios.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
