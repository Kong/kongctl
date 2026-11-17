package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestRendererInteractiveLiveBlockRedrawsInPlace(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	clock := func() time.Time { return now }
	r := newRenderer(&buf, true, clock, 30*time.Second)

	r.onUpdate(update{kind: updateTotalKnown, total: 2, counts: counts{total: 2}})
	r.onUpdate(update{kind: updateRunStarted, counts: counts{total: 2}})
	r.onUpdate(update{kind: updateScenarioStarted, name: "identity/groups", counts: counts{total: 2}})

	now = now.Add(5 * time.Second)
	r.tick(now)

	out := buf.String()
	if n := strings.Count(out, "\x1b[2K"); n == 0 {
		t.Fatalf("expected interactive redraws to clear lines, got %q", out)
	}
	if !strings.Contains(out, "Running    identity/groups · 5s") {
		t.Fatalf("expected a running line with elapsed, got %q", out)
	}
	if !strings.Contains(out, "Scenarios  0/2 complete") {
		t.Fatalf("expected a scenarios count line, got %q", out)
	}
}

func TestRendererInteractivePassDoesNotLeavePermanentLine(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	r := newRenderer(&buf, true, func() time.Time { return now }, 30*time.Second)

	r.onUpdate(update{kind: updateRunStarted})
	r.onUpdate(update{kind: updateScenarioStarted, name: "a", counts: counts{total: 1}})
	r.onUpdate(update{
		kind:   updateScenarioFinished,
		result: scenarioResult{name: "a", outcome: outcomePass},
		counts: counts{total: 1, completed: 1, passed: 1},
	})

	out := buf.String()
	if strings.Contains(out, "PASS") {
		t.Fatalf("expected no permanent PASS line in interactive mode, got %q", out)
	}
}

func TestRendererInteractiveFailureStaysAbovePermanentlyAndLiveBlockSurvives(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	r := newRenderer(&buf, true, func() time.Time { return now }, 30*time.Second)

	r.onUpdate(update{kind: updateRunStarted})
	r.onUpdate(update{kind: updateScenarioStarted, name: "identity/csv", counts: counts{total: 2}})
	r.onUpdate(update{
		kind:   updateScenarioFinished,
		counts: counts{total: 2, completed: 1, failed: 1},
		result: scenarioResult{
			name: "identity/csv", outcome: outcomeFail, elapsed: 23390 * time.Millisecond,
			detail: "step 4: expected exit code 0, got 1",
		},
	})
	r.onUpdate(update{
		kind: updateScenarioStarted, name: "identity/groups",
		counts: counts{total: 2, completed: 1, failed: 1},
	})

	out := buf.String()
	if !strings.Contains(out, "FAIL  identity/csv · 23.39s") {
		t.Fatalf("expected a permanent FAIL header, got %q", out)
	}
	if !strings.Contains(out, "step 4: expected exit code 0, got 1") {
		t.Fatalf("expected failure diagnostics to be visible, got %q", out)
	}
	if !strings.Contains(out, "Running    identity/groups") {
		t.Fatalf("expected the live block to still show the next running scenario, got %q", out)
	}

	// The FAIL block must appear exactly once: later redraws must never
	// reprint or erase it, only the live block below it may be redrawn.
	if n := strings.Count(out, "FAIL  identity/csv"); n != 1 {
		t.Fatalf("expected the FAIL line to appear exactly once, got %d in %q", n, out)
	}
}

