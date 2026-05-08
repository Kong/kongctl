package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type scenarioShard struct {
	Enabled bool
	Index   int
	Total   int
}

type scenarioAssignment struct {
	Environment string
}

type scenarioSelectionConfig struct {
	Filter       string
	Shard        scenarioShard
	CurrentEnv   string
	AllowedEnvs  []string
	Assignments  map[string]scenarioAssignment
	ValidateEnvs bool
	EnforceEnv   bool
}

func selectScenarios(scenarios []string, filter string, shard scenarioShard) []string {
	selected, _ := selectScenariosWithConfig(scenarios, scenarioSelectionConfig{
		Filter: filter,
		Shard:  shard,
	})
	return selected
}

func selectScenariosWithConfig(scenarios []string, cfg scenarioSelectionConfig) ([]string, error) {
	if cfg.Filter != "" {
		selected := make([]string, 0, len(scenarios))
		for _, p := range scenarios {
			if scenarioMatches(p, cfg.Filter) {
				selected = append(selected, p)
			}
		}
		return selected, nil
	}

	if cfg.ValidateEnvs {
		for scenarioPath, assignment := range cfg.Assignments {
			if assignment.Environment == "" {
				continue
			}
			if !slices.Contains(cfg.AllowedEnvs, assignment.Environment) {
				return nil, fmt.Errorf(
					"scenario %s is assigned to environment %q, but KONGCTL_E2E_ORGS_JSON does not include that org_name",
					normalizeScenarioPath(scenarioPath),
					assignment.Environment,
				)
			}
		}
	}

	currentEnv := strings.TrimSpace(cfg.CurrentEnv)
	enforceEnv := cfg.EnforceEnv || currentEnv != ""
	unassigned := make([]string, 0, len(scenarios))
	pinned := make([]string, 0)
	for _, p := range scenarios {
		assignment := cfg.Assignments[normalizeScenarioPath(p)]
		if assignment.Environment != "" && enforceEnv {
			if assignment.Environment == currentEnv {
				pinned = append(pinned, p)
			}
			continue
		}
		unassigned = append(unassigned, p)
	}

	selected := unassigned
	if cfg.Shard.Enabled {
		selected = make([]string, 0, len(unassigned))
		for i, p := range unassigned {
			if i%cfg.Shard.Total == cfg.Shard.Index {
				selected = append(selected, p)
			}
		}
	}

	selected = append(selected, pinned...)
	return selected, nil
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
	return strings.TrimSuffix(path, "/")
}

func loadScenarioOrgNames(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	var orgs []struct {
		OrgName string `json:"org_name"`
	}
	if err := json.Unmarshal([]byte(raw), &orgs); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(orgs))
	for _, org := range orgs {
		name := strings.TrimSpace(org.OrgName)
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
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
	return os.WriteFile(path, []byte(b.String()), 0o600)
}
