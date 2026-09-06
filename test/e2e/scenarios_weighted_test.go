package e2e

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestWeightedScenarioPlan(t *testing.T) {
	paths := []string{"scenarios/a/scenario.yaml", "scenarios/b/scenario.yaml", "scenarios/c/scenario.yaml"}
	original := slices.Clone(paths)
	weights := scenarioWeights{SchemaVersion: 1, DefaultMS: 10, Scenarios: map[string]scenarioWeight{
		"a/scenario.yaml":       {DurationMS: 100, Samples: 10},
		"b/scenario.yaml":       {DurationMS: 60, Samples: 11},
		"removed/scenario.yaml": {DurationMS: 999, Samples: 10},
	}}
	pins := map[string]scenarioAssignment{"a/scenario.yaml": {Environment: "one"}}
	plan, err := planWeightedScenarios(paths, []string{"one", "two"}, pins, weights)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Organizations[0].WeightedMS != 100 || plan.Organizations[1].WeightedMS != 70 {
		t.Fatalf("pinned load not reserved first: %+v", plan)
	}
	seen := map[string]bool{}
	for _, org := range plan.Organizations {
		for _, item := range org.Weighted {
			if seen[item.Scenario] {
				t.Fatalf("duplicate assignment: %s", item.Scenario)
			}
			seen[item.Scenario] = true
		}
	}
	if len(seen) != len(paths) || !plan.Organizations[1].Weighted[1].Fallback {
		t.Fatalf("coverage or new-scenario fallback incorrect: %+v", plan)
	}
	if plan.Organizations[0].LegacyMS != 160 || plan.Organizations[1].LegacyMS != 10 {
		t.Fatalf("current selector comparison incorrect: %+v", plan)
	}
	reversed := slices.Clone(paths)
	slices.Reverse(reversed)
	other, err := planWeightedScenarios(reversed, []string{"one", "two"}, pins, weights)
	if err != nil {
		t.Fatal(err)
	}
	for i, org := range plan.Organizations {
		if !reflect.DeepEqual(org.Weighted, other.Organizations[i].Weighted) {
			t.Fatal("proposed assignment depends on input order")
		}
	}
	if !reflect.DeepEqual(paths, original) {
		t.Fatal("scheduler mutated input")
	}
}

func TestWeightedUniformTiesAndEmptyShards(t *testing.T) {
	weights := scenarioWeights{SchemaVersion: 1, DefaultMS: 1000}
	for _, paths := range [][]string{nil, {"b", "a"}, {"b", "a", "c"}} {
		plan, err := planWeightedScenarios(paths, []string{"one", "two", "three"}, nil, weights)
		if err != nil {
			t.Fatal(err)
		}
		if !plan.Uniform || len(plan.Organizations) != 3 {
			t.Fatal("uniform fallback or empty shard missing")
		}
		if len(paths) > 0 && plan.Organizations[0].Weighted[0].Scenario != "a" {
			t.Fatal("ties must use path and configured org order")
		}
	}
}

func TestWeightedInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		paths   []string
		envs    []string
		pins    map[string]scenarioAssignment
		weights scenarioWeights
	}{
		{"version", nil, []string{"one"}, nil, scenarioWeights{DefaultMS: 1}},
		{"zero default", nil, []string{"one"}, nil, scenarioWeights{SchemaVersion: 1}},
		{"no orgs", nil, nil, nil, scenarioWeights{SchemaVersion: 1, DefaultMS: 1}},
		{"duplicate org", nil, []string{"one", "one"}, nil, scenarioWeights{SchemaVersion: 1, DefaultMS: 1}},
		{
			"duplicate scenario",
			[]string{"a", "scenarios/a"},
			[]string{"one"},
			nil,
			scenarioWeights{SchemaVersion: 1, DefaultMS: 1},
		},
		{
			"missing pin",
			[]string{"a"},
			[]string{"one"},
			map[string]scenarioAssignment{"a": {Environment: "two"}},
			scenarioWeights{SchemaVersion: 1, DefaultMS: 1},
		},
		{"overflow", []string{"a", "b"}, []string{"one"}, nil, scenarioWeights{SchemaVersion: 1, DefaultMS: math.MaxInt64}},
		{
			"bad weight",
			[]string{"a"},
			[]string{"one"},
			nil,
			scenarioWeights{
				SchemaVersion: 1, DefaultMS: 1,
				Scenarios: map[string]scenarioWeight{"a": {DurationMS: -1, Samples: 10}},
			},
		},
		{
			"few samples",
			[]string{"a"},
			[]string{"one"},
			nil,
			scenarioWeights{
				SchemaVersion: 1, DefaultMS: 1,
				Scenarios: map[string]scenarioWeight{"a": {DurationMS: 1, Samples: 9}},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := planWeightedScenarios(tc.paths, tc.envs, tc.pins, tc.weights); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestWeightedReportIsolation(t *testing.T) {
	var weights scenarioWeights
	if err := json.Unmarshal(scenarioWeightsJSON, &weights); err != nil {
		t.Fatal(err)
	}
	if err := weights.validate(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	paths := []string{"new/scenario.yaml"}
	cfg := scenarioSelectionConfig{
		Shard: scenarioShard{Enabled: true, Total: 1}, CurrentEnv: "one",
		AllowedEnvs: []string{"one"}, ValidateEnvs: true,
	}
	if err := writeScenarioShardManifest(dir, cfg.Shard, paths, nil); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(dir, "assigned-scenarios.txt")
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeWeightedScenarioReport(dir, paths, cfg); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(manifest)
	if err != nil || string(before) != string(after) {
		t.Fatal("actual manifest changed")
	}
	for _, name := range []string{"proposed-scenario-assignments.json", "weighted-sharding-summary.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []string{"filtered", "tech", "unsharded"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			copyCfg := cfg
			switch mode {
			case "filtered":
				copyCfg.Filter = "new"
			case "tech":
				copyCfg.ValidateEnvs = false
			case "unsharded":
				copyCfg.Shard.Enabled = false
			}
			if err := writeWeightedScenarioReport(dir, paths, copyCfg); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 {
				t.Fatal("ineligible run generated report")
			}
		})
	}
	invalid := cfg
	invalid.Shard.Total = 2
	if err := writeWeightedScenarioReport(dir, paths, invalid); err == nil {
		t.Fatal("expected mismatched matrix error")
	}
	if err := writeWeightedScenarioReport(manifest, paths, cfg); err == nil {
		t.Fatal("expected report I/O error")
	}
}
