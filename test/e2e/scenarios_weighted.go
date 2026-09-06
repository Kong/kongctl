package e2e

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Embed the snapshot so the precompiled scenario binary uses the same weights
// regardless of its working directory. No network access is needed.
//
//go:embed baselines/scenario-weights.json
var scenarioWeightsJSON []byte

type scenarioWeight struct {
	DurationMS int64 `json:"duration_ms"`
	Samples    int   `json:"samples"`
}

type scenarioWeights struct {
	SchemaVersion int                       `json:"schema_version"`
	DefaultMS     int64                     `json:"default_duration_ms"`
	Sources       []string                  `json:"sources"`
	SourceSHA256  map[string]string         `json:"source_sha256"`
	Scenarios     map[string]scenarioWeight `json:"scenarios"`
}

type weightedScenario struct {
	Scenario   string `json:"scenario"`
	DurationMS int64  `json:"duration_ms"`
	Samples    int    `json:"samples"`
	Fallback   bool   `json:"fallback"`
}

type weightedOrganization struct {
	Environment string             `json:"environment"`
	Current     []weightedScenario `json:"current"`
	Proposed    []weightedScenario `json:"proposed"`
	CurrentMS   int64              `json:"current_estimated_ms"`
	ProposedMS  int64              `json:"proposed_estimated_ms"`
}

type weightedScenarioReport struct {
	SchemaVersion int                    `json:"schema_version"`
	Mode          string                 `json:"mode"`
	Uniform       bool                   `json:"uniform_weights"`
	Sources       []string               `json:"weight_sources"`
	SourceSHA256  map[string]string      `json:"weight_source_sha256"`
	Organizations []weightedOrganization `json:"organizations"`
}

func (w scenarioWeights) validate() error {
	if w.SchemaVersion != 1 || w.DefaultMS <= 0 {
		return fmt.Errorf("invalid scenario weight version or default duration")
	}
	for path, weight := range w.Scenarios {
		if path == "" || path != normalizeScenarioPath(path) || weight.DurationMS <= 0 || weight.Samples < 10 {
			return fmt.Errorf("invalid scenario weight for %q", path)
		}
	}
	return nil
}

// planWeightedScenarios never selects scenarios for execution. Current retains
// the existing selector's ordering; Proposed is a deterministic shadow plan.
func planWeightedScenarios(
	scenarios []string,
	environments []string,
	assignments map[string]scenarioAssignment,
	weights scenarioWeights,
) (weightedScenarioReport, error) {
	report := weightedScenarioReport{SchemaVersion: 1, Mode: "report-only"}
	if err := weights.validate(); err != nil {
		return report, err
	}
	if len(environments) == 0 {
		return report, fmt.Errorf("weighted plan requires environments")
	}
	report.Sources = weights.Sources
	report.SourceSHA256 = weights.SourceSHA256
	report.Uniform = len(weights.Scenarios) == 0
	indices := make(map[string]int, len(environments))
	for i, env := range environments {
		if _, exists := indices[env]; exists || strings.TrimSpace(env) == "" {
			return report, fmt.Errorf("invalid or duplicate environment %q", env)
		}
		indices[env] = i
		report.Organizations = append(report.Organizations, weightedOrganization{
			Environment: env, Current: []weightedScenario{}, Proposed: []weightedScenario{},
		})
	}
	items := make(map[string]weightedScenario, len(scenarios))
	unpinned := make([]weightedScenario, 0, len(scenarios))
	var total int64
	for _, path := range scenarios {
		path = normalizeScenarioPath(path)
		if _, exists := items[path]; exists || path == "" {
			return report, fmt.Errorf("invalid or duplicate scenario %q", path)
		}
		weight, known := weights.Scenarios[path]
		if !known {
			weight.DurationMS = weights.DefaultMS
		}
		if weight.DurationMS > math.MaxInt64-total {
			return report, fmt.Errorf("scenario weight total overflows milliseconds")
		}
		total += weight.DurationMS
		item := weightedScenario{path, weight.DurationMS, weight.Samples, !known}
		items[path] = item
		if env := assignments[path].Environment; env != "" {
			index, exists := indices[env]
			if !exists {
				return report, fmt.Errorf("scenario %s pinned to unavailable environment %q", path, env)
			}
			org := &report.Organizations[index]
			org.Proposed = append(org.Proposed, item)
			org.ProposedMS += item.DurationMS
		} else {
			unpinned = append(unpinned, item)
		}
	}
	slices.SortFunc(unpinned, func(a, b weightedScenario) int {
		if a.DurationMS > b.DurationMS {
			return -1
		}
		if a.DurationMS < b.DurationMS {
			return 1
		}
		return strings.Compare(a.Scenario, b.Scenario)
	})
	for _, item := range unpinned {
		index := 0
		for i := range report.Organizations {
			if report.Organizations[i].ProposedMS < report.Organizations[index].ProposedMS {
				index = i
			}
		}
		org := &report.Organizations[index]
		org.Proposed = append(org.Proposed, item)
		org.ProposedMS += item.DurationMS
	}
	for i := range report.Organizations {
		org := &report.Organizations[i]
		slices.SortFunc(org.Proposed, func(a, b weightedScenario) int { return strings.Compare(a.Scenario, b.Scenario) })
		current, err := selectScenariosWithConfig(scenarios, scenarioSelectionConfig{
			Shard:      scenarioShard{Enabled: true, Index: i, Total: len(environments)},
			CurrentEnv: org.Environment, AllowedEnvs: environments, Assignments: assignments,
			ValidateEnvs: true, EnforceEnv: true,
		})
		if err != nil {
			return report, err
		}
		for _, path := range current {
			item := items[normalizeScenarioPath(path)]
			org.Current = append(org.Current, item)
			org.CurrentMS += item.DurationMS
		}
	}
	return report, nil
}