func TestRendererNonInteractiveIsANSIFreeAndReportsEveryOutcome(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	r := newRenderer(&buf, false, func() time.Time { return now }, 30*time.Second)

	r.onUpdate(update{kind: updateRunStarted})
	r.onUpdate(update{kind: updateScenarioStarted, name: "a", counts: counts{total: 2}})
	r.onUpdate(update{
		kind:   updateScenarioFinished,
		counts: counts{total: 2, completed: 1, passed: 1},
		result: scenarioResult{name: "a", outcome: outcomePass, elapsed: 2 * time.Second},
	})
	r.onUpdate(update{
		kind: updateScenarioStarted, name: "b",
		counts: counts{total: 2, completed: 1, passed: 1},
	})
	r.onUpdate(update{
		kind:   updateScenarioFinished,
		counts: counts{total: 2, completed: 2, passed: 1, skipped: 1},
		result: scenarioResult{name: "b", outcome: outcomeSkip, elapsed: 0, detail: "beta disabled"},
	})

	out := buf.String()
	if strings.Contains(out, "\x1b") {
		t.Fatalf("expected no ANSI escape sequences in non-interactive output, got %q", out)
	}
	if !strings.Contains(out, "[1/2] PASS  a · 2.00s") {
		t.Fatalf("expected a plain pass line, got %q", out)
	}
	if !strings.Contains(out, "[2/2] SKIP  b · 0.00s") || !strings.Contains(out, "beta disabled") {
		t.Fatalf("expected a plain skip line with reason, got %q", out)
	}
}

func TestRendererNonInteractiveHeartbeatForLongScenario(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	clock := func() time.Time { return now }
	r := newRenderer(&buf, false, clock, 30*time.Second)

	r.onUpdate(update{kind: updateRunStarted})
	r.onUpdate(update{kind: updateScenarioStarted, name: "slow", counts: counts{total: 1}})

	now = now.Add(10 * time.Second)
	r.tick(now) // below heartbeat: no line
	now = now.Add(21 * time.Second)
	r.tick(now) // 31s: heartbeat fires

	out := buf.String()
	if strings.Count(out, "still running") != 1 {
		t.Fatalf("expected exactly one heartbeat line, got %q", out)
	}
}

func TestRendererFinishReplacesLiveBlockWithSummary(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	r := newRenderer(&buf, true, func() time.Time { return now }, 30*time.Second)

	r.onUpdate(update{kind: updateRunStarted})
	r.onUpdate(update{kind: updateScenarioStarted, name: "a", counts: counts{total: 1}})

	r.Finish(summary{
		counts:       counts{total: 1, completed: 1, passed: 1},
		totalElapsed: 9*time.Minute + 39*time.Second,
		elapsedKnown: true,
		artifactsDir: "/tmp/e2e-artifacts/run",
	})

	out := buf.String()
	tail := out[strings.LastIndex(out, "\x1b[2K")+len("\x1b[2K"):]
	if strings.Contains(tail, "Running") || strings.Contains(tail, "Scenarios ") {
		t.Fatalf("expected nothing after the final clear but the summary, got tail %q (full %q)", tail, out)
	}
	if !strings.Contains(out, "1 passed · 0 skipped · 0 failed · total 9m39s") {
		t.Fatalf("expected a summary line, got %q", out)
	}
	if !strings.Contains(out, "Artifacts: /tmp/e2e-artifacts/run") {
		t.Fatalf("expected an artifacts line, got %q", out)
	}

	// A second Finish call must be a no-op (idempotent finalize-once semantics).
	before := buf.String()
	r.Finish(summary{artifactsDir: "should-not-appear"})
	if buf.String() != before {
		t.Fatalf("expected a second Finish call to be a no-op")
	}
}

func TestRendererFinishIncludesFailedAndSlowest(t *testing.T) {
	var buf bytes.Buffer
	r := newRenderer(&buf, false, time.Now, 30*time.Second)

	r.Finish(summary{
		counts:       counts{passed: 28, skipped: 1, failed: 1},
		totalElapsed: 9*time.Minute + 39*time.Second,
		elapsedKnown: true,
		failedList:   []scenarioResult{{name: "identity/principal-csv", elapsed: 23390 * time.Millisecond}},
		slowest: []scenarioResult{
			{name: "identity/directory-delete-guards", elapsed: 46950 * time.Millisecond},
			{name: "identity/declarative-api-keys", elapsed: 46560 * time.Millisecond},
		},
		artifactsDir: "/tmp/run",
	})

	out := buf.String()
	for _, want := range []string{
		"28 passed · 1 skipped · 1 failed · total 9m39s",
		"Failed:",
		"identity/principal-csv · 23.39s",
		"Slowest: identity/directory-delete-guards 46.95s · identity/declarative-api-keys 46.56s",
		"Artifacts: /tmp/run",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected summary to contain %q, got %q", want, out)
		}
	}
}
