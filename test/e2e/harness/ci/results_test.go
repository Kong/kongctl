//go:build e2e

package ci

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestAIGatewayScenariosAreBeta(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(resultsScriptPath(t)), "..", "..", "..", ".."))
	paths, err := filepath.Glob(filepath.Join(repoRoot, "test", "e2e", "scenarios", "ai-gateway", "*", "scenario.yaml"))
	if err != nil {
		t.Fatalf("glob AI Gateway scenarios: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no AI Gateway scenarios found")
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var metadata struct {
			Test struct {
				Maturity string `yaml:"maturity"`
			} `yaml:"test"`
		}
		if err := yaml.Unmarshal(data, &metadata); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if metadata.Test.Maturity != "beta" {
			t.Errorf("%s maturity = %q, want beta", path, metadata.Test.Maturity)
		}
	}
}

func TestResultAccountingSeparatesBetaFailures(t *testing.T) {
	dir := t.TempDir()
	runLog := filepath.Join(dir, "run.log")
	if err := os.WriteFile(runLog, []byte(strings.Join([]string{
		"--- PASS: Test_Scenarios/test/e2e/scenarios/portal/pages/scenario.yaml (1.00s)",
		"--- PASS: Test_Scenarios/scenarios/ai-gateway/model/scenario.yaml (2.00s)",
		"--- FAIL: Test_Scenarios/test/e2e/scenarios/portal/assets/scenario.yaml (3.00s)",
		"--- SKIP: Test_Scenarios/test/e2e/scenarios/portal/email/scenario.yaml (0.00s)",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("write run log: %v", err)
	}

	failureDir := filepath.Join(dir, "beta-failures", "ai-gateway", "model")
	if err := os.MkdirAll(failureDir, 0o755); err != nil {
		t.Fatalf("create beta failure dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(failureDir, "failure.json"), []byte(`{
  "scenario": "ai-gateway/model/scenario.yaml",
  "maturity": "beta",
  "mode": "warn",
  "error": "model failed",
  "cleanup_error": "reset failed"
}`), 0o600); err != nil {
		t.Fatalf("write beta failure: %v", err)
	}

	script := resultsScriptPath(t)
	command := `
source "$1"
e2e_extract_go_results "$2/run.log" PASS "$2/passed.raw"
e2e_extract_go_results "$2/run.log" FAIL "$2/failed"
e2e_extract_go_results "$2/run.log" SKIP "$2/skipped"
e2e_extract_beta_failures "$2" "$2/beta"
e2e_subtract_results "$2/passed.raw" "$2/beta" "$2/passed"
e2e_emit_beta_annotations "$2"
`
	output, err := exec.Command("bash", "-c", command, "bash", script, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("result accounting failed: %v\n%s", err, output)
	}

	assertFileContents(t, filepath.Join(dir, "passed"), "portal/pages/scenario.yaml\n")
	assertFileContents(t, filepath.Join(dir, "failed"), "portal/assets/scenario.yaml\n")
	assertFileContents(t, filepath.Join(dir, "skipped"), "portal/email/scenario.yaml\n")
	assertFileContents(t, filepath.Join(dir, "beta"), "ai-gateway/model/scenario.yaml\n")
	annotations := string(output)
	if !strings.Contains(annotations, "title=Beta E2E failure::model failed") {
		t.Fatalf("annotations missing beta failure: %s", annotations)
	}
	if !strings.Contains(annotations, "title=Beta E2E cleanup failure::reset failed") {
		t.Fatalf("annotations missing cleanup failure: %s", annotations)
	}
}

func TestBlockingFailureReasonIgnoresAdvisoryFailures(t *testing.T) {
	script := resultsScriptPath(t)
	command := `
source "$1"
printf 'advisory=%s\n' "$(e2e_blocking_failure_reason 0 0)"
printf 'stable=%s\n' "$(e2e_blocking_failure_reason 2 0)"
printf 'shard=%s\n' "$(e2e_blocking_failure_reason 0 1)"
printf 'both=%s\n' "$(e2e_blocking_failure_reason 2 1)"
`
	output, err := exec.Command("bash", "-c", command, "bash", script).CombinedOutput()
	if err != nil {
		t.Fatalf("failure gating failed: %v\n%s", err, output)
	}
	got := string(output)
	for _, want := range []string{
		"advisory=\n",
		"stable=2 scenario(s) failed\n",
		"shard=1 shard(s) exited non-zero\n",
		"both=2 scenario(s) failed; 1 shard(s) exited non-zero\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("gating output missing %q:\n%s", want, got)
		}
	}
}

func TestResultCoverageIncludesAllFourCategories(t *testing.T) {
	dir := t.TempDir()
	assigned := filepath.Join(dir, "assigned")
	observed := filepath.Join(dir, "observed")
	if err := os.WriteFile(assigned, []byte(strings.Join([]string{
		"portal/pages/scenario.yaml",
		"portal/assets/scenario.yaml",
		"portal/email/scenario.yaml",
		"ai-gateway/model/scenario.yaml",
	}, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write assigned scenarios: %v", err)
	}
	if err := os.WriteFile(observed, []byte(strings.Join([]string{
		"portal/pages/scenario.yaml",
		"portal/assets/scenario.yaml",
		"portal/email/scenario.yaml",
		"ai-gateway/model/scenario.yaml",
	}, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write observed scenarios: %v", err)
	}

	script := resultsScriptPath(t)
	command := `source "$1"; e2e_result_coverage_error "$2" "$3"`
	output, err := exec.Command("bash", "-c", command, "bash", script, assigned, observed).CombinedOutput()
	if err != nil {
		t.Fatalf("result coverage failed: %v\n%s", err, output)
	}
	if len(output) != 0 {
		t.Fatalf("complete result coverage error = %q, want empty", output)
	}

	if err := os.WriteFile(observed, []byte(strings.Join([]string{
		"portal/pages/scenario.yaml",
		"portal/pages/scenario.yaml",
		"portal/email/scenario.yaml",
		"ai-gateway/model/scenario.yaml",
	}, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write duplicate observed scenarios: %v", err)
	}
	output, err = exec.Command("bash", "-c", command, "bash", script, assigned, observed).CombinedOutput()
	if err != nil {
		t.Fatalf("duplicate result coverage failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "multiple result categories") {
		t.Fatalf("duplicate result coverage error = %q", output)
	}
}

func resultsScriptPath(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	return filepath.Join(filepath.Dir(filename), "results.sh")
}

func assertFileContents(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", path, data, want)
	}
}
