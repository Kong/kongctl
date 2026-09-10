package e2e

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"slices"
)

// Routing is explicit and whole-scenario. No scenario is excluded merely
// because a cassette file exists; the workflow must supply a complete plan.
func liveScenarioInventory(scenarios []string, planPath string) ([]string, string, error) {
	if planPath == "" {
		return scenarios, "", nil
	}
	data, err := os.ReadFile(planPath)
	if err != nil {
		return nil, "", fmt.Errorf("read replay routing: %w", err)
	}
	var plan struct {
		SchemaVersion int      `json:"schema_version"`
		Mode          string   `json:"mode"`
		Live          []string `json:"live"`
		Replay        []string `json:"replay"`
	}
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, "", fmt.Errorf("parse replay routing: %w", err)
	}
	if plan.SchemaVersion != 1 || (plan.Mode != "pr" && plan.Mode != "live") ||
		(plan.Mode == "live" && len(plan.Replay) != 0) {
		return nil, "", fmt.Errorf("invalid replay routing mode or version")
	}
	expected := make([]string, len(scenarios))
	for i, path := range scenarios {
		expected[i] = normalizeScenarioPath(path)
	}
	actual := append(slices.Clone(plan.Live), plan.Replay...)
	slices.Sort(expected)
	slices.Sort(actual)
	if !slices.Equal(expected, actual) {
		return nil, "", fmt.Errorf("replay routing must cover every scenario exactly once")
	}
	live := make([]string, 0, len(plan.Live))
	for _, path := range scenarios {
		if slices.Contains(plan.Live, normalizeScenarioPath(path)) {
			live = append(live, path)
		}
	}
	if len(plan.Replay) == 0 {
		return live, "", nil
	}
	slices.Sort(plan.Replay)
	replayJSON, err := json.Marshal(plan.Replay)
	if err != nil {
		return nil, "", fmt.Errorf("encode replay membership: %w", err)
	}
	return live, fmt.Sprintf(":pr-replay:%x", sha256.Sum256(replayJSON)), nil
}
