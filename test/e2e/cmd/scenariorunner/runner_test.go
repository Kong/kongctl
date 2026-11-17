package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// noTick installs no real ticker; these tests only care about events arriving
// through drive(), not heartbeat/live-redraw cadence.
func noTick(func(time.Time)) func() { return func() {} }

func TestDrivePreservesRawOutputInRunLogIncludingPasses(t *testing.T) {
	stdout := strings.NewReader(events(
		t,
		testEvent{Action: "run", Test: topLevelTest},
		testEvent{Action: "output", Test: topLevelTest, Output: "=== RUN   " + topLevelTest + "\n"},
		testEvent{Action: "output", Test: topLevelTest, Output: sentinelPrefix + "1\n"},

		testEvent{Action: "run", Test: scenarioOneTest},
		testEvent{Action: "output", Test: scenarioOneTest, Output: "=== RUN   " + scenarioOneTest + "\n"},
		testEvent{Action: "output", Test: scenarioOneTest, Output: "--- PASS: " + scenarioOneTest + " (1.23s)\n"},
		testEvent{Action: "pass", Test: scenarioOneTest, Elapsed: 1.23},

		testEvent{Action: "output", Test: topLevelTest, Output: "--- PASS: " + topLevelTest + " (1.50s)\n"},
		testEvent{Action: "pass", Test: topLevelTest, Elapsed: 1.5},
		testEvent{Action: "output", Output: "PASS\n"},
	))
	var runLog bytes.Buffer
	var term bytes.Buffer
	renderer := newRenderer(&term, false, time.Now, 30*time.Second)

	s, ranToCompletion := drive(driveConfig{
		stdout:       stdout,
		stderr:       strings.NewReader(""),
		runLog:       &runLog,
		renderer:     renderer,
		tick:         noTick,
		artifactsDir: "/tmp/run",
	})

	if !ranToCompletion {
		t.Fatalf("expected the run to be marked complete")
	}
	if s.counts.passed != 1 || s.counts.completed != 1 {
		t.Fatalf("unexpected summary counts: %+v", s.counts)
	}

	log := runLog.String()
	for _, want := range []string{
		"=== RUN   " + topLevelTest + "\n",
		"--- PASS: " + scenarioOneTest + " (1.23s)\n",
		"--- PASS: " + topLevelTest + " (1.50s)\n",
		"PASS\n",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("expected run.log to contain %q, got %q", want, log)
		}
	}
}

func TestDriveForwardsStderrAsPermanentLinesAndIntoRunLog(t *testing.T) {
	stdout := strings.NewReader(events(
		t,
		testEvent{Action: "run", Test: topLevelTest},
		testEvent{Action: "output", Test: topLevelTest, Output: sentinelPrefix + "0\n"},
		testEvent{Action: "output", Test: topLevelTest, Output: "--- SKIP: " + topLevelTest + " (0.00s)\n"},
		testEvent{Action: "skip", Test: topLevelTest, Elapsed: 0},
	))
	stderr := strings.NewReader("2026-01-01T00:00:00Z [e2e WARN] beta scenario failed\n")
	var runLog bytes.Buffer
	var term bytes.Buffer
	renderer := newRenderer(&term, false, time.Now, 30*time.Second)

	_, ranToCompletion := drive(driveConfig{
		stdout:       stdout,
		stderr:       stderr,
		runLog:       &runLog,
		renderer:     renderer,
		tick:         noTick,
		artifactsDir: "/tmp/run",
	})
	if !ranToCompletion {
		t.Fatalf("expected a top-level skip to still count as a completed run")
	}

	if !strings.Contains(term.String(), "beta scenario failed") {
		t.Fatalf("expected the warning to be visible in terminal output, got %q", term.String())
	}
	if !strings.Contains(runLog.String(), "beta scenario failed") {
		t.Fatalf("expected the warning to also be preserved in run.log, got %q", runLog.String())
	}
}

