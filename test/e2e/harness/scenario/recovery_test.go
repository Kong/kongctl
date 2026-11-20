//go:build e2e

package scenario

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kong/kongctl/test/e2e/harness"
)

func TestRecoveryModesAndCleanupFailure(t *testing.T) {
	for _, value := range []string{"", "report", "active", "off", "invalid"} {
		t.Setenv("KONGCTL_E2E_SCENARIO_RETRY", value)
		mode, err := RecoveryModeFromEnv()
		if (err != nil) != (value == "invalid") || value == "" && mode != RecoveryReport {
			t.Fatalf("incorrect mode for %q: %s, %v", value, mode, err)
		}
	}
	for _, mode := range []RecoveryMode{RecoveryReport, RecoveryOff, RecoveryActive} {
		calls := 0
		first := &harness.ScenarioFailure{Cause: errors.New("original"), Class: "http_5xx"}
		r, err := recoverScenario(mode, func(int) error {
			calls++
			return &RecoveryBlockedError{Cause: first}
		}, func() error {
			t.Fatal("cleanup failure must never authorize reset or retry")
			return nil
		})
		if !IsRecoveryBlocked(err) || r.Eligible || calls != 1 {
			t.Fatalf("incorrect cleanup handling: %+v, %v, calls=%d", r, err, calls)
		}
	}
}

// Exercise the real runner in a subprocess so fail-fast and testing.T cleanup
// failures are observable without marking this parent test failed.
func TestScenarioRecoveryShard(t *testing.T) {
	for _, name := range []string{
		"first", "middle", "last", "exhausted", "assertion", "reset", "report", "disabled", "unparsable",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			//nolint:gosec // Re-execute this test binary with fixed fixture arguments.
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestScenarioRecoveryFixture$",
				"-test.v", "-test.failfast")
			cmd.Env = append(os.Environ(), "KONGCTL_RECOVERY_FIXTURE="+name,
				"KONGCTL_E2E_ARTIFACTS_DIR="+dir, "KONGCTL_E2E_BIN=/bin/true")
			out, err := cmd.CombinedOutput()
			green := name == "first" || name == "middle" || name == "last"
			if (err == nil) != green {
				t.Fatalf("unexpected fixture result: %v\n%s", err, out)
			}
			files, err := filepath.Glob(filepath.Join(dir, "tests", "*", "scenario-recovery.json"))
			if err != nil {
				t.Fatal(err)
			}
			wantCount := 1
			if green {
				wantCount = 3
			}
			if len(files) != wantCount {
				t.Fatalf("got %d results, want %d\n%s", len(files), wantCount, out)
			}
			recovered, firstPass := 0, 0
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				var r recoveryResult
				if err := json.Unmarshal(data, &r); err != nil {
					t.Fatal(err)
				}
				if r.Eligible && !strings.Contains(r.FirstError, "kong:trace:first-failure") {
					t.Fatalf("lost original trace ID: %+v", r)
				}
				switch r.Outcome {
				case "first_pass":
					firstPass++
				case "recovered":
					recovered++
					if !r.Eligible || !strings.Contains(r.FirstError, "kong:trace:first-failure") {
						t.Fatalf("lost first failure: %+v", r)
					}
				}
				if name == "exhausted" && (r.Outcome != "exhausted" || r.RetryError == "") {
					t.Fatalf("missing exhausted result: %+v", r)
				}
				if (name == "reset" || name == "disabled") && (r.Outcome != "reset_failed" || r.ResetError == "") {
					t.Fatalf("missing reset failure: %+v", r)
				}
				if name == "assertion" && (r.Eligible || !strings.Contains(r.FirstError, "assertion mismatch")) {
					t.Fatalf("assertion incorrectly eligible: %+v", r)
				}
				if name == "unparsable" && (r.Eligible ||
					!strings.Contains(r.FirstError, "reported a failure to testing.T") ||
					!strings.Contains(r.FirstError, "unparsable output") || strings.Contains(string(out), "attempt-2")) {
					t.Fatalf("incorrect execution failure report: %+v\n%s", r, out)
				}
				if name == "report" && (!r.Eligible || r.Outcome != "failed") {
					t.Fatalf("missing report-only eligibility: %+v", r)
				}
			}
			if green {
				log := strings.ReplaceAll(string(out), "TestScenarioRecoveryFixture/", "Test_Scenarios/")
				if err := os.WriteFile(filepath.Join(dir, "run.log"), []byte(log), 0o600); err != nil {
					t.Fatal(err)
				}
				assigned := "0/scenario.yaml\n1/scenario.yaml\n2/scenario.yaml\n"
				if err := os.WriteFile(filepath.Join(dir, "assigned"), []byte(assigned), 0o600); err != nil {
					t.Fatal(err)
				}
				check := exec.CommandContext(t.Context(), "bash", "-c", `
source "$1"
e2e_extract_go_results "$2/run.log" PASS "$2/observed"
error="$(e2e_result_coverage_error "$2/assigned" "$2/observed" false)"
if [ -n "$error" ]; then echo "$error"; exit 1; fi
`, "bash", "../ci/results.sh", dir)
				if output, err := check.CombinedOutput(); err != nil {
					t.Fatalf("green shard did not satisfy coverage verifier: %v\n%s", err, output)
				}
			}
			if green && (recovered != 1 || firstPass != 2) {
				t.Fatalf("incorrect shard outcomes: recovered=%d first_pass=%d", recovered, firstPass)
			}
		})
	}
}

