//go:build e2e

package scenario

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads a scenario.yaml path into a Scenario with basic normalization.
func Load(path string) (Scenario, error) {
	var s Scenario
	b, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := yaml.Unmarshal(b, &s); err != nil {
		return s, err
	}
	// Defaults
	if s.Defaults.Retry.Attempts == 0 {
		s.Defaults.Retry.Attempts = 1
	}
	if strings.TrimSpace(s.Defaults.Retry.Interval) == "" {
		s.Defaults.Retry.Interval = "0s"
	}
	return s, nil
}

// ScenarioRoot returns the directory containing the scenario file.
func ScenarioRoot(scenarioPath string) string {
	p, _ := filepath.Abs(filepath.Dir(scenarioPath))
	return p
}
