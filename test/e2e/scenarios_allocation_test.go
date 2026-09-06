package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestWeightedExecutionAndRollback(t *testing.T) {
	paths := []string{"scenarios/a", "scenarios/b", "scenarios/c", "scenarios/d", "scenarios/e", "scenarios/f"}
	cfg := scenarioSelectionConfig{
		Shard: scenarioShard{Enabled: true, Total: 2}, CurrentEnv: "one", AllowedEnvs: []string{"one", "two"},
		Assignments: map[string]scenarioAssignment{"a": {Environment: "one"}}, ValidateEnvs: true, EnforceEnv: true,
	}
	selected, allocation, err := selectScenariosForExecution(paths, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selected, []string{"scenarios/a", "scenarios/c", "scenarios/e"}) {
		t.Fatalf("unexpected weighted execution order: %v", selected)
	}
	if allocation.Strategy != allocationWeighted || !strings.HasPrefix(allocation.ID, "weighted-v1:") {
		t.Fatalf("missing activation identity: %+v", allocation)
	}
	weightedID := allocation.ID
	dir := t.TempDir()
	if err := writeScenarioAllocation(dir, cfg.Shard, allocation); err != nil {
		t.Fatal(err)
	}
	if err := writeScenarioShardManifest(dir, cfg.Shard, selected, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "scenario-allocation.json"))
	if err != nil {
		t.Fatal(err)
	}
	var recorded scenarioAllocation
	if err := json.Unmarshal(data, &recorded); err != nil || recorded != allocation {
		t.Fatalf("allocation identity did not round trip: %s, %v", data, err)
	}
	data, err = os.ReadFile(filepath.Join(dir, "assigned-scenarios.txt"))
	if err != nil || string(data) != "shard_index=0\nshard_total=2\n\na\nc\ne\n" {
		t.Fatalf("manifest does not describe actual execution: %s, %v", data, err)
	}
	cfg.Strategy = allocationModulo
	selected, allocation, err = selectScenariosForExecution(paths, cfg)
	legacy, legacyErr := selectScenariosWithConfig(paths, cfg)
	if err != nil || legacyErr != nil || !reflect.DeepEqual(selected, legacy) {
		t.Fatalf("rollback differs from legacy allocation: %v, %v", err, legacyErr)
	}
	if allocation.ID != allocationModuloID || allocation.ID == weightedID {
		t.Fatal("rollback must be identifiable separately from weighted execution")
	}
}

func TestWeightedScopeAndInvalidConfiguration(t *testing.T) {
	base := scenarioSelectionConfig{
		Shard: scenarioShard{Enabled: true, Total: 1}, CurrentEnv: "one", AllowedEnvs: []string{"one"},
		ValidateEnvs: true, EnforceEnv: true,
	}
	paths := []string{"scenarios/a/scenario.yaml"}
	for _, mode := range []string{"filtered", "tech", "unsharded"} {
		t.Run(mode, func(t *testing.T) {
			cfg := base
			switch mode {
			case "filtered":
				cfg.Filter = "a"
			case "tech":
				cfg.ValidateEnvs = false
			case "unsharded":
				cfg.Shard.Enabled = false
			}
			selected, allocation, err := selectScenariosForExecution(paths, cfg)
			legacy, legacyErr := selectScenariosWithConfig(paths, cfg)
			if err != nil || legacyErr != nil || !slices.Equal(selected, legacy) || allocation.ID != allocationModuloID {
				t.Fatalf("out-of-scope selection changed: %+v, %v", allocation, err)
			}
		})
	}
	invalid := base
	invalid.Strategy = "weightd"
	if _, _, err := selectScenariosForExecution(paths, invalid); err == nil {
		t.Fatal("invalid strategy must not silently change allocation")
	}
	invalid = base
	invalid.CurrentEnv = "different"
	if _, _, err := selectScenariosForExecution(paths, invalid); err == nil {
		t.Fatal("invalid matrix must fail before execution")
	}
}

func TestWeightedMalformedSnapshotFailsClosed(t *testing.T) {
	original := scenarioWeightsJSON
	t.Cleanup(func() { scenarioWeightsJSON = original })
	cfg := scenarioSelectionConfig{
		Shard: scenarioShard{Enabled: true, Total: 1}, CurrentEnv: "one", AllowedEnvs: []string{"one"},
		ValidateEnvs: true,
	}
	for _, data := range []string{"{", `{"schema_version":1,"default_duration_ms":0}`} {
		scenarioWeightsJSON = []byte(data)
		if _, _, err := selectScenariosForExecution([]string{"a"}, cfg); err == nil {
			t.Fatal("invalid weights must fail weighted execution")
		}
		rollback := cfg
		rollback.Strategy = allocationModulo
		if _, allocation, err := selectScenariosForExecution([]string{"a"}, rollback); err != nil ||
			allocation.ID != allocationModuloID {
			t.Fatal("rollback must work independently of broken weights")
		}
	}
}
