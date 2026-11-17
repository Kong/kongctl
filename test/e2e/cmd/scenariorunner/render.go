package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

// summary carries everything Finish needs to print the final report.
type summary struct {
	counts       counts
	totalElapsed time.Duration
	elapsedKnown bool
	failedList   []scenarioResult
	slowest      []scenarioResult
	artifactsDir string
}

// Renderer is the sole writer of terminal output (and, by extension, the
// in-place live block) for one scenario run. Interactive terminals get a
// redrawn-in-place live block with permanent lines for skips/failures/
// warnings printed above it; non-interactive output is plain, ANSI-free,
// append-only lines.
type Renderer struct {
	out         io.Writer
	interactive bool
	now         func() time.Time
	heartbeat   time.Duration

	mu            sync.Mutex
	liveLines     int
	finished      bool
	runStart      time.Time
	currentName   string
	currentStart  time.Time
	lastHeartbeat time.Time
	lastCounts    counts
}

func newRenderer(out io.Writer, interactive bool, now func() time.Time, heartbeat time.Duration) *Renderer {
	return &Renderer{out: out, interactive: interactive, now: now, heartbeat: heartbeat}
}

// onUpdate reacts to one semantic change from the aggregator.
func (r *Renderer) onUpdate(u update) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch u.kind {
	case updateTotalKnown:
		r.lastCounts = u.counts
		r.redrawLiveLocked(r.now())
	case updateRunStarted:
		now := r.now()
		r.runStart = now
		r.lastCounts = u.counts
		r.redrawLiveLocked(now)
	case updateScenarioStarted:
		now := r.now()
		r.currentName = u.name
		r.currentStart = now
		r.lastHeartbeat = now
		r.lastCounts = u.counts
		r.redrawLiveLocked(now)
	case updateScenarioFinished:
		r.currentName = ""
		r.lastCounts = u.counts
		r.onScenarioFinishedLocked(u.result)
	case updateRunFinished:
		r.lastCounts = u.counts
		if u.abnormal || u.result.outcome != outcomePass || len(u.result.warnings) > 0 {
			r.onTopLevelNoticeLocked(u)
		}
	}
}

// tick drives the live-block redraw cadence (interactive) or heartbeat lines
// for a long-running scenario (non-interactive).
func (r *Renderer) tick(now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished {
		return
	}
	if r.interactive {
		r.redrawLiveLocked(now)
		return
	}
	if r.currentName != "" && now.Sub(r.lastHeartbeat) >= r.heartbeat {
		fmt.Fprintf(r.out, "...   %s · %s (still running)\n", r.currentName, formatDuration(now.Sub(r.currentStart)))
		r.lastHeartbeat = now
	}
}

// PermanentLine surfaces one line of forwarded subprocess stderr (warnings,
// panics, build errors) above the live block, never through it.
func (r *Renderer) PermanentLine(text string) {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished {
		return
	}
	r.printPermanentLocked(text)
	r.redrawLiveLocked(r.now())
}

// Abnormal reports that the subprocess ended without Test_Scenarios ever
// reaching a terminal action (a panic, timeout, or killed process).
func (r *Renderer) Abnormal(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished {
		return
	}
	r.printPermanentLocked(fmt.Sprintf("FAIL  scenario run ended unexpectedly: %v", err))
}

// Finish replaces the live block with the final summary. Safe to call at most
// meaningfully once; later calls are no-ops.
func (r *Renderer) Finish(s summary) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished {
		return
	}
	r.finished = true
	r.clearLiveLocked()

	elapsed := s.totalElapsed
	if !s.elapsedKnown && !r.runStart.IsZero() {
		elapsed = r.now().Sub(r.runStart)
	}

	fmt.Fprintln(r.out)
	fmt.Fprintf(r.out, "%d passed · %d skipped · %d failed · total %s\n",
		s.counts.passed, s.counts.skipped, s.counts.failed, formatDuration(elapsed))

	if len(s.failedList) > 0 {
		fmt.Fprintln(r.out)
		fmt.Fprintln(r.out, "Failed:")
		for _, f := range s.failedList {
			fmt.Fprintf(r.out, "  %s · %s\n", f.name, formatScenarioDuration(f.elapsed))
		}
	}

	if len(s.slowest) > 0 {
		parts := make([]string, 0, len(s.slowest))
		for _, sc := range s.slowest {
			parts = append(parts, fmt.Sprintf("%s %s", sc.name, formatScenarioDuration(sc.elapsed)))
		}
		fmt.Fprintf(r.out, "\nSlowest: %s\n", strings.Join(parts, " · "))
	}

	fmt.Fprintf(r.out, "Artifacts: %s\n", s.artifactsDir)
}

