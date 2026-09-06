package e2e

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	allocationModulo   = "modulo"
	allocationWeighted = "weighted"
	allocationModuloID = "modulo-v1"
)

type scenarioAllocation struct {
	SchemaVersion int    `json:"schema_version"`
	Strategy      string `json:"strategy"`
	ID            string `json:"allocation_id"`
}

func scenarioAllocationForConfig(cfg scenarioSelectionConfig) (scenarioAllocation, error) {
	allocation := scenarioAllocation{SchemaVersion: 1, Strategy: allocationModulo, ID: allocationModuloID}
	if cfg.Filter != "" || !cfg.Shard.Enabled || !cfg.ValidateEnvs {
		return allocation, nil
	}
	switch strings.TrimSpace(cfg.Strategy) {
	case allocationModulo:
		return allocation, nil
	case "", allocationWeighted:
		allocation.Strategy = allocationWeighted
		allocation.ID = fmt.Sprintf("weighted-v1:%x", sha256.Sum256(scenarioWeightsJSON))
		return allocation, nil
	default:
		return allocation, fmt.Errorf("invalid KONGCTL_E2E_SHARD_STRATEGY %q: use weighted or modulo", cfg.Strategy)
	}
}

func validateWeightedMatrix(cfg scenarioSelectionConfig) error {
	if cfg.Shard.Total != len(cfg.AllowedEnvs) || cfg.Shard.Index < 0 || cfg.Shard.Index >= cfg.Shard.Total ||
		cfg.AllowedEnvs[cfg.Shard.Index] != cfg.CurrentEnv {
		return fmt.Errorf("weighted allocation requires shard indices matching configured organization order")
	}
	return nil
}

func selectScenariosForExecution(
	scenarios []string, cfg scenarioSelectionConfig,
) ([]string, scenarioAllocation, error) {
	allocation, err := scenarioAllocationForConfig(cfg)
	if err != nil {
		return nil, allocation, err
	}
	if allocation.Strategy == allocationModulo {
		selected, err := selectScenariosWithConfig(scenarios, cfg)
		return selected, allocation, err
	}
	if err := validateWeightedMatrix(cfg); err != nil {
		return nil, allocation, err
	}
	var weights scenarioWeights
	if err := json.Unmarshal(scenarioWeightsJSON, &weights); err != nil {
		return nil, allocation, fmt.Errorf("parse activation weights: %w", err)
	}
	plan, err := planWeightedScenarios(scenarios, cfg.AllowedEnvs, cfg.Assignments, weights)
	if err != nil {
		return nil, allocation, err
	}
	paths := make(map[string]string, len(scenarios))
	for _, path := range scenarios {
		paths[normalizeScenarioPath(path)] = path
	}
	selected := make([]string, 0, len(plan.Organizations[cfg.Shard.Index].Weighted))
	for _, item := range plan.Organizations[cfg.Shard.Index].Weighted {
		selected = append(selected, paths[item.Scenario])
	}
	return selected, allocation, nil
}

func writeScenarioAllocation(dir string, shard scenarioShard, allocation scenarioAllocation) error {
	if !shard.Enabled || strings.TrimSpace(dir) == "" {
		return nil
	}
	data, err := json.MarshalIndent(allocation, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "scenario-allocation.json"), append(data, '\n'), 0o600)
}
