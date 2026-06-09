//go:build e2e

package e2e

import (
	"os"
	"strings"

	"sigs.k8s.io/yaml"
)

func loadScenarioAssignments(scenarios []string) (map[string]scenarioAssignment, error) {
	assignments := make(map[string]scenarioAssignment)
	for _, scenarioPath := range scenarios {
		assignment, err := loadScenarioAssignment(scenarioPath)
		if err != nil {
			return nil, err
		}
		if assignment.Environment != "" {
			assignments[normalizeScenarioPath(scenarioPath)] = assignment
		}
	}
	return assignments, nil
}

func loadScenarioAssignment(path string) (scenarioAssignment, error) {
	var raw struct {
		Test struct {
			AssignedEnvironment string `yaml:"assignedEnvironment"`
		} `yaml:"test"`
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return scenarioAssignment{}, err
	}
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return scenarioAssignment{}, err
	}

	return scenarioAssignment{
		Environment: strings.TrimSpace(raw.Test.AssignedEnvironment),
	}, nil
}