func (r *Renderer) onScenarioFinishedLocked(res scenarioResult) {
	if r.interactive {
		if res.outcome == outcomePass && len(res.warnings) == 0 {
			r.redrawLiveLocked(r.now())
			return
		}
		r.printPermanentLocked(scenarioBlockLines(res)...)
		r.redrawLiveLocked(r.now())
		return
	}
	for _, line := range nonInteractiveScenarioLines(r.lastCounts, res) {
		fmt.Fprintln(r.out, line)
	}
}

func (r *Renderer) onTopLevelNoticeLocked(u update) {
	name := topLevelTest
	if u.abnormal {
		name = "setup"
	}
	lines := append([]string{fmt.Sprintf("%s  %s", u.result.outcome.label(), name)}, extraLines(u.result)...)
	if r.interactive {
		r.printPermanentLocked(lines...)
		r.redrawLiveLocked(r.now())
		return
	}
	for _, line := range lines {
		fmt.Fprintln(r.out, line)
	}
}

func (r *Renderer) clearLiveLocked() {
	if !r.interactive || r.liveLines == 0 {
		return
	}
	for range r.liveLines {
		fmt.Fprint(r.out, "\x1b[1A\x1b[2K")
	}
	r.liveLines = 0
}

func (r *Renderer) drawLiveLocked(lines []string) {
	if !r.interactive {
		return
	}
	for _, line := range lines {
		fmt.Fprintln(r.out, line)
	}
	r.liveLines = len(lines)
}

func (r *Renderer) redrawLiveLocked(now time.Time) {
	if !r.interactive || r.finished {
		return
	}
	r.clearLiveLocked()
	r.drawLiveLocked(r.buildLiveBlockLocked(now))
}

func (r *Renderer) printPermanentLocked(lines ...string) {
	r.clearLiveLocked()
	for _, line := range lines {
		fmt.Fprintln(r.out, line)
	}
}

func (r *Renderer) buildLiveBlockLocked(now time.Time) []string {
	c := r.lastCounts
	line1 := fmt.Sprintf("Scenarios  %d/%d complete · %d passed · %d skipped · %d failed",
		c.completed, c.total, c.passed, c.skipped, c.failed)

	line2 := "Running    -"
	if r.currentName != "" {
		line2 = fmt.Sprintf("Running    %s · %s", r.currentName, formatDuration(now.Sub(r.currentStart)))
	}

	var runElapsed time.Duration
	if !r.runStart.IsZero() {
		runElapsed = now.Sub(r.runStart)
	}
	line3 := fmt.Sprintf("Elapsed    %s", formatDuration(runElapsed))
	return []string{line1, line2, line3}
}

func scenarioBlockLines(res scenarioResult) []string {
	header := fmt.Sprintf("%s  %s · %s", res.outcome.label(), res.name, formatScenarioDuration(res.elapsed))
	return append([]string{header}, extraLines(res)...)
}

func nonInteractiveScenarioLines(c counts, res scenarioResult) []string {
	header := fmt.Sprintf("[%d/%d] %s  %s · %s",
		c.completed, c.total, res.outcome.label(), res.name, formatScenarioDuration(res.elapsed))
	return append([]string{header}, extraLines(res)...)
}

// extraLines renders a result's skip reason/failure diagnostics followed by
// any harness warnings, each indented so they read as detail under the
// header line.
func extraLines(res scenarioResult) []string {
	lines := detailLines(res.detail)
	for _, w := range res.warnings {
		lines = append(lines, "      "+w)
	}
	return lines
}

func detailLines(detail string) []string {
	if detail == "" {
		return nil
	}
	var lines []string
	for _, d := range strings.Split(detail, "\n") {
		if d == "" {
			continue
		}
		lines = append(lines, "      "+d)
	}
	return lines
}

// formatDuration renders an approximate/aggregate duration rounded to whole
// seconds, e.g. "7m50s".
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return d.Round(time.Second).String()
}

// formatScenarioDuration renders an authoritative per-scenario duration with
// sub-second precision, e.g. "23.39s".
func formatScenarioDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return strconv.FormatFloat(d.Seconds(), 'f', 2, 64) + "s"
}
