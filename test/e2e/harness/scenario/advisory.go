//go:build e2e

package scenario

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kong/kongctl/test/e2e/harness"
)

type AdvisoryFailure struct {
	Scenario     string   `json:"scenario"`
	Maturity     Maturity `json:"maturity"`
	Mode         BetaMode `json:"mode"`
	Error        string   `json:"error"`
	CleanupError string   `json:"cleanup_error,omitempty"`
}

func RecordAdvisoryFailure(
	scenarioPath string,
	maturity Maturity,
	mode BetaMode,
	scenarioErr error,
	cleanupErr error,
) (string, error) {
	artifactsDir, err := harness.ArtifactsDir()
	if err != nil {
		return "", fmt.Errorf("resolve artifacts directory: %w", err)
	}
	return recordAdvisoryFailureAt(
		artifactsDir,
		scenarioPath,
		maturity,
		mode,
		scenarioErr,
		cleanupErr,
	)
}

func recordAdvisoryFailureAt(
	artifactsDir string,
	scenarioPath string,
	maturity Maturity,
	mode BetaMode,
	scenarioErr error,
	cleanupErr error,
) (string, error) {
	if scenarioErr == nil {
		return "", fmt.Errorf("scenario error is required")
	}
	scenarioName, err := advisoryScenarioName(scenarioPath)
	if err != nil {
		return "", err
	}

	record := AdvisoryFailure{
		Scenario: scenarioName,
		Maturity: maturity,
		Mode:     mode,
		Error:    scenarioErr.Error(),
	}
	if cleanupErr != nil {
		record.CleanupError = cleanupErr.Error()
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal advisory failure: %w", err)
	}
	data = append(data, '\n')

	scenarioDir := strings.TrimSuffix(scenarioName, "/scenario.yaml")
	path := filepath.Join(artifactsDir, "beta-failures", filepath.FromSlash(scenarioDir), "failure.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create advisory artifact directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".failure-*.json")
	if err != nil {
		return "", fmt.Errorf("create advisory artifact: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("set advisory artifact permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write advisory artifact: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close advisory artifact: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", fmt.Errorf("publish advisory artifact: %w", err)
	}

	return path, nil
}

func advisoryScenarioName(path string) (string, error) {
	name := filepath.ToSlash(strings.TrimSpace(path))
	name = strings.TrimPrefix(name, "scenarios/")
	name = strings.TrimPrefix(name, "test/e2e/scenarios/")
	name = strings.TrimSuffix(name, "/")
	name = filepath.ToSlash(filepath.Clean(name))

	if name == "" || name == "." || filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") {
		return "", fmt.Errorf("invalid advisory scenario path %q", path)
	}
	if !strings.HasSuffix(name, "/scenario.yaml") {
		return "", fmt.Errorf("invalid advisory scenario path %q: expected scenario.yaml", path)
	}
	return name, nil
}
