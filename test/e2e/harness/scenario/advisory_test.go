//go:build e2e

package scenario

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRecordAdvisoryFailure(t *testing.T) {
	artifactsDir := t.TempDir()
	t.Setenv("KONGCTL_E2E_ARTIFACTS_DIR", artifactsDir)

	path, err := RecordAdvisoryFailure(
		"test/e2e/scenarios/ai-gateway/model/scenario.yaml",
		MaturityBeta,
		BetaModeWarn,
		errors.New("model scenario failed"),
		errors.New("reset failed"),
	)
	if err != nil {
		t.Fatalf("RecordAdvisoryFailure() error = %v", err)
	}

	wantPath := filepath.Join(artifactsDir, "beta-failures", "ai-gateway", "model", "failure.json")
	if path != wantPath {
		t.Fatalf("RecordAdvisoryFailure() path = %q, want %q", path, wantPath)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read advisory artifact: %v", err)
	}
	var got AdvisoryFailure
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parse advisory artifact: %v", err)
	}
	if got.Scenario != "ai-gateway/model/scenario.yaml" {
		t.Fatalf("scenario = %q, want ai-gateway/model/scenario.yaml", got.Scenario)
	}
	if got.Maturity != MaturityBeta || got.Mode != BetaModeWarn {
		t.Fatalf("policy = %q/%q, want beta/warn", got.Maturity, got.Mode)
	}
	if got.Error != "model scenario failed" || got.CleanupError != "reset failed" {
		t.Fatalf("errors = %q/%q, want scenario and cleanup errors", got.Error, got.CleanupError)
	}
}

func TestRecordAdvisoryFailureOverwritesInitialRecord(t *testing.T) {
	artifactsDir := t.TempDir()
	t.Setenv("KONGCTL_E2E_ARTIFACTS_DIR", artifactsDir)
	scenarioErr := errors.New("scenario failed")
	scenarioPath := "test/e2e/scenarios/ai-gateway/model/scenario.yaml"

	path, err := RecordAdvisoryFailure(scenarioPath, MaturityBeta, BetaModeWarn, scenarioErr, nil)
	if err != nil {
		t.Fatalf("write initial advisory: %v", err)
	}
	if _, err := RecordAdvisoryFailure(
		scenarioPath,
		MaturityBeta,
		BetaModeWarn,
		scenarioErr,
		errors.New("cleanup failed"),
	); err != nil {
		t.Fatalf("update advisory: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read advisory artifact: %v", err)
	}
	var got AdvisoryFailure
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parse advisory artifact: %v", err)
	}
	if got.Error != "scenario failed" || got.CleanupError != "cleanup failed" {
		t.Fatalf("updated errors = %q/%q, want original and cleanup failures", got.Error, got.CleanupError)
	}
}

func TestRecordAdvisoryFailureRejectsInvalidInputs(t *testing.T) {
	t.Setenv("KONGCTL_E2E_ARTIFACTS_DIR", t.TempDir())

	if _, err := RecordAdvisoryFailure("scenario.yaml", MaturityBeta, BetaModeWarn, nil, nil); err == nil {
		t.Fatal("RecordAdvisoryFailure(nil error) error = nil, want error")
	}
	if _, err := RecordAdvisoryFailure(
		"../../scenario.yaml",
		MaturityBeta,
		BetaModeWarn,
		errors.New("failed"),
		nil,
	); err == nil {
		t.Fatal("RecordAdvisoryFailure(invalid path) error = nil, want error")
	}
}
