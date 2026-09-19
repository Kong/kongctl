//go:build e2e

package scenario

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
)

func TestTerminalDiagnosticsDoNotReuseRecoveredFailure(t *testing.T) {
	for _, phase := range []string{"assertion", "output", "setup", "reset", "expected_failure"} {
		t.Run(phase, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "scenario-diagnostics.json")
			d := newScenarioDiagnostics(path, "test/e2e/scenarios/all/scenario.yaml")
			d.begin("sync", "sync-all")
			d.subprocess(harness.Result{TimedOut: true, ExitCode: -1}, time.Minute)
			d.subprocess(harness.Result{Duration: time.Second}, time.Minute)
			d.phase = phase
			if err := d.finish(errors.New("secret-token timeout 503")); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var got scenarioDiagnostics
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if got.Failure.Cause != "unknown" || got.Failure.Phase != phase || len(got.Commands[0].Attempts) != 2 {
				t.Fatalf("unexpected diagnostics: %s", data)
			}
			if strings.Contains(string(data), "secret-token") {
				t.Fatal("diagnostics included raw error text")
			}
		})
	}
}

func TestDiagnosticsPreserveSuccessfulCommandAndTerminalTimeout(t *testing.T) {
	d := newScenarioDiagnostics(filepath.Join(t.TempDir(), "scenario-diagnostics.json"), "all/scenario.yaml")
	d.begin("reset", "reset-org")
	d.started = time.Now().Add(-time.Second)
	d.begin("sync", "sync-all")
	d.phase = "execution"
	d.subprocess(harness.Result{Duration: time.Minute, TimedOut: true, ExitCode: -1}, time.Minute)
	d.current.RetryStop = "retry_policy"
	if err := d.finish(errors.New("signal: killed")); err != nil {
		t.Fatal(err)
	}
	if len(d.Commands) != 2 || d.Commands[0].Outcome != "passed" || d.Commands[0].DurationMS < 1000 {
		t.Fatalf("successful command missing: %+v", d.Commands)
	}
	if d.Failure.Cause != "subprocess_deadline" || d.Failure.Command != "sync-all" {
		t.Fatalf("unexpected terminal failure: %+v", d.Failure)
	}
}

func TestDiagnosticsHTTPFailure(t *testing.T) {
	d := newScenarioDiagnostics(filepath.Join(t.TempDir(), "scenario-diagnostics.json"), "http/scenario.yaml")
	d.begin("create", "create-resource")
	d.phase = "http"
	d.http(harness.HTTPAttempt{Method: "POST", Host: "example.test", Status: 503, Duration: time.Second})
	if err := d.finish(errors.New("request failed")); err != nil {
		t.Fatal(err)
	}
	if d.Failure.Cause != "http_status" || d.Commands[0].Attempts[0].Status != 503 {
		t.Fatalf("HTTP evidence missing: %+v", d)
	}
}

func TestObservedCLITimeoutKeepsRetryPolicy(t *testing.T) {
	d := newScenarioDiagnostics(filepath.Join(t.TempDir(), "scenario-diagnostics.json"), "all/scenario.yaml")
	d.begin("sync", "sync-all")
	cli := &harness.CLI{BinPath: "/bin/sh", Env: os.Environ()}
	res, err := runCLIWithRetry(cli, "sync-all", Retry{}, []string{"-c", "exec sleep 10"}, nil,
		20*time.Millisecond, d)
	if err == nil || !res.TimedOut {
		t.Fatalf("expected timeout, got %+v, %v", res, err)
	}
	if len(d.current.Attempts) != 1 || d.current.RetryStop != "retry_policy" {
		t.Fatalf("timeout retry behavior changed: %+v", d.current)
	}
}

