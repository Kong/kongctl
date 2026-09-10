package e2e

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLiveScenarioInventory(t *testing.T) {
	scenarios := []string{"scenarios/a/scenario.yaml", "scenarios/b/scenario.yaml"}
	for _, tt := range []struct {
		name, plan string
		want       []string
		replay     bool
	}{
		{"no plan", "", scenarios, false},
		{
			"main", `{"schema_version":1,"mode":"live","live":["a/scenario.yaml","b/scenario.yaml"],"replay":[]}`,
			scenarios, false,
		},
		{
			"pr", `{"schema_version":1,"mode":"pr","live":["b/scenario.yaml"],"replay":["a/scenario.yaml"]}`,
			scenarios[1:], true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := ""
			if tt.plan != "" {
				path = filepath.Join(t.TempDir(), "routing.json")
				if err := os.WriteFile(path, []byte(tt.plan), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, suffix, err := liveScenarioInventory(scenarios, path)
			if err != nil || !slices.Equal(got, tt.want) || strings.HasPrefix(suffix, ":pr-replay:") != tt.replay {
				t.Fatalf("got %v, %q, %v", got, suffix, err)
			}
		})
	}
}

func TestLiveScenarioInventoryRejectsCoverageGaps(t *testing.T) {
	for _, plan := range []string{
		`{}`,
		`{"schema_version":1,"mode":"live","live":[],"replay":["a/scenario.yaml"]}`,
		`{"schema_version":1,"mode":"pr","live":[],"replay":[]}`,
		`{"schema_version":1,"mode":"pr","live":["a/scenario.yaml"],"replay":["a/scenario.yaml"]}`,
		`{"schema_version":1,"mode":"pr","live":["unknown/scenario.yaml"],"replay":[]}`,
	} {
		path := filepath.Join(t.TempDir(), "routing.json")
		if err := os.WriteFile(path, []byte(plan), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := liveScenarioInventory([]string{"scenarios/a/scenario.yaml"}, path); err == nil {
			t.Fatalf("accepted invalid plan %s", plan)
		}
	}
}
