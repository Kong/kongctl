package e2e

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFormatProgressLine(t *testing.T) {
	got := formatProgressLine(18, 37, 6*time.Minute+32*time.Second, "identity/groups")
	want := "[18/37] elapsed 6m32s · current: identity/groups"
	if got != want {
		t.Fatalf("formatProgressLine() = %q, want %q", got, want)
	}
}

func TestFormatElapsed(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{6*time.Minute + 32*time.Second + 400*time.Millisecond, "6m32s"},
		{6*time.Minute + 32*time.Second + 600*time.Millisecond, "6m33s"},
		{0, "0s"},
		{-5 * time.Second, "0s"},
	}
	for _, tc := range cases {
		if got := formatElapsed(tc.d); got != tc.want {
			t.Errorf("formatElapsed(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestProgressScenarioName(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"test/e2e/scenarios/identity/groups/scenario.yaml", "identity/groups"},
		{"scenarios/apis/basic/scenario.yaml", "apis/basic"},
	}
	for _, tc := range cases {
		if got := progressScenarioName(tc.path); got != tc.want {
			t.Errorf("progressScenarioName(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestIsInteractiveWriterPipeIsNotInteractive(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if isInteractiveWriter(w) {
		t.Fatalf("expected pipe to be reported as non-interactive")
	}
}

func TestScenarioProgressInteractiveRedrawsInPlace(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	clock := func() time.Time { return now }

	p := newScenarioProgress(&buf, 2, true, withProgressClock(clock), withManualTick())

	p.startScenario("identity/groups")
	now = now.Add(5 * time.Second)
	p.tick()
	p.completeScenario()
	now = now.Add(1 * time.Second)
	p.startScenario("identity/users")
	p.stop()

	out := buf.String()
	if n := strings.Count(out, "\r"); n != 3 {
		t.Fatalf("expected 3 in-place redraws, got %d; output: %q", n, out)
	}
	if !strings.Contains(out, "[0/2] elapsed 0s · current: identity/groups") {
		t.Fatalf("missing initial line; output: %q", out)
	}
	if !strings.Contains(out, "[0/2] elapsed 5s · current: identity/groups") {
		t.Fatalf("missing tick line; output: %q", out)
	}
	if !strings.Contains(out, "[1/2] elapsed 6s · current: identity/users") {
		t.Fatalf("missing final line; output: %q", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected trailing newline after stop(); output: %q", out)
	}
}

// TestProgressCompletesOnPassAndSkip verifies completeScenario still runs (via
// defer) when a scenario subtest exits through t.SkipNow, mirroring how
// Test_Scenarios defers progress.completeScenario() around scenario.Run,
// including for skipped beta/advisory scenarios.
func TestProgressCompletesOnPassAndSkip(t *testing.T) {
	var buf bytes.Buffer
	p := newScenarioProgress(&buf, 2, false, withManualTick())

	run := func(name string, body func(t *testing.T)) {
		t.Run(name, func(t *testing.T) {
			p.startScenario(name)
			defer p.completeScenario()
			body(t)
		})
	}

	run("pass", func(_ *testing.T) {})
	run("skip", func(t *testing.T) { t.SkipNow() })

	p.mu.Lock()
	completed := p.completed
	p.mu.Unlock()
	if completed != 2 {
		t.Fatalf("expected completeScenario to run for both pass and skip, got completed=%d", completed)
	}
}

func TestScenarioProgressNonInteractiveHeartbeat(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	clock := func() time.Time { return now }

	p := newScenarioProgress(
		&buf, 2, false,
		withProgressClock(clock),
		withManualTick(),
		withProgressHeartbeat(30*time.Second),
	)

	p.startScenario("a")

	now = now.Add(10 * time.Second)
	p.tick() // below heartbeat interval: no additional line

	now = now.Add(21 * time.Second) // 31s total: heartbeat fires
	p.tick()

	p.completeScenario()
	now = now.Add(1 * time.Second)
	p.startScenario("b")
	p.stop()

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), lines)
	}
	want := []string{
		"[0/2] elapsed 0s · current: a",
		"[0/2] elapsed 31s · current: a",
		"[1/2] elapsed 32s · current: b",
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d = %q, want %q", i, lines[i], w)
		}
	}
}