// TestDriveSurfacesHarnessWarningsFromStdoutEvenWhenScenarioPasses reflects
// real `go test -json` behavior (verified against the Go toolchain): the
// test binary's os.Stderr writes, including harness.Warnf/Errorf, are folded
// into the stdout event stream as "output" actions tied to whichever test is
// current, not delivered on the process's real stderr. A beta advisory
// failure logs exactly such a warning while still letting the scenario
// subtest pass, so the warning must surface even though cfg.stderr is empty
// and the scenario's own outcome is "pass".
func TestDriveSurfacesHarnessWarningsFromStdoutEvenWhenScenarioPasses(t *testing.T) {
	warnLine := "2026-01-01T00:00:00Z [e2e WARN] Beta scenario failed: identity/one: boom\n"
	stdout := strings.NewReader(events(
		t,
		testEvent{Action: "run", Test: topLevelTest},
		testEvent{Action: "output", Test: topLevelTest, Output: sentinelPrefix + "1\n"},

		testEvent{Action: "run", Test: scenarioOneTest},
		testEvent{Action: "output", Test: scenarioOneTest, Output: warnLine},
		testEvent{Action: "output", Test: scenarioOneTest, Output: "--- PASS: " + scenarioOneTest + " (1.23s)\n"},
		testEvent{Action: "pass", Test: scenarioOneTest, Elapsed: 1.23},

		testEvent{Action: "output", Test: topLevelTest, Output: "--- PASS: " + topLevelTest + " (1.50s)\n"},
		testEvent{Action: "pass", Test: topLevelTest, Elapsed: 1.5},
	))
	var term bytes.Buffer
	renderer := newRenderer(&term, false, time.Now, 30*time.Second)

	s, ranToCompletion := drive(driveConfig{
		stdout:       stdout,
		stderr:       strings.NewReader(""), // real go test -json never writes this here
		runLog:       &bytes.Buffer{},
		renderer:     renderer,
		tick:         noTick,
		artifactsDir: "/tmp/run",
	})
	if !ranToCompletion {
		t.Fatalf("expected the run to be marked complete")
	}
	if s.counts.passed != 1 {
		t.Fatalf("expected the scenario to still count as passed, got %+v", s.counts)
	}

	out := term.String()
	if !strings.Contains(out, "[e2e WARN] Beta scenario failed") {
		t.Fatalf("expected the beta advisory warning to be visible despite the scenario passing, got %q", out)
	}
}

// TestDriveAppendsBuildOutputToRunLog ensures compiler diagnostics from a
// failed build (delivered as "build-output" events, not "output") are
// preserved in run.log, matching the documented guarantee that run.log
// preserves the full raw go test output.
func TestDriveAppendsBuildOutputToRunLog(t *testing.T) {
	stdout := strings.NewReader(events(
		t,
		testEvent{Action: "build-output", Output: "# t2jcheck\n"},
		testEvent{Action: "build-output", Output: "./x_test.go:5:2: undefined: foo\n"},
		testEvent{Action: "build-fail"},
	))
	var runLog bytes.Buffer
	renderer := newRenderer(&bytes.Buffer{}, false, time.Now, 30*time.Second)

	_, ranToCompletion := drive(driveConfig{
		stdout:       stdout,
		stderr:       strings.NewReader(""),
		runLog:       &runLog,
		renderer:     renderer,
		tick:         noTick,
		artifactsDir: "/tmp/run",
	})
	if !ranToCompletion {
		t.Fatalf("expected build-fail to mark the run done")
	}
	if !strings.Contains(runLog.String(), "undefined: foo") {
		t.Fatalf("expected build-output text to be preserved in run.log, got %q", runLog.String())
	}
}

func TestDriveAbnormalTerminationReportsFailureAndStopsCleanly(t *testing.T) {
	// stdout ends with EOF without Test_Scenarios ever reaching pass/fail/skip,
	// simulating a panic or timeout that killed the process mid-run.
	stdout := strings.NewReader(events(
		t,
		testEvent{Action: "run", Test: topLevelTest},
		testEvent{Action: "output", Test: topLevelTest, Output: sentinelPrefix + "1\n"},
		testEvent{Action: "run", Test: scenarioOneTest},
	))
	var term bytes.Buffer
	renderer := newRenderer(&term, true, time.Now, 30*time.Second)

	s, ranToCompletion := drive(driveConfig{
		stdout:       stdout,
		stderr:       strings.NewReader("panic: boom\n"),
		runLog:       &bytes.Buffer{},
		renderer:     renderer,
		tick:         noTick,
		artifactsDir: "/tmp/run",
	})
	if ranToCompletion {
		t.Fatalf("expected ranToCompletion=false when Test_Scenarios never reaches a terminal action")
	}

	renderer.Abnormal(errors.New("exit status 2"))
	renderer.Finish(s)

	out := term.String()
	if !strings.Contains(out, "panic: boom") {
		t.Fatalf("expected the panic text to be visible, got %q", out)
	}
	if !strings.Contains(out, "ended unexpectedly") {
		t.Fatalf("expected an abnormal-termination notice, got %q", out)
	}
	// No stale live block: nothing should follow the final clear but the notices/summary.
	tail := out[strings.LastIndex(out, "\x1b[2K")+len("\x1b[2K"):]
	if strings.Contains(tail, "Running") || strings.Contains(tail, "Scenarios ") {
		t.Fatalf("expected no stale live block content after the final clear, got tail %q (full %q)", tail, out)
	}
}