func writeWeightedScenarioReport(artifactsDir string, scenarios []string, cfg scenarioSelectionConfig) error {
	if cfg.Filter != "" || !cfg.Shard.Enabled || !cfg.ValidateEnvs || artifactsDir == "" {
		return nil
	}
	if cfg.Shard.Total != len(cfg.AllowedEnvs) || cfg.Shard.Index < 0 || cfg.Shard.Index >= cfg.Shard.Total ||
		cfg.AllowedEnvs[cfg.Shard.Index] != cfg.CurrentEnv {
		return fmt.Errorf("weighted report requires shard indices matching configured organization order")
	}
	var weights scenarioWeights
	if err := json.Unmarshal(scenarioWeightsJSON, &weights); err != nil {
		return fmt.Errorf("parse scenario weights: %w", err)
	}
	report, err := planWeightedScenarios(scenarios, cfg.AllowedEnvs, cfg.Assignments, weights)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(artifactsDir, "proposed-scenario-assignments.json"), data, 0o600); err != nil {
		return err
	}
	var summary strings.Builder
	summary.WriteString("### Weighted sharding predictions (report only)\n\n")
	summary.WriteString("Live assignments are unchanged. Estimates exclude job overhead and are not measured savings.\n\n")
	fmt.Fprintf(&summary, "Uniform fallback weights: %t.\n\n", report.Uniform)
	summary.WriteString("| Organization | Current estimate (s) | Proposed estimate (s) |\n| --- | ---: | ---: |\n")
	currentMin, proposedMin := int64(math.MaxInt64), int64(math.MaxInt64)
	var currentMax, proposedMax int64
	for _, org := range report.Organizations {
		fmt.Fprintf(&summary, "| %s | %.2f | %.2f |\n", org.Environment,
			float64(org.CurrentMS)/1000, float64(org.ProposedMS)/1000)
		currentMin, proposedMin = min(currentMin, org.CurrentMS), min(proposedMin, org.ProposedMS)
		currentMax, proposedMax = max(currentMax, org.CurrentMS), max(proposedMax, org.ProposedMS)
	}
	fmt.Fprintf(&summary, "\nEstimated longest shard: %.2fs → %.2fs; spread: %.2fs → %.2fs.\n",
		float64(currentMax)/1000, float64(proposedMax)/1000,
		float64(currentMax-currentMin)/1000, float64(proposedMax-proposedMin)/1000)
	return os.WriteFile(filepath.Join(artifactsDir, "weighted-sharding-summary.md"), []byte(summary.String()), 0o600)
}