func TestScenarioDiagnosticsForExternalCommandAndAssertion(t *testing.T) {
	dir := t.TempDir()
	d := newScenarioDiagnostics(filepath.Join(dir, "scenario-diagnostics.json"), "deck/scenario.yaml")
	cli := &harness.CLI{BinPath: "/bin/sh", Env: os.Environ(), TestDir: dir, Timeout: time.Second}
	s := Scenario{Steps: []Step{{Name: "external", SkipInputs: true, Commands: []Command{
		{Name: "deck-like-success", Exec: []string{"/bin/sh", "-c", "echo '{}'"}},
		{Name: "assertion-failure", Exec: []string{"/bin/sh", "-c", "echo '{}'"}, Assertions: []Assertion{
			{Retry: Retry{Attempts: 1}, Expect: Expect{Fields: map[string]any{"missing": "value"}}},
		}},
	}}}}
	err := executeScenario(t, "deck/scenario.yaml", s, cli, d)
	if err == nil {
		t.Fatal("expected assertion failure")
	}
	if err := d.finish(err); err != nil {
		t.Fatal(err)
	}
	if d.Failure.Phase != "assertion" || d.Failure.Cause != "unknown" {
		t.Fatalf("assertion misclassified: %+v", d.Failure)
	}
	if len(d.Commands) != 2 || d.Commands[0].Outcome != "passed" || d.Commands[1].Outcome != "failed" {
		t.Fatalf("command outcomes incorrect: %+v", d.Commands)
	}
	for _, command := range d.Commands {
		if len(command.Attempts) != 1 || command.Attempts[0].ExitCode != 0 {
			t.Fatalf("external execution observation missing: %+v", command)
		}
	}
}

func TestObservedCLIRetryPreservesBothAttempts(t *testing.T) {
	dir := t.TempDir()
	d := newScenarioDiagnostics(filepath.Join(dir, "scenario-diagnostics.json"), "retry/scenario.yaml")
	d.begin("read", "read-resource")
	cli := &harness.CLI{BinPath: "/bin/sh", Env: os.Environ()}
	args := []string{
		"-c", `if [ -e "$1" ]; then exit 0; fi; touch "$1"; echo transient >&2; exit 1`,
		"retry-test", filepath.Join(dir, "attempted"),
	}
	res, err := runCLIWithRetry(cli, "read-resource", Retry{
		Attempts: 2, Interval: "1ms", MaxInterval: "1ms", Only: []string{"transient"},
	}, args, nil, time.Second, d)
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("expected recovery: %+v, %v", res, err)
	}
	if len(d.current.Attempts) != 2 || d.current.RetryStop != "succeeded" || d.current.AttemptLimit != 2 {
		t.Fatalf("retry observations incorrect: %+v", d.current)
	}
	if d.current.Attempts[0].ExitCode != 1 || d.current.Attempts[1].ExitCode != 0 {
		t.Fatalf("attempt outcomes incorrect: %+v", d.current.Attempts)
	}
}

func TestDiagnosticsLinkRecoveredAndFinalAttemptLogs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "steps", "sync", "commands", "sync")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "kongctl.log")
	failed := "log_type=http_phase request_id=khttp-000001 phase=request_written elapsed_ms=5 http_timeout_ms=15000\n"
	if err := os.WriteFile(path, []byte(failed), 0o600); err != nil {
		t.Fatal(err)
	}
	d := newScenarioDiagnostics(filepath.Join(root, "scenario-diagnostics.json"), "all")
	d.begin("sync", "sync")
	d.subprocess(harness.Result{ExitCode: 1}, time.Minute, dir)
	preserveAttemptArtifacts(dir, 0)
	d.preservedAttempt(0)
	completed := failed + "log_type=http_phase request_id=khttp-000001 phase=request_done " +
		"elapsed_ms=10 http_timeout_ms=15000 outcome=response_headers\n"
	if err := os.WriteFile(path, []byte(completed), 0o600); err != nil {
		t.Fatal(err)
	}
	d.subprocess(harness.Result{}, time.Minute, dir)
	d.phase = "assertion"
	if err := d.finish(errors.New("mismatch")); err != nil {
		t.Fatal(err)
	}
	attempts := d.Commands[0].Attempts
	if attempts[0].LogPath == attempts[1].LogPath {
		t.Fatal("attempt links collide")
	}
	for _, a := range attempts {
		if _, err := os.Stat(filepath.Join(root, a.LogPath)); err != nil {
			t.Fatal(err)
		}
	}
	if attempts[0].HTTPRequests[0].Outcome != "unfinished" || attempts[1].HTTPRequests[0].Outcome != "response_headers" {
		t.Fatalf("lost per-process trace evidence: %+v", attempts)
	}
	if d.Failure.Cause != "unknown" || d.Failure.Phase != "assertion" {
		t.Fatalf("wrong terminal cause: %+v", d.Failure)
	}
}
