//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestWeightedRefreshHoldouts compares placement using measured durations from
// runs excluded from weight generation. It never executes scenarios or APIs.
func TestWeightedRefreshHoldouts(t *testing.T) {
	root := "."
	if _, err := os.Stat("scenarios"); os.IsNotExist(err) {
		root = "test/e2e"
	}
	readJSON := func(path string, target any) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
	}
	var oldWeights, newWeights scenarioWeights
	readJSON("baselines/scenario-weights-2026-09.json", &oldWeights)
	readJSON("baselines/scenario-weights.json", &newWeights)
	trainingIDs := map[int64]bool{}
	for _, route := range []string{"live", "pr"} {
		var training struct {
			Runs []struct {
				ID int64 `json:"run_id"`
			} `json:"runs"`
		}
		readJSON("baselines/weights-refresh-2026-11-training-"+route+".json", &training)
		for _, run := range training.Runs {
			trainingIDs[run.ID] = true
		}
	}
	for _, route := range []string{"live", "pr"} {
		t.Run(route, func(t *testing.T) {
			var observations struct {
				Assignments map[string]scenarioAssignment `json:"scenario_assignments"`
				Runs        []struct {
					ID     int64 `json:"run_id"`
					Shards []struct {
						Environment string `json:"org_name"`
						Count       int    `json:"selected_scenario_count"`
					} `json:"shards"`
					Durations []struct {
						Scenario string  `json:"scenario"`
						Seconds  float64 `json:"duration_seconds"`
						Result   string  `json:"result"`
					} `json:"scenario_durations"`
				} `json:"runs"`
			}
			readJSON("baselines/weights-refresh-2026-11-"+route+"-holdout.json", &observations)
			if len(observations.Runs) == 0 {
				t.Fatal("expected held-out runs")
			}
			for _, run := range observations.Runs {
				if trainingIDs[run.ID] {
					t.Fatalf("run %d leaked into training", run.ID)
				}
				var paths, environments []string
				measured := map[string]float64{}
				for _, record := range run.Durations {
					path := normalizeScenarioPath(record.Scenario)
					if _, exists := measured[path]; exists {
						t.Fatalf("duplicate observation %s in run %d", path, run.ID)
					}
					if record.Result != "pass" && record.Result != "skip" {
						t.Fatalf("unsuccessful scenario in held-out run %d", run.ID)
					}
					paths = append(paths, filepath.Join(root, "scenarios", path))
					measured[path] = record.Seconds
				}
				slices.Sort(paths)
				pins := observations.Assignments
				if len(pins) == 0 {
					t.Fatal("missing frozen organization pins")
				}
				recordedOrganizations := map[string]string{}
				offset := 0
				for _, shard := range run.Shards {
					environments = append(environments, shard.Environment)
					end := offset + shard.Count
					if shard.Count < 0 || end > len(run.Durations) {
						t.Fatal("invalid recorded shard inventory")
					}
					for _, record := range run.Durations[offset:end] {
						recordedOrganizations[normalizeScenarioPath(record.Scenario)] = shard.Environment
					}
					offset = end
				}
				if len(environments) != 5 || offset != len(paths) {
					t.Fatal("expected complete inventories for five production organizations")
				}
				for i, weights := range []scenarioWeights{oldWeights, newWeights} {
					plan, err := planWeightedScenarios(paths, environments, pins, weights)
					if err != nil {
						t.Fatal(err)
					}
					seen := map[string]bool{}
					loads := make([]float64, len(environments))
					for shard, org := range plan.Organizations {
						for _, item := range org.Weighted {
							seconds, exists := measured[item.Scenario]
							if !exists || seen[item.Scenario] {
								t.Fatalf("unexpected or duplicate assignment %s", item.Scenario)
							}
							seen[item.Scenario] = true
							if i == 0 && recordedOrganizations[item.Scenario] != org.Environment {
								t.Fatalf("old placement does not reproduce recorded organization for %s in run %d",
									item.Scenario, run.ID)
							}
							if pin := pins[item.Scenario].Environment; pin != "" && pin != org.Environment {
								t.Fatalf("scenario %s moved away from pin %s", item.Scenario, pin)
							}
							loads[shard] += seconds
						}
					}
					if len(seen) != len(paths) {
						t.Fatal("held-out inventory coverage changed")
					}
					t.Logf("run=%d snapshot=%s scenarios=%d pins=%d loads=%v longest=%.3f spread=%.3f",
						run.ID, []string{"old", "new"}[i], len(paths), len(pins), loads,
						slices.Max(loads), slices.Max(loads)-slices.Min(loads))
				}
			}
		})
	}
}