func TestScenarioRecoveryFixture(t *testing.T) {
	name := os.Getenv("KONGCTL_RECOVERY_FIXTURE")
	if name == "" {
		t.Skip("subprocess fixture")
	}
	failIndex := 0
	switch name {
	case "middle":
		failIndex = 1
	case "last":
		failIndex = 2
	}
	var posts, resets atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			attempt := posts.Add(1)
			if name == "unparsable" {
				w.WriteHeader(http.StatusCreated)
				_, _ = fmt.Fprint(w, "invalid JSON")
				return
			}
			if name != "assertion" && (attempt == int32(failIndex+1) || name == "exhausted") {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprint(w, `{"instance":"kong:trace:first-failure"}`)
				return
			}
			// Recovery must reset even though the fixture has no initial resetOrg.
			if name != "assertion" && attempt == int32(failIndex+2) && resets.Load() == 0 {
				w.WriteHeader(http.StatusConflict)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(w, `{"id":"portal-id","name":"fixture"}`)
			return
		}
		resets.Add(1)
		if name == "reset" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()
	t.Setenv("KONGCTL_E2E_KONNECT_BASE_URL", server.URL)
	t.Setenv("KONGCTL_E2E_KONNECT_BASE_AUTH_URL", server.URL)
	t.Setenv("KONGCTL_E2E_KONNECT_PAT", "fixture-token")
	t.Setenv("KONGCTL_E2E_RESET", "1")
	t.Setenv("KONGCTL_E2E_RESET_IDENTITY", "0")
	t.Setenv("KONGCTL_E2E_RESET_RETRY_ATTEMPTS", "1")
	t.Setenv("KONGCTL_E2E_SKIP_STEPS", "")
	t.Setenv("KONGCTL_E2E_STOP_AFTER", "")
	if name == "disabled" {
		t.Setenv("KONGCTL_E2E_RESET", "0")
	}
	mode := RecoveryActive
	if name == "report" {
		mode = RecoveryReport
	}
	for i := range 3 {
		t.Run("scenarios/"+strconv.Itoa(i)+"/scenario.yaml", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "scenario.yaml")
			definition := `defaults:
  retry:
    attempts: 1
steps:
  - name: create
    skipInputs: true
    commands:
      - name: portal
        parseAs: json
        create:
          resource: portal
          payload:
            inline:
              name: fixture
`
			if name == "assertion" {
				definition += `        assertions:
          - expect:
              fields:
                name: wrong
`
			}
			if err := os.WriteFile(path, []byte(definition), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := RunWithRecovery(t, path, BetaModeFail, mode)
			if err != nil {
				t.Fatalf("scenario failed: %v", err)
			}
		})
	}
}
